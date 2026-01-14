// Package workflow provides the merge workflow engine for PR lifecycle management.
//
// The workflow orchestrates the complete PR merge process:
// 1. Pre-flight checks (PR approved, checks pass, Jira status)
// 2. Context gathering (PR details, commits, timeline)
// 3. AI debrief (optional interactive Q&A)
// 4. Merge execution (GitHub merge, branch cleanup)
// 5. Close-out (Jira transition, notes, tmux cleanup, worktree queue)
//
// Workflows support checkpointing for resume after interruption.
package workflow

import (
	"time"

	"thoreinstein.com/rig/pkg/github"
	"thoreinstein.com/rig/pkg/jira"
)

// MergeWorkflow represents the state of a merge operation.
type MergeWorkflow struct {
	PRNumber       int
	Ticket         string
	Worktree       string
	StartedAt      time.Time
	CompletedSteps []Step
	CurrentStep    Step
	Context        *WorkflowContext
}

// Step represents a workflow step.
type Step string

const (
	// StepPreflight checks prerequisites before merge.
	StepPreflight Step = "preflight"
	// StepGather collects context about the PR.
	StepGather Step = "gather"
	// StepDebrief runs interactive AI debrief.
	StepDebrief Step = "debrief"
	// StepMerge executes the actual merge.
	StepMerge Step = "merge"
	// StepCloseout performs post-merge cleanup.
	StepCloseout Step = "closeout"
)

// AllSteps returns all workflow steps in execution order.
func AllSteps() []Step {
	return []Step{StepPreflight, StepGather, StepDebrief, StepMerge, StepCloseout}
}

// String returns the string representation of the step.
func (s Step) String() string {
	return string(s)
}

// WorkflowContext holds gathered data for the workflow.
type WorkflowContext struct {
	PR           *github.PRInfo   `json:"pr,omitempty"`
	Ticket       *jira.TicketInfo `json:"ticket,omitempty"`
	Commits      []CommitInfo     `json:"commits,omitempty"`
	Timeline     []TimelineEntry  `json:"timeline,omitempty"`
	BranchName   string           `json:"branch_name"`
	BaseBranch   string           `json:"base_branch"`
	DebriefNotes string           `json:"debrief_notes,omitempty"`
}

// CommitInfo represents a commit in the PR.
type CommitInfo struct {
	SHA     string    `json:"sha"`
	Message string    `json:"message"`
	Author  string    `json:"author"`
	Date    time.Time `json:"date"`
}

// TimelineEntry represents a timeline event.
type TimelineEntry struct {
	Time    time.Time `json:"time"`
	Type    string    `json:"type"` // "commit", "review", "comment", "status"
	Summary string    `json:"summary"`
}

// MergeOptions configures the workflow execution.
type MergeOptions struct {
	// SkipAI disables the AI debrief step.
	SkipAI bool

	// SkipJira disables Jira-related operations (preflight check and closeout transition).
	SkipJira bool

	// SkipApproval bypasses the PR approval check (for self-authored PRs).
	SkipApproval bool

	// KeepWorktree prevents worktree cleanup during closeout.
	KeepWorktree bool

	// MergeMethod specifies how to merge: "merge", "squash", or "rebase".
	// Empty string uses the repository or config default.
	MergeMethod string

	// SkipConfirmation bypasses interactive confirmation prompts.
	SkipConfirmation bool

	// DeleteBranch controls whether to delete the branch after merge.
	// When nil, uses the config default.
	DeleteBranch *bool

	// KillSession controls whether to kill the tmux session.
	// When nil, uses the config default.
	KillSession *bool

	// NotePath specifies where to append debrief notes.
	NotePath string
}

// PreflightResult holds the results of preflight checks.
type PreflightResult struct {
	PRExists        bool
	PROpen          bool
	PRApproved      bool
	ApprovalSkipped bool
	ChecksPassing   bool
	JiraInReview    bool
	JiraSkipped     bool
	FailureReason   string
	Warnings        []string
}

// IsReady returns true if all preflight checks passed.
func (r *PreflightResult) IsReady() bool {
	if !r.PRExists || !r.PROpen {
		return false
	}
	if !r.PRApproved && !r.ApprovalSkipped {
		return false
	}
	if !r.ChecksPassing {
		return false
	}
	if !r.JiraSkipped && !r.JiraInReview {
		return false
	}
	return true
}
