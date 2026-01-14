package jira

import "strings"

// WorkflowPhase represents a phase in the PR workflow.
type WorkflowPhase string

const (
	// PhaseNotStarted indicates work has not begun (e.g., "Open", "To Do", "Backlog").
	PhaseNotStarted WorkflowPhase = "not_started"

	// PhaseInProgress indicates work is actively being done (e.g., "In Progress", "In Development").
	PhaseInProgress WorkflowPhase = "in_progress"

	// PhaseInReview indicates work is under review (e.g., "In Review", "Code Review").
	PhaseInReview WorkflowPhase = "in_review"

	// PhaseDone indicates work is complete (e.g., "Done", "Closed", "Resolved").
	PhaseDone WorkflowPhase = "done"
)

// statusToPhaseMap maps lowercase Jira status names to workflow phases.
var statusToPhaseMap = map[string]WorkflowPhase{
	// Not started statuses
	"open":                     PhaseNotStarted,
	"to do":                    PhaseNotStarted,
	"backlog":                  PhaseNotStarted,
	"new":                      PhaseNotStarted,
	"reopened":                 PhaseNotStarted,
	"ready":                    PhaseNotStarted,
	"selected":                 PhaseNotStarted,
	"selected for development": PhaseNotStarted,

	// In progress statuses
	"in progress":    PhaseInProgress,
	"in development": PhaseInProgress,
	"in dev":         PhaseInProgress,
	"development":    PhaseInProgress,
	"working":        PhaseInProgress,
	"implementing":   PhaseInProgress,
	"active":         PhaseInProgress,

	// In review statuses
	"in review":        PhaseInReview,
	"code review":      PhaseInReview,
	"review":           PhaseInReview,
	"ready for review": PhaseInReview,
	"pending review":   PhaseInReview,
	"awaiting review":  PhaseInReview,
	"under review":     PhaseInReview,
	"qa":               PhaseInReview,
	"testing":          PhaseInReview,
	"in qa":            PhaseInReview,

	// Done statuses
	"done":     PhaseDone,
	"closed":   PhaseDone,
	"resolved": PhaseDone,
	"complete": PhaseDone,
	"finished": PhaseDone,
	"released": PhaseDone,
	"deployed": PhaseDone,
	"verified": PhaseDone,
}

// phaseToStatusMap maps workflow phases to their preferred Jira status names.
// These are the most common/standard names used in Jira workflows.
var phaseToStatusMap = map[WorkflowPhase]string{
	PhaseNotStarted: "To Do",
	PhaseInProgress: "In Progress",
	PhaseInReview:   "In Review",
	PhaseDone:       "Done",
}

// MapStatusToPhase maps a Jira status name to a workflow phase.
// The mapping is case-insensitive. Returns PhaseNotStarted for unknown statuses.
func MapStatusToPhase(status string) WorkflowPhase {
	statusLower := strings.ToLower(strings.TrimSpace(status))

	if phase, ok := statusToPhaseMap[statusLower]; ok {
		return phase
	}

	// Fallback: try to infer from keywords
	switch {
	case strings.Contains(statusLower, "progress") || strings.Contains(statusLower, "dev"):
		return PhaseInProgress
	case strings.Contains(statusLower, "review") || strings.Contains(statusLower, "qa") || strings.Contains(statusLower, "test"):
		return PhaseInReview
	case strings.Contains(statusLower, "done") || strings.Contains(statusLower, "close") || strings.Contains(statusLower, "resolv"):
		return PhaseDone
	default:
		return PhaseNotStarted
	}
}

// GetTargetStatus returns the preferred Jira status name for a workflow phase.
// This is the inverse of MapStatusToPhase and returns the most common status name.
func GetTargetStatus(phase WorkflowPhase) string {
	if status, ok := phaseToStatusMap[phase]; ok {
		return status
	}
	return "To Do" // Default fallback
}
