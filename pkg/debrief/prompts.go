package debrief

import (
	"fmt"
	"strings"
	"time"
)

// SystemPromptQuestions is the system prompt for generating debrief questions.
const SystemPromptQuestions = `You are a technical debrief facilitator helping developers reflect on their work.

Your role is to generate 3-5 targeted questions that will help extract:
1. Key technical decisions and their rationale
2. Challenges encountered and how they were resolved
3. Alternative approaches that were considered
4. Lessons learned that could help future work
5. Follow-up items or technical debt introduced

Guidelines:
- Questions should be specific to the work described, not generic
- Focus on decisions and learning, not just what was done
- Ask about trade-offs and alternatives considered
- Be concise - each question should be answerable in 1-3 sentences
- Mark questions about blockers or risks as required

You MUST respond with a JSON array of question objects. Each object must have:
- id: a short unique identifier (e.g., "q1", "decisions", "challenges")
- text: the question text
- purpose: brief explanation of why this is being asked (shown to user)
- required: boolean, true for essential questions

Example response format:
[
  {"id": "q1", "text": "What was the main technical decision you made?", "purpose": "Understanding key choices", "required": true},
  {"id": "q2", "text": "Were there any unexpected challenges?", "purpose": "Capturing blockers", "required": false}
]`

// SystemPromptSummary is the system prompt for generating the debrief summary.
const SystemPromptSummary = `You are a technical writer creating a debrief summary from a developer Q&A session.

Your role is to synthesize the provided context and answers into a structured summary that will be useful for:
1. Future reference when revisiting this work
2. Knowledge sharing with teammates
3. Tracking decisions and their rationale
4. Identifying patterns across projects

Guidelines:
- Be concise but capture all important points
- Use technical terminology appropriately
- Focus on decisions and learnings, not just tasks
- Extract actionable follow-ups
- Keep the summary professional but human

You MUST respond with a JSON object containing:
- summary: a 2-3 sentence high-level summary of the work
- key_decisions: array of important decisions made (with brief rationale)
- challenges: array of challenges encountered (with how they were resolved)
- lessons_learned: array of insights that could help future work
- follow_ups: array of items that need future attention

Keep each array item to 1-2 sentences maximum.`

// BuildQuestionPrompt creates the prompt for generating questions.
func BuildQuestionPrompt(ctx *Context) string {
	var sb strings.Builder

	sb.WriteString("Generate debrief questions based on the following work context:\n\n")

	// PR context
	if ctx.PRTitle != "" {
		sb.WriteString("## Pull Request\n")
		sb.WriteString(fmt.Sprintf("**Title:** %s\n", ctx.PRTitle))
		if ctx.PRBody != "" {
			sb.WriteString(fmt.Sprintf("**Description:**\n%s\n", truncate(ctx.PRBody, 500)))
		}
		sb.WriteString("\n")
	}

	// Ticket context
	if ctx.TicketID != "" {
		sb.WriteString("## Ticket\n")
		sb.WriteString(fmt.Sprintf("**ID:** %s\n", ctx.TicketID))
		sb.WriteString(fmt.Sprintf("**Type:** %s\n", ctx.TicketType))
		sb.WriteString(fmt.Sprintf("**Summary:** %s\n", ctx.TicketSummary))
		if ctx.TicketDescription != "" {
			sb.WriteString(fmt.Sprintf("**Description:**\n%s\n", truncate(ctx.TicketDescription, 300)))
		}
		sb.WriteString("\n")
	}

	// Commits
	if len(ctx.Commits) > 0 {
		sb.WriteString("## Commits\n")
		for _, commit := range ctx.Commits {
			sb.WriteString(fmt.Sprintf("- %s: %s\n", commit.SHA, commit.Message))
		}
		sb.WriteString("\n")
	}

	// Files changed
	if len(ctx.FilesChanged) > 0 {
		sb.WriteString("## Files Changed\n")
		// Group by directory for readability
		files := ctx.FilesChanged
		if len(files) > 20 {
			files = files[:20]
			sb.WriteString(fmt.Sprintf("(showing first 20 of %d files)\n", len(ctx.FilesChanged)))
		}
		for _, f := range files {
			sb.WriteString(fmt.Sprintf("- %s\n", f))
		}
		sb.WriteString("\n")
	}

	// Diff stats
	if ctx.DiffStats.FilesChanged > 0 {
		sb.WriteString("## Change Statistics\n")
		sb.WriteString(fmt.Sprintf("- Files changed: %d\n", ctx.DiffStats.FilesChanged))
		sb.WriteString(fmt.Sprintf("- Lines added: %d\n", ctx.DiffStats.Insertions))
		sb.WriteString(fmt.Sprintf("- Lines removed: %d\n", ctx.DiffStats.Deletions))
		sb.WriteString("\n")
	}

	// Relevant commands
	if len(ctx.RelevantCommands) > 0 {
		sb.WriteString("## Notable Commands Run\n")
		commands := ctx.RelevantCommands
		if len(commands) > 10 {
			commands = commands[len(commands)-10:]
		}
		for _, cmd := range commands {
			sb.WriteString(fmt.Sprintf("- %s\n", truncate(cmd, 100)))
		}
		sb.WriteString("\n")
	}

	// Duration
	if ctx.Duration > 0 {
		sb.WriteString("## Duration\n")
		sb.WriteString(fmt.Sprintf("Work duration: approximately %s\n\n", formatDuration(ctx.Duration)))
	}

	sb.WriteString("Generate 3-5 targeted questions that will help document the decisions, challenges, and learnings from this work.")

	return sb.String()
}

// BuildSummaryPrompt creates the prompt for generating the summary.
func BuildSummaryPrompt(ctx *Context, answers map[string]string) string {
	var sb strings.Builder

	sb.WriteString("Create a debrief summary from the following context and Q&A session:\n\n")

	// Include context summary
	sb.WriteString("## Work Context\n")
	if ctx.PRTitle != "" {
		sb.WriteString(fmt.Sprintf("**PR:** %s\n", ctx.PRTitle))
	}
	if ctx.TicketID != "" {
		sb.WriteString(fmt.Sprintf("**Ticket:** %s - %s\n", ctx.TicketID, ctx.TicketSummary))
	}
	sb.WriteString(fmt.Sprintf("**Branch:** %s\n", ctx.BranchName))
	if ctx.DiffStats.FilesChanged > 0 {
		sb.WriteString(fmt.Sprintf("**Changes:** %d files, +%d/-%d lines\n",
			ctx.DiffStats.FilesChanged, ctx.DiffStats.Insertions, ctx.DiffStats.Deletions))
	}
	if len(ctx.Commits) > 0 {
		sb.WriteString(fmt.Sprintf("**Commits:** %d\n", len(ctx.Commits)))
	}
	sb.WriteString("\n")

	// Include Q&A
	sb.WriteString("## Q&A Session\n")
	for id, answer := range answers {
		sb.WriteString(fmt.Sprintf("**%s:**\n%s\n\n", id, answer))
	}

	sb.WriteString("Generate a structured summary capturing the key decisions, challenges, lessons learned, and follow-ups.")

	return sb.String()
}

// truncate shortens a string to maxLen characters, adding ellipsis if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// formatDuration formats a duration in a human-readable way.
func formatDuration(d time.Duration) string {
	hours := int(d.Hours())
	days := hours / 24
	hours = hours % 24

	if days > 0 {
		return fmt.Sprintf("%d days, %d hours", days, hours)
	}
	if hours > 0 {
		return fmt.Sprintf("%d hours", hours)
	}
	return fmt.Sprintf("%d minutes", int(d.Minutes()))
}
