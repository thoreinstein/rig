package workflow

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	rigerrors "thoreinstein.com/rig/pkg/errors"
	"thoreinstein.com/rig/pkg/github"
)

// runPreflight checks prerequisites before merge.
//
// Checks performed:
// - PR exists and is open
// - PR is approved by at least one reviewer
// - CI checks are passing
// - (Optional) Jira ticket is in "In Review" status
func (e *Engine) runPreflight(ctx context.Context, wf *MergeWorkflow, opts MergeOptions) error {
	result, err := e.Preflight(ctx, wf.PRNumber, opts)
	if err != nil {
		return err
	}

	// Log warnings
	for _, w := range result.Warnings {
		e.logger.Warn(w)
	}

	if !result.IsReady() {
		return rigerrors.NewWorkflowError("preflight", result.FailureReason)
	}

	e.log("Preflight checks passed")
	return nil
}

// runGather collects context about the PR.
//
// This step fetches:
// - Full PR details from GitHub
// - Jira ticket details (if not skipped)
// - Commit history
// - Timeline of events
func (e *Engine) runGather(ctx context.Context, wf *MergeWorkflow, opts MergeOptions) error {
	// Fetch PR details
	pr, err := e.github.GetPR(ctx, wf.PRNumber)
	if err != nil {
		return rigerrors.NewWorkflowErrorWithCause("gather", "failed to fetch PR details", err)
	}
	wf.Context.PR = pr
	wf.Context.BranchName = pr.HeadBranch
	wf.Context.BaseBranch = pr.BaseBranch

	// Extract ticket from branch name
	ticket := extractTicketFromBranch(pr.HeadBranch)
	wf.Ticket = ticket

	// Fetch Jira ticket details if not skipped and ticket routes to Jira
	if !opts.SkipJira && e.jira != nil && e.jira.IsAvailable() && ticket != "" {
		source := e.router.RouteTicket(ticket)
		if source == TicketSourceJira {
			ticketInfo, err := e.jira.FetchTicketDetails(ticket)
			if err != nil {
				e.logger.Warn("failed to fetch Jira ticket", "ticket", ticket, "error", err)
			} else {
				wf.Context.Ticket = ticketInfo
			}
		}
	}

	// Get commit history
	commits, err := e.getCommitHistory(ctx, pr)
	if err != nil {
		e.logger.Warn("failed to get commit history", "error", err)
	} else {
		wf.Context.Commits = commits
	}

	// Build timeline
	wf.Context.Timeline = e.buildTimeline(pr, wf.Context.Commits)

	e.log("Gathered context: %d commits, %d timeline entries", len(wf.Context.Commits), len(wf.Context.Timeline))
	return nil
}

// getCommitHistory fetches commit information for the PR.
func (e *Engine) getCommitHistory(ctx context.Context, pr *github.PRInfo) ([]CommitInfo, error) {
	// Use git log to get commits between base and head
	// This works from within a worktree
	// Branch names are validated by GitHub API (alphanumeric, -, _, /)
	revRange := pr.BaseBranch + ".." + pr.HeadBranch
	output, err := exec.CommandContext(ctx, "git", "log", "--format=%H|%s|%an|%aI", revRange).Output()
	if err != nil {
		// Fallback: try origin/base..HEAD
		revRange = "origin/" + pr.BaseBranch + "..HEAD"
		output, err = exec.CommandContext(ctx, "git", "log", "--format=%H|%s|%an|%aI", revRange).Output()
		if err != nil {
			return nil, rigerrors.Wrapf(err, "failed to get commit history")
		}
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	commits := make([]CommitInfo, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 4)
		if len(parts) < 4 {
			continue
		}
		date, _ := time.Parse(time.RFC3339, parts[3])
		commits = append(commits, CommitInfo{
			SHA:     parts[0],
			Message: parts[1],
			Author:  parts[2],
			Date:    date,
		})
	}

	return commits, nil
}

