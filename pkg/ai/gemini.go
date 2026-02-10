package ai

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os/exec"
	"strings"

	rigerrors "thoreinstein.com/rig/pkg/errors"
)

// GeminiProvider implements Provider using the Genkit SDK.
type GeminiProvider struct {
        apiKey string
        model  string
        logger *slog.Logger
}

// NewGeminiProvider creates a new Gemini provider.
func NewGeminiProvider(apiKey, model string, logger *slog.Logger) *GeminiProvider {
        return &GeminiProvider{
                apiKey: apiKey,
                model:  model,
                logger: logger,
        }
}
// Name returns the provider name.
func (p *GeminiProvider) Name() string {
	return ProviderGemini
}

// IsAvailable checks if the provider is configured.
func (p *GeminiProvider) IsAvailable() bool {
	return p.apiKey != ""
}

// Chat performs a single-turn chat completion using the gemini CLI.
func (p *GeminiProvider) Chat(ctx context.Context, messages []Message) (*Response, error) {
	if !p.IsAvailable() {
		return nil, rigerrors.NewAIError(ProviderGemini, "Chat", "Gemini API key not set")
	}

	prompt := p.buildPrompt(messages)
	args := []string{"-p", prompt, "-o", "json"}
	if p.model != "" {
		args = append(args, "-m", p.model)
	}

	p.logDebug("executing gemini cli", "apiKey", p.apiKey, "args", args)

	// #nosec G204 - command is configurable by user in config file
	cmd := exec.CommandContext(ctx, "gemini", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, rigerrors.NewAIErrorWithCause(ProviderGemini, "Chat",
			"gemini cli failed: "+string(output), err)
	}

	cleanOutput := p.stripNonJSON(string(output))

	// Try to parse as JSON
	var res struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(cleanOutput), &res); err != nil {
		// If JSON parsing fails, return the cleaned output as content
		if p.logger != nil {
			p.logger.Debug("failed to parse gemini JSON output", "error", err)
		}
		return &Response{
			Content: cleanOutput,
		}, nil
	}

	return &Response{
		Content: res.Content,
	}, nil
}

// StreamChat performs a streaming chat completion using the gemini CLI.
func (p *GeminiProvider) StreamChat(ctx context.Context, messages []Message) (<-chan StreamChunk, error) {
	if !p.IsAvailable() {
		return nil, rigerrors.NewAIError(ProviderGemini, "StreamChat", "Gemini API key not set")
	}

	prompt := p.buildPrompt(messages)
	args := []string{"-p", prompt, "-o", "stream-json"}
	if p.model != "" {
		args = append(args, "-m", p.model)
	}

	p.logDebug("executing gemini cli (streaming)", "apiKey", p.apiKey, "args", args)

	// #nosec G204 - command is configurable by user in config file
	cmd := exec.CommandContext(ctx, "gemini", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, rigerrors.NewAIErrorWithCause(ProviderGemini, "StreamChat",
			"failed to create stdout pipe", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, rigerrors.NewAIErrorWithCause(ProviderGemini, "StreamChat",
			"failed to start gemini cli", err)
	}

	chunks := make(chan StreamChunk)
	go p.streamOutput(ctx, stdout, cmd, chunks)

	return chunks, nil
}

func (p *GeminiProvider) streamOutput(ctx context.Context, r io.Reader, cmd *exec.Cmd, chunks chan<- StreamChunk) {
	defer close(chunks)

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			chunks <- StreamChunk{Error: ctx.Err(), Done: true}
			return
		default:
		}

		line := scanner.Text()
		if line == "" || !strings.HasPrefix(strings.TrimSpace(line), "{") {
			continue
		}

		var chunk struct {
			Content string `json:"content"`
			Done    bool   `json:"done"`
		}
		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			continue
		}

		if chunk.Content != "" {
			chunks <- StreamChunk{Content: chunk.Content}
		}
		if chunk.Done {
			chunks <- StreamChunk{Done: true}
			return
		}
	}

	if err := scanner.Err(); err != nil {
		chunks <- StreamChunk{Error: err, Done: true}
	}

	_ = cmd.Wait()
}

func (p *GeminiProvider) buildPrompt(messages []Message) string {
	var sb strings.Builder
	for _, msg := range messages {
		switch msg.Role {
		case "system":
			sb.WriteString("System: ")
		case "assistant":
			sb.WriteString("Assistant: ")
		default:
			sb.WriteString("User: ")
		}
		sb.WriteString(msg.Content)
		sb.WriteString("\n\n")
	}
	return strings.TrimSpace(sb.String())
}

func (p *GeminiProvider) stripNonJSON(output string) string {
	// Simple heuristic to find the first '{' and last '}'
	start := strings.Index(output, "{")
	end := strings.LastIndex(output, "}")
	if start != -1 && end != -1 && start < end {
		return output[start : end+1]
	}
	return strings.TrimSpace(output)
}

func (p *GeminiProvider) logDebug(msg string, args ...any) {
	if p.logger != nil {
		p.logger.Debug(msg, args...)
	}
}
