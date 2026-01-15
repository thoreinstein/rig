package beads

import "testing"

func TestIsValidStatus(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   bool
	}{
		{"open", "open", true},
		{"in_progress", "in_progress", true},
		{"closed", "closed", true},
		{"invalid", "invalid", false},
		{"empty", "", false},
		{"uppercase", "OPEN", false},
		{"mixed case", "Open", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidStatus(tt.status)
			if got != tt.want {
				t.Errorf("IsValidStatus(%q) = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}

func TestIsValidType(t *testing.T) {
	tests := []struct {
		name      string
		issueType string
		want      bool
	}{
		{"task", "task", true},
		{"bug", "bug", true},
		{"feature", "feature", true},
		{"epic", "epic", true},
		{"invalid", "invalid", false},
		{"empty", "", false},
		{"uppercase", "TASK", false},
		{"story", "story", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidType(tt.issueType)
			if got != tt.want {
				t.Errorf("IsValidType(%q) = %v, want %v", tt.issueType, got, tt.want)
			}
		})
	}
}