// buildTimeline creates a timeline from PR and commit data.
func (e *Engine) buildTimeline(pr *github.PRInfo, commits []CommitInfo) []TimelineEntry {
	// Pre-allocate timeline with estimated capacity
	timeline := make([]TimelineEntry, 0, len(commits)+2)

	// Add PR creation
	timeline = append(timeline, TimelineEntry{
		Time:    pr.CreatedAt,
		Type:    "pr_created",
		Summary: fmt.Sprintf("PR #%d created: %s", pr.Number, pr.Title),
	})

	// Add commits
	for _, c := range commits {
		timeline = append(timeline, TimelineEntry{
			Time:    c.Date,
			Type:    "commit",
			Summary: fmt.Sprintf("Commit %s: %s", c.SHA[:7], c.Message),
		})
	}

	// Add PR update timestamp
	if pr.UpdatedAt.After(pr.CreatedAt) {
		timeline = append(timeline, TimelineEntry{
			Time:    pr.UpdatedAt,
			Type:    "pr_updated",
			Summary: "PR updated",
		})
	}

	// Sort timeline by time
	slices.SortFunc(timeline, func(a, b TimelineEntry) int {
		return a.Time.Compare(b.Time)
	})

	return timeline
}

// runDebrief triggers AI debrief.
//
// This step can be skipped with opts.SkipAI.
// When enabled, it delegates to the debrief package for interactive Q&A.
func (e *Engine) runDebrief(ctx context.Context, wf *MergeWorkflow, opts MergeOptions) error {
	if opts.SkipAI {
		e.log("Skipping AI debrief (--skip-ai)")
		return nil
	}

	// Check if AI is enabled in config
	if !e.cfg.AI.Enabled {
		e.log("Skipping AI debrief (AI disabled in config)")
		return nil
	}

	// TODO: Integrate with pkg/debrief when available
	// For now, log that debrief would run here
	e.log("AI debrief step (integration pending with pkg/debrief)")

	// Placeholder: In the future, this will:
	// 1. Create a debrief session with context
	// 2. Run interactive Q&A
	// 3. Store notes in wf.Context.DebriefNotes

	return nil
}

// runMerge executes the actual merge on GitHub.
//
// This step:
// - Merges the PR via GitHub API
// - Optionally deletes the branch
func (e *Engine) runMerge(ctx context.Context, wf *MergeWorkflow, opts MergeOptions) error {
	// Determine merge method
	mergeMethod := opts.MergeMethod
	if mergeMethod == "" {
		mergeMethod = e.cfg.GitHub.DefaultMergeMethod
	}
	if mergeMethod == "" {
		mergeMethod = "squash" // Default to squash if nothing specified
	}

	// Determine if we should delete the branch
	deleteBranch := e.cfg.GitHub.DeleteBranchOnMerge
	if opts.DeleteBranch != nil {
		deleteBranch = *opts.DeleteBranch
	}

	e.log("Merging PR #%d using method: %s", wf.PRNumber, mergeMethod)

	// Execute the merge
	mergeOpts := github.MergeOptions{
		Method:       mergeMethod,
		DeleteBranch: deleteBranch,
	}

	if err := e.github.MergePR(ctx, wf.PRNumber, mergeOpts); err != nil {
		return rigerrors.NewWorkflowErrorWithCause("merge", "failed to merge PR", err)
	}

	e.log("PR #%d merged successfully", wf.PRNumber)
	return nil
}

