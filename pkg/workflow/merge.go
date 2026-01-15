package workflow

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"thoreinstein.com/rig/pkg/config"
	rigerrors "thoreinstein.com/rig/pkg/errors"
	"thoreinstein.com/rig/pkg/github"
	"thoreinstein.com/rig/pkg/jira"
)

// Engine orchestrates the merge workflow.
type Engine struct {
	github  github.Client
	jira    jira.JiraClient
	router  *TicketRouter
	cfg     *config.Config
	verbose bool
	logger  *slog.Logger
}

// NewEngine creates a workflow engine.
//
// Parameters:
//   - gh: GitHub client for PR operations (required)
//   - jiraClient: Jira client for ticket operations (may be nil if Jira is disabled)
//   - cfg: Configuration (required)
//   - projectPath: Path to the project directory for ticket routing
//   - verbose: Enable verbose logging
func NewEngine(gh github.Client, jiraClient jira.JiraClient, cfg *config.Config, projectPath string, verbose bool) *Engine {
	var logger *slog.Logger
	if verbose {
		logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	} else {
		logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	}

	return &Engine{
		github:  gh,
		jira:    jiraClient,
		router:  NewTicketRouter(cfg, projectPath, verbose),
		cfg:     cfg,
		verbose: verbose,
		logger:  logger,
	}
}

// Run executes the full merge workflow.
//
// The workflow proceeds through five steps:
// 1. Preflight - Verify PR is ready for merge
// 2. Gather - Collect PR details, commits, timeline
// 3. Debrief - Interactive AI Q&A (skippable)
// 4. Merge - Execute the merge on GitHub
// 5. Closeout - Transition Jira, cleanup resources
//
// If any step fails, the workflow saves a checkpoint and returns an error.
// Use Resume() to continue from the last successful step.
func (e *Engine) Run(ctx context.Context, prNumber int, opts MergeOptions) error {
	wf := &MergeWorkflow{
		PRNumber:       prNumber,
		StartedAt:      time.Now(),
		CompletedSteps: make([]Step, 0),
		Context:        &WorkflowContext{},
	}

	e.log("Starting merge workflow for PR #%d", prNumber)

	// Execute each step in order
	steps := []struct {
		step Step
		fn   func(context.Context, *MergeWorkflow, MergeOptions) error
	}{
		{StepPreflight, e.runPreflight},
		{StepGather, e.runGather},
		{StepDebrief, e.runDebrief},
		{StepMerge, e.runMerge},
		{StepCloseout, e.runCloseout},
	}

	for _, s := range steps {
		wf.CurrentStep = s.step
		e.log("Executing step: %s", s.step)

		if err := s.fn(ctx, wf, opts); err != nil {
			// Save checkpoint before returning error
			if wf.Worktree != "" {
				if saveErr := SaveCheckpoint(wf.Worktree, e.workflowToCheckpoint(wf)); saveErr != nil {
					e.logger.Warn("failed to save checkpoint", "error", saveErr)
				}
			}
			return rigerrors.NewWorkflowErrorWithCause(string(s.step), err.Error(), err)
		}

		wf.CompletedSteps = append(wf.CompletedSteps, s.step)
		e.log("Completed step: %s", s.step)

		// Update checkpoint after each successful step
		if wf.Worktree != "" {
			if err := SaveCheckpoint(wf.Worktree, e.workflowToCheckpoint(wf)); err != nil {
				e.logger.Warn("failed to update checkpoint", "error", err)
			}
		}
	}

	// Clear checkpoint on successful completion
	if wf.Worktree != "" {
		if err := ClearCheckpoint(wf.Worktree); err != nil {
			e.logger.Warn("failed to clear checkpoint", "error", err)
		}
	}

	e.log("Merge workflow completed successfully for PR #%d", prNumber)
	return nil
}

