package jira

import (
	"testing"
)

func TestMapStatusToPhase(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   WorkflowPhase
	}{
		// Not started statuses
		{"Open", "Open", PhaseNotStarted},
		{"To Do", "To Do", PhaseNotStarted},
		{"Backlog", "Backlog", PhaseNotStarted},
		{"New", "new", PhaseNotStarted},
		{"Reopened", "Reopened", PhaseNotStarted},
		{"Selected for Development", "Selected for Development", PhaseNotStarted},

		// In progress statuses
		{"In Progress", "In Progress", PhaseInProgress},
		{"In Development", "In Development", PhaseInProgress},
		{"In Dev", "in dev", PhaseInProgress},
		{"Development", "Development", PhaseInProgress},
		{"Working", "Working", PhaseInProgress},
		{"Active", "active", PhaseInProgress},

		// In review statuses
		{"In Review", "In Review", PhaseInReview},
		{"Code Review", "Code Review", PhaseInReview},
		{"Review", "Review", PhaseInReview},
		{"Ready for Review", "Ready for Review", PhaseInReview},
		{"QA", "QA", PhaseInReview},
		{"Testing", "testing", PhaseInReview},
		{"In QA", "In QA", PhaseInReview},

		// Done statuses
		{"Done", "Done", PhaseDone},
		{"Closed", "Closed", PhaseDone},
		{"Resolved", "resolved", PhaseDone},
		{"Complete", "Complete", PhaseDone},
		{"Released", "Released", PhaseDone},
		{"Deployed", "deployed", PhaseDone},

		// Case insensitivity
		{"Case insensitive - IN PROGRESS", "IN PROGRESS", PhaseInProgress},
		{"Case insensitive - done", "done", PhaseDone},

		// Whitespace handling
		{"Leading/trailing whitespace", "  In Progress  ", PhaseInProgress},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MapStatusToPhase(tt.status)
			if got != tt.want {
				t.Errorf("MapStatusToPhase(%q) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

func TestMapStatusToPhase_Fallback(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   WorkflowPhase
	}{
		// Keyword-based fallback for unknown statuses
		{"Contains 'progress'", "Custom In Progress Status", PhaseInProgress},
		{"Contains 'dev'", "Under Development", PhaseInProgress},
		{"Contains 'review'", "Peer Review Required", PhaseInReview},
		{"Contains 'qa'", "In QA Phase", PhaseInReview},
		{"Contains 'test'", "Under Testing", PhaseInReview},
		{"Contains 'done'", "Almost Done", PhaseDone},
		{"Contains 'close'", "Ready to Close", PhaseDone},
		{"Contains 'resolv'", "Being Resolved", PhaseDone},

		// Unknown status defaults to NotStarted
		{"Unknown status", "Unknown Status XYZ", PhaseNotStarted},
		{"Empty string", "", PhaseNotStarted},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MapStatusToPhase(tt.status)
			if got != tt.want {
				t.Errorf("MapStatusToPhase(%q) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

func TestGetTargetStatus(t *testing.T) {
	tests := []struct {
		phase WorkflowPhase
		want  string
	}{
		{PhaseNotStarted, "To Do"},
		{PhaseInProgress, "In Progress"},
		{PhaseInReview, "In Review"},
		{PhaseDone, "Done"},
	}

	for _, tt := range tests {
		t.Run(string(tt.phase), func(t *testing.T) {
			got := GetTargetStatus(tt.phase)
			if got != tt.want {
				t.Errorf("GetTargetStatus(%q) = %q, want %q", tt.phase, got, tt.want)
			}
		})
	}
}

func TestGetTargetStatus_UnknownPhase(t *testing.T) {
	got := GetTargetStatus(WorkflowPhase("unknown"))
	if got != "To Do" {
		t.Errorf("GetTargetStatus(unknown) = %q, want %q", got, "To Do")
	}
}

func TestWorkflowPhaseConstants(t *testing.T) {
	// Verify constant values haven't changed unexpectedly
	tests := []struct {
		phase WorkflowPhase
		want  string
	}{
		{PhaseNotStarted, "not_started"},
		{PhaseInProgress, "in_progress"},
		{PhaseInReview, "in_review"},
		{PhaseDone, "done"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if string(tt.phase) != tt.want {
				t.Errorf("WorkflowPhase constant = %q, want %q", string(tt.phase), tt.want)
			}
		})
	}
}
