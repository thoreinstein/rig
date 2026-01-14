package debrief

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"thoreinstein.com/rig/pkg/ai"
	rigerrors "thoreinstein.com/rig/pkg/errors"
)

// DebriefSession manages the interactive debrief process.
type DebriefSession struct {
	provider ai.Provider
	context  *Context
	verbose  bool
	reader   io.Reader
	writer   io.Writer
}

// NewDebriefSession creates a new debrief session.
func NewDebriefSession(provider ai.Provider, ctx *Context, verbose bool) *DebriefSession {
	return &DebriefSession{
		provider: provider,
		context:  ctx,
		verbose:  verbose,
		reader:   os.Stdin,
		writer:   os.Stdout,
	}
}

// WithIO sets custom reader and writer for testing.
func (s *DebriefSession) WithIO(r io.Reader, w io.Writer) *DebriefSession {
	s.reader = r
	s.writer = w
	return s
}

// Run executes the interactive debrief session.
// It generates questions, collects answers interactively, and produces a summary.
func (s *DebriefSession) Run(ctx context.Context) (*Output, error) {
	// Generate questions based on context
	questions, err := s.GenerateQuestions(ctx)
	if err != nil {
		return nil, rigerrors.Wrap(err, "failed to generate questions")
	}

	if len(questions) == 0 {
		return nil, rigerrors.New("no questions generated")
	}

	// Collect answers interactively
	answers := make(map[string]string)
	for _, q := range questions {
		answer, err := s.AskQuestion(q)
		if err != nil {
			if err == io.EOF {
				break // User quit early
			}
			return nil, rigerrors.Wrapf(err, "failed to get answer for question %s", q.ID)
		}

		// Allow skipping non-required questions
		if answer == "" && !q.Required {
			continue
		}

		answers[q.ID] = answer
	}

	// Generate summary from Q&A
	output, err := s.GenerateSummary(ctx, answers)
	if err != nil {
		return nil, rigerrors.Wrap(err, "failed to generate summary")
	}

	return output, nil
}

// GenerateQuestions uses AI to create targeted questions based on the context.
func (s *DebriefSession) GenerateQuestions(ctx context.Context) ([]Question, error) {
	prompt := BuildQuestionPrompt(s.context)

	messages := []ai.Message{
		{Role: "system", Content: SystemPromptQuestions},
		{Role: "user", Content: prompt},
	}

	if s.verbose {
		fmt.Fprintf(s.writer, "Generating debrief questions...\n")
	}

	resp, err := s.provider.Chat(ctx, messages)
	if err != nil {
		return nil, err
	}

	// Parse questions from response
	questions, err := parseQuestions(resp.Content)
	if err != nil {
		return nil, rigerrors.Wrap(err, "failed to parse questions from AI response")
	}

	return questions, nil
}

// parseQuestions parses the AI response into Question structs.
// Expected format is JSON array of questions.
func parseQuestions(content string) ([]Question, error) {
	// Try to extract JSON from the response
	content = extractJSON(content)

	var rawQuestions []struct {
		ID       string `json:"id"`
		Text     string `json:"text"`
		Purpose  string `json:"purpose"`
		Required bool   `json:"required"`
	}

	if err := json.Unmarshal([]byte(content), &rawQuestions); err != nil {
		return nil, rigerrors.Wrapf(err, "failed to parse questions JSON: %s", content)
	}

	questions := make([]Question, len(rawQuestions))
	for i, rq := range rawQuestions {
		questions[i] = Question{
			ID:       rq.ID,
			Text:     rq.Text,
			Purpose:  rq.Purpose,
			Required: rq.Required,
		}
	}

	return questions, nil
}

// extractJSON attempts to extract a JSON array from text that may contain markdown.
func extractJSON(content string) string {
	// Look for JSON array markers
	start := strings.Index(content, "[")
	end := strings.LastIndex(content, "]")

	if start != -1 && end != -1 && end > start {
		return content[start : end+1]
	}

	return content
}

// AskQuestion prompts the user for an answer to a question.
func (s *DebriefSession) AskQuestion(q Question) (string, error) {
	// Display the question
	fmt.Fprintf(s.writer, "\n")
	if q.Purpose != "" {
		fmt.Fprintf(s.writer, "[%s]\n", q.Purpose)
	}
	fmt.Fprintf(s.writer, "%s\n", q.Text)
	if !q.Required {
		fmt.Fprintf(s.writer, "(Press Enter to skip)\n")
	}
	fmt.Fprintf(s.writer, "> ")

	// Read answer
	reader := bufio.NewReader(s.reader)

	var lines []string
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				if len(lines) == 0 {
					return "", io.EOF
				}
				break
			}
			return "", err
		}

		line = strings.TrimSuffix(line, "\n")
		line = strings.TrimSuffix(line, "\r")

		// Empty line ends multi-line input
		if line == "" {
			break
		}

		lines = append(lines, line)

		// For single-line answers, break after first line
		// unless user continues typing
		if len(lines) == 1 {
			// Check if there's more input immediately available
			// If not, assume single-line answer
			break
		}
	}

	return strings.Join(lines, "\n"), nil
}

// GenerateSummary creates the final output from the Q&A session.
func (s *DebriefSession) GenerateSummary(ctx context.Context, answers map[string]string) (*Output, error) {
	prompt := BuildSummaryPrompt(s.context, answers)

	messages := []ai.Message{
		{Role: "system", Content: SystemPromptSummary},
		{Role: "user", Content: prompt},
	}

	if s.verbose {
		fmt.Fprintf(s.writer, "\nGenerating debrief summary...\n")
	}

	resp, err := s.provider.Chat(ctx, messages)
	if err != nil {
		return nil, err
	}

	// Parse summary from response
	output, err := parseSummary(resp.Content)
	if err != nil {
		return nil, rigerrors.Wrap(err, "failed to parse summary from AI response")
	}

	output.GeneratedAt = time.Now()
	return output, nil
}

// parseSummary parses the AI response into an Output struct.
func parseSummary(content string) (*Output, error) {
	// Try to extract JSON from the response
	content = extractJSONObject(content)

	var rawOutput struct {
		Summary        string   `json:"summary"`
		KeyDecisions   []string `json:"key_decisions"`
		Challenges     []string `json:"challenges"`
		LessonsLearned []string `json:"lessons_learned"`
		FollowUps      []string `json:"follow_ups"`
	}

	if err := json.Unmarshal([]byte(content), &rawOutput); err != nil {
		return nil, rigerrors.Wrapf(err, "failed to parse summary JSON: %s", content)
	}

	return &Output{
		Summary:        rawOutput.Summary,
		KeyDecisions:   rawOutput.KeyDecisions,
		Challenges:     rawOutput.Challenges,
		LessonsLearned: rawOutput.LessonsLearned,
		FollowUps:      rawOutput.FollowUps,
	}, nil
}

// extractJSONObject attempts to extract a JSON object from text.
func extractJSONObject(content string) string {
	// Look for JSON object markers
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")

	if start != -1 && end != -1 && end > start {
		return content[start : end+1]
	}

	return content
}