// Resume continues a workflow from checkpoint.
//
// The checkpoint contains the state of a previously interrupted workflow.
// Resume will skip already-completed steps and continue from where it left off.
func (e *Engine) Resume(ctx context.Context, checkpoint *Checkpoint) error {
	if checkpoint == nil {
		return rigerrors.NewWorkflowError("resume", "checkpoint is nil")
	}

	wf := &MergeWorkflow{
		PRNumber:       checkpoint.PRNumber,
		Ticket:         checkpoint.Ticket,
		Worktree:       checkpoint.Worktree,
		StartedAt:      checkpoint.CreatedAt,
		CompletedSteps: checkpoint.CompletedSteps,
		CurrentStep:    checkpoint.CurrentStep,
		Context:        checkpoint.Context,
	}

	// Build a set of completed steps for fast lookup
	completedSet := make(map[Step]bool)
	for _, s := range wf.CompletedSteps {
		completedSet[s] = true
	}

	e.log("Resuming merge workflow for PR #%d from step %s", wf.PRNumber, wf.CurrentStep)

	// Create default options for resume (could be enhanced to store in checkpoint)
	opts := MergeOptions{}

	// Execute remaining steps
	steps := []struct {
		step Step
		fn   func(context.Context, *MergeWorkflow, MergeOptions) error
	}{
		{StepPreflight, e.runPreflight},
		{StepGather, e.runGather},
		{StepDebrief, e.runDebrief},
		{StepMerge, e.runMerge},
		{StepCloseout, e.runCloseout},
	}

	for _, s := range steps {
		// Skip already-completed steps
		if completedSet[s.step] {
			e.log("Skipping completed step: %s", s.step)
			continue
		}

		wf.CurrentStep = s.step
		e.log("Executing step: %s", s.step)

		if err := s.fn(ctx, wf, opts); err != nil {
			// Save checkpoint before returning error
			if wf.Worktree != "" {
				if saveErr := SaveCheckpoint(wf.Worktree, e.workflowToCheckpoint(wf)); saveErr != nil {
					e.logger.Warn("failed to save checkpoint", "error", saveErr)
				}
			}
			return rigerrors.NewWorkflowErrorWithCause(string(s.step), err.Error(), err)
		}

		wf.CompletedSteps = append(wf.CompletedSteps, s.step)
		e.log("Completed step: %s", s.step)

		// Update checkpoint after each successful step
		if wf.Worktree != "" {
			if err := SaveCheckpoint(wf.Worktree, e.workflowToCheckpoint(wf)); err != nil {
				e.logger.Warn("failed to update checkpoint", "error", err)
			}
		}
	}

	// Clear checkpoint on successful completion
	if wf.Worktree != "" {
		if err := ClearCheckpoint(wf.Worktree); err != nil {
			e.logger.Warn("failed to clear checkpoint", "error", err)
		}
	}

	e.log("Resumed workflow completed successfully for PR #%d", wf.PRNumber)
	return nil
}

// workflowToCheckpoint converts a workflow state to a checkpoint.
func (e *Engine) workflowToCheckpoint(wf *MergeWorkflow) *Checkpoint {
	return &Checkpoint{
		PRNumber:       wf.PRNumber,
		Ticket:         wf.Ticket,
		Worktree:       wf.Worktree,
		CompletedSteps: wf.CompletedSteps,
		CurrentStep:    wf.CurrentStep,
		Context:        wf.Context,
		CreatedAt:      wf.StartedAt,
		UpdatedAt:      time.Now(),
	}
}

// log writes a message if verbose mode is enabled.
func (e *Engine) log(format string, args ...any) {
	if e.verbose {
		fmt.Printf("[workflow] "+format+"\n", args...)
	}
}