// runCloseout performs post-merge cleanup.
//
// This step:
// - Transitions Jira ticket to Done (if not skipped)
// - Appends debrief notes to note file
// - Kills tmux session (if configured)
// - Queues worktree for cleanup (if not kept)
func (e *Engine) runCloseout(ctx context.Context, wf *MergeWorkflow, opts MergeOptions) error {
	var errs []string

	// Transition Jira to Done (only for Jira tickets)
	if !opts.SkipJira && wf.Ticket != "" && e.jira != nil && e.jira.IsAvailable() {
		source := e.router.RouteTicket(wf.Ticket)
		if source == TicketSourceJira {
			if err := e.transitionJiraToDone(wf.Ticket); err != nil {
				errs = append(errs, fmt.Sprintf("jira transition: %v", err))
				e.logger.Warn("failed to transition Jira", "error", err)
			} else {
				e.log("Transitioned %s to Done", wf.Ticket)
			}
		}
	}

	// Append debrief notes to note file
	if wf.Context.DebriefNotes != "" && opts.NotePath != "" {
		if err := e.appendDebriefNotes(opts.NotePath, wf.Context.DebriefNotes); err != nil {
			errs = append(errs, fmt.Sprintf("append notes: %v", err))
			e.logger.Warn("failed to append debrief notes", "error", err)
		} else {
			e.log("Appended debrief notes to %s", opts.NotePath)
		}
	}

	// Kill tmux session if configured
	killSession := e.cfg.Workflow.KillSession
	if opts.KillSession != nil {
		killSession = *opts.KillSession
	}
	if killSession && wf.Ticket != "" {
		if err := e.killTmuxSession(wf.Ticket); err != nil {
			// Don't fail on tmux errors - session might not exist
			e.logger.Debug("tmux session not killed", "error", err)
		} else {
			e.log("Killed tmux session for %s", wf.Ticket)
		}
	}

	// Queue worktree for cleanup
	if !opts.KeepWorktree && e.cfg.Workflow.QueueWorktreeCleanup && wf.Worktree != "" {
		if err := e.queueWorktreeCleanup(wf.Worktree); err != nil {
			errs = append(errs, fmt.Sprintf("queue worktree: %v", err))
			e.logger.Warn("failed to queue worktree cleanup", "error", err)
		} else {
			e.log("Queued worktree for cleanup: %s", wf.Worktree)
		}
	}

	// Report non-fatal errors
	if len(errs) > 0 {
		e.logger.Warn("closeout completed with warnings", "warnings", strings.Join(errs, "; "))
	}

	return nil
}

// transitionJiraToDone transitions a Jira ticket to "Done" status.
func (e *Engine) transitionJiraToDone(ticket string) error {
	// Try common "Done" status names
	doneStatuses := []string{"Done", "Closed", "Complete", "Resolved"}

	for _, status := range doneStatuses {
		err := e.jira.TransitionTicketByName(ticket, status)
		if err == nil {
			return nil
		}
		// Log and try next status
		e.logger.Debug("transition attempt failed", "status", status, "error", err)
	}

	return rigerrors.NewWorkflowError("closeout", "could not transition ticket to done status")
}

// appendDebriefNotes appends notes to a file.
func (e *Engine) appendDebriefNotes(notePath, notes string) (err error) {
	// Open file for appending, create if doesn't exist
	f, err := os.OpenFile(notePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return rigerrors.Wrapf(err, "failed to open note file")
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = rigerrors.Wrap(cerr, "failed to close note file")
		}
	}()

	// Add a header for the debrief section
	content := fmt.Sprintf("\n\n## Merge Debrief\n\n%s\n", notes)
	if _, err := f.WriteString(content); err != nil {
		return rigerrors.Wrapf(err, "failed to write notes")
	}

	return nil
}

// killTmuxSession kills a tmux session by ticket name.
func (e *Engine) killTmuxSession(ticket string) error {
	// Construct session name using configured prefix
	sessionName := ticket
	if e.cfg.Tmux.SessionPrefix != "" {
		sessionName = e.cfg.Tmux.SessionPrefix + ticket
	}

	cmd := exec.Command("tmux", "kill-session", "-t", sessionName)
	return cmd.Run()
}

// queueWorktreeCleanup marks a worktree for later cleanup.
// This creates a marker file in the .rig directory.
func (e *Engine) queueWorktreeCleanup(worktreePath string) error {
	// Create .rig directory if it doesn't exist
	rigDir := filepath.Join(worktreePath, ".rig")
	if err := os.MkdirAll(rigDir, 0700); err != nil {
		return rigerrors.Wrapf(err, "failed to create .rig directory")
	}

	// Create cleanup marker file
	markerPath := filepath.Join(rigDir, "cleanup-queued")
	content := fmt.Sprintf("queued_at: %s\n", time.Now().Format(time.RFC3339))

	if err := os.WriteFile(markerPath, []byte(content), 0600); err != nil {
		return rigerrors.Wrapf(err, "failed to write cleanup marker")
	}

	return nil
}