// Preflight runs only the preflight checks and returns the result.
// This is useful for checking if a PR is ready without running the full workflow.
func (e *Engine) Preflight(ctx context.Context, prNumber int, opts MergeOptions) (*PreflightResult, error) {
	result := &PreflightResult{
		JiraSkipped: opts.SkipJira,
	}

	// Check if PR exists and is open
	pr, err := e.github.GetPR(ctx, prNumber)
	if err != nil {
		result.FailureReason = fmt.Sprintf("failed to fetch PR: %v", err)
		return result, nil
	}

	result.PRExists = true
	result.PROpen = pr.State == "open" || pr.State == "OPEN"
	result.PRApproved = pr.Approved
	result.ApprovalSkipped = opts.SkipApproval
	result.ChecksPassing = pr.ChecksPassing

	if !result.PROpen {
		result.FailureReason = fmt.Sprintf("PR is not open (state: %s)", pr.State)
	} else if !result.PRApproved && !result.ApprovalSkipped {
		result.FailureReason = "PR is not approved (use --skip-approval for self-authored PRs)"
	} else if !result.ChecksPassing {
		result.FailureReason = "CI checks are not passing"
	}

	// Check Jira status if not skipped
	if !opts.SkipJira && e.jira != nil && e.jira.IsAvailable() {
		ticket := extractTicketFromBranch(pr.HeadBranch)
		if ticket != "" {
			source := e.router.RouteTicket(ticket)
			if source == TicketSourceJira {
				ticketInfo, err := e.jira.FetchTicketDetails(ticket)
				if err != nil {
					result.Warnings = append(result.Warnings, fmt.Sprintf("could not fetch Jira ticket: %v", err))
				} else {
					// Check if ticket is in "In Review" or similar status
					result.JiraInReview = isInReviewStatus(ticketInfo.Status)
					if !result.JiraInReview && result.FailureReason == "" {
						result.FailureReason = fmt.Sprintf("Jira ticket is not in review status (status: %s)", ticketInfo.Status)
					}
				}
			} else if source == TicketSourceBeads {
				// Beads tickets don't need Jira status checks
				result.JiraSkipped = true
			}
		} else {
			result.JiraSkipped = true
			result.Warnings = append(result.Warnings, "could not extract ticket from branch name")
		}
	}

	return result, nil
}

// extractTicketFromBranch attempts to extract a ticket ID from a branch name.
// Supports patterns like: PROJ-123, proj-123, feature/PROJ-123, etc.
func extractTicketFromBranch(branch string) string {
	// Simple extraction: look for patterns like WORD-NUMBER
	// This could be made more sophisticated with regex patterns from config
	for i := len(branch) - 1; i >= 0; i-- {
		if branch[i] == '/' || branch[i] == '-' || branch[i] == '_' {
			continue
		}
		// Find the start of a potential ticket
		start := i
		for start > 0 && branch[start-1] != '/' && branch[start-1] != '_' {
			start--
		}
		candidate := branch[start : i+1]
		if looksLikeTicket(candidate) {
			return candidate
		}
	}
	// Fallback: return the branch name itself if it looks like a ticket
	if looksLikeTicket(branch) {
		return branch
	}
	return ""
}

// looksLikeTicket checks if a string looks like a ticket (e.g., PROJ-123 or rig-abc123).
// Supports both traditional Jira-style tickets and beads-style alphanumeric identifiers.
func looksLikeTicket(s string) bool {
	if len(s) < 3 {
		return false
	}
	// Look for pattern: letters followed by dash followed by alphanumeric identifier
	dashIdx := -1
	for i, c := range s {
		if c == '-' {
			dashIdx = i
			break
		}
	}
	if dashIdx < 1 || dashIdx >= len(s)-1 {
		return false
	}
	// Check prefix is letters
	for i := range dashIdx {
		if !isLetter(s[i]) {
			return false
		}
	}
	// Check suffix is alphanumeric (digits or letters)
	for i := dashIdx + 1; i < len(s); i++ {
		if !isDigit(s[i]) && !isLetter(s[i]) {
			return false
		}
	}
	return true
}

func isLetter(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

// isInReviewStatus checks if a Jira status indicates the ticket is in review.
func isInReviewStatus(status string) bool {
	// Normalize to lowercase for comparison
	s := normalizeStatus(status)
	reviewStatuses := []string{
		"in review",
		"code review",
		"review",
		"pr review",
		"peer review",
		"awaiting review",
		"ready for review",
	}
	for _, rs := range reviewStatuses {
		if s == rs {
			return true
		}
	}
	return false
}

func normalizeStatus(s string) string {
	// Convert to lowercase and trim whitespace
	result := make([]byte, 0, len(s))
	for i := range len(s) {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c = c + 32 // Convert to lowercase
		}
		result = append(result, c)
	}
	// Trim leading/trailing spaces
	start := 0
	for start < len(result) && result[start] == ' ' {
		start++
	}
	end := len(result)
	for end > start && result[end-1] == ' ' {
		end--
	}
	return string(result[start:end])
}
