package workflow

import (
	"os"
	"path/filepath"
	"testing"

	"thoreinstein.com/rig/pkg/config"
)

func TestIsBeadsTicket(t *testing.T) {
	tests := []struct {
		name     string
		ticketID string
		want     bool
	}{
		// Valid beads tickets (alphanumeric suffix with at least one letter)
		{name: "lowercase alpha suffix", ticketID: "rig-k6n", want: true},
		{name: "mixed case alpha suffix", ticketID: "rig-K6N", want: true},
		{name: "letters only suffix", ticketID: "rig-abc", want: true},
		{name: "letters and digits suffix", ticketID: "beads-xyz123", want: true},
		{name: "digits then letters", ticketID: "rig-2o1", want: true},
		{name: "mixed case suffix", ticketID: "proj-AbC", want: true},
		{name: "single letter suffix", ticketID: "proj-a", want: true},
		{name: "letter at end", ticketID: "proj-123a", want: true},
		{name: "letter at start", ticketID: "proj-a123", want: true},

		// Invalid - these are JIRA style (numeric only suffix)
		{name: "numeric only suffix", ticketID: "PROJ-123", want: false},
		{name: "lowercase numeric suffix", ticketID: "proj-456", want: false},
		{name: "single digit suffix", ticketID: "ABC-1", want: false},

		// Invalid - malformed
		{name: "no dash", ticketID: "main", want: false},
		{name: "no prefix", ticketID: "-123", want: false},
		{name: "no suffix", ticketID: "PROJ-", want: false},
		{name: "empty string", ticketID: "", want: false},
		{name: "too short", ticketID: "AB", want: false},
		{name: "numeric prefix", ticketID: "123-ABC", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsBeadsTicket(tt.ticketID)
			if got != tt.want {
				t.Errorf("IsBeadsTicket(%q) = %v, want %v", tt.ticketID, got, tt.want)
			}
		})
	}
}

func TestIsJiraTicket(t *testing.T) {
	tests := []struct {
		name     string
		ticketID string
		want     bool
	}{
		// Valid JIRA tickets (numeric only suffix)
		{name: "uppercase prefix", ticketID: "PROJ-123", want: true},
		{name: "lowercase prefix", ticketID: "proj-456", want: true},
		{name: "mixed case prefix", ticketID: "Proj-789", want: true},
		{name: "single digit", ticketID: "ABC-1", want: true},
		{name: "large number", ticketID: "FRAAS-99999", want: true},
		{name: "single letter prefix", ticketID: "A-1", want: true},

		// Invalid - these are beads style (alphanumeric suffix)
		{name: "alpha suffix", ticketID: "rig-k6n", want: false},
		{name: "letters in suffix", ticketID: "rig-abc", want: false},
		{name: "mixed suffix", ticketID: "beads-xyz123", want: false},
		{name: "letter at end of suffix", ticketID: "proj-123a", want: false},

		// Invalid - malformed
		{name: "no dash", ticketID: "main", want: false},
		{name: "no prefix", ticketID: "-123", want: false},
		{name: "no suffix", ticketID: "PROJ-", want: false},
		{name: "empty string", ticketID: "", want: false},
		{name: "too short", ticketID: "AB", want: false},
		{name: "numeric prefix", ticketID: "123-ABC", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsJiraTicket(tt.ticketID)
			if got != tt.want {
				t.Errorf("IsJiraTicket(%q) = %v, want %v", tt.ticketID, got, tt.want)
			}
		})
	}
}

func TestTicketRouter_RouteTicket(t *testing.T) {
	// Create a temporary directory structure for testing beads detection
	tmpDir := t.TempDir()
	beadsProject := filepath.Join(tmpDir, "beads-project")
	nonBeadsProject := filepath.Join(tmpDir, "non-beads-project")

	// Create beads project structure
	beadsDir := filepath.Join(beadsProject, ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatalf("failed to create .beads directory: %v", err)
	}
	beadsFile := filepath.Join(beadsDir, "beads.jsonl")
	if err := os.WriteFile(beadsFile, []byte("{}"), 0o644); err != nil {
		t.Fatalf("failed to create beads.jsonl: %v", err)
	}

	// Create non-beads project structure
	if err := os.MkdirAll(nonBeadsProject, 0o755); err != nil {
		t.Fatalf("failed to create non-beads project directory: %v", err)
	}

	tests := []struct {
		name         string
		beadsEnabled bool
		jiraEnabled  bool
		projectPath  string
		ticketID     string
		want         TicketSource
	}{
		// Both systems enabled, beads project
		{
			name:         "beads ticket in beads project with both enabled",
			beadsEnabled: true,
			jiraEnabled:  true,
			projectPath:  beadsProject,
			ticketID:     "rig-k6n",
			want:         TicketSourceBeads,
		},
		{
			name:         "jira ticket in beads project with both enabled",
			beadsEnabled: true,
			jiraEnabled:  true,
			projectPath:  beadsProject,
			ticketID:     "PROJ-123",
			want:         TicketSourceJira,
		},

		// Both systems enabled, non-beads project
		{
			name:         "beads-style ticket in non-beads project falls through to unknown",
			beadsEnabled: true,
			jiraEnabled:  true,
			projectPath:  nonBeadsProject,
			ticketID:     "rig-k6n",
			want:         TicketSourceUnknown,
		},
		{
			name:         "jira ticket in non-beads project",
			beadsEnabled: true,
			jiraEnabled:  true,
			projectPath:  nonBeadsProject,
			ticketID:     "PROJ-123",
			want:         TicketSourceJira,
		},

		// Only beads enabled
		{
			name:         "beads ticket with only beads enabled",
			beadsEnabled: true,
			jiraEnabled:  false,
			projectPath:  beadsProject,
			ticketID:     "rig-k6n",
			want:         TicketSourceBeads,
		},
		{
			name:         "jira-style ticket with only beads enabled",
			beadsEnabled: true,
			jiraEnabled:  false,
			projectPath:  beadsProject,
			ticketID:     "PROJ-123",
			want:         TicketSourceUnknown,
		},

		// Only jira enabled
		{
			name:         "jira ticket with only jira enabled",
			beadsEnabled: false,
			jiraEnabled:  true,
			projectPath:  beadsProject,
			ticketID:     "PROJ-123",
			want:         TicketSourceJira,
		},
		{
			name:         "beads-style ticket with only jira enabled",
			beadsEnabled: false,
			jiraEnabled:  true,
			projectPath:  beadsProject,
			ticketID:     "rig-k6n",
			want:         TicketSourceUnknown,
		},

		// Neither system enabled
		{
			name:         "any ticket with neither enabled",
			beadsEnabled: false,
			jiraEnabled:  false,
			projectPath:  beadsProject,
			ticketID:     "PROJ-123",
			want:         TicketSourceUnknown,
		},
		{
			name:         "beads-style with neither enabled",
			beadsEnabled: false,
			jiraEnabled:  false,
			projectPath:  beadsProject,
			ticketID:     "rig-k6n",
			want:         TicketSourceUnknown,
		},

		// Invalid ticket formats
		{
			name:         "invalid ticket format",
			beadsEnabled: true,
			jiraEnabled:  true,
			projectPath:  beadsProject,
			ticketID:     "main",
			want:         TicketSourceUnknown,
		},
		{
			name:         "empty ticket ID",
			beadsEnabled: true,
			jiraEnabled:  true,
			projectPath:  beadsProject,
			ticketID:     "",
			want:         TicketSourceUnknown,
		},

		// Empty project path
		{
			name:         "beads ticket with empty project path",
			beadsEnabled: true,
			jiraEnabled:  true,
			projectPath:  "",
			ticketID:     "rig-k6n",
			want:         TicketSourceUnknown,
		},
		{
			name:         "jira ticket with empty project path",
			beadsEnabled: true,
			jiraEnabled:  true,
			projectPath:  "",
			ticketID:     "PROJ-123",
			want:         TicketSourceJira,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Beads: config.BeadsConfig{Enabled: tt.beadsEnabled},
				Jira:  config.JiraConfig{Enabled: tt.jiraEnabled},
				AI:    config.AIConfig{Enabled: false},
			}
			router := NewTicketRouter(cfg, tt.projectPath, false)
			got := router.RouteTicket(tt.ticketID)
			if got != tt.want {
				t.Errorf("RouteTicket(%q) = %v, want %v", tt.ticketID, got, tt.want)
			}
		})
	}
}

func TestTicketRouter_RouteTicket_NestedBeadsProject(t *testing.T) {
	// Test that FindBeadsRoot works for nested directories
	tmpDir := t.TempDir()
	beadsRoot := filepath.Join(tmpDir, "project")
	nestedDir := filepath.Join(beadsRoot, "src", "pkg", "feature")

	// Create beads project structure
	beadsDir := filepath.Join(beadsRoot, ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatalf("failed to create .beads directory: %v", err)
	}
	beadsFile := filepath.Join(beadsDir, "beads.jsonl")
	if err := os.WriteFile(beadsFile, []byte("{}"), 0o644); err != nil {
		t.Fatalf("failed to create beads.jsonl: %v", err)
	}

	// Create nested directory
	if err := os.MkdirAll(nestedDir, 0o755); err != nil {
		t.Fatalf("failed to create nested directory: %v", err)
	}

	cfg := &config.Config{
		Beads: config.BeadsConfig{Enabled: true},
		Jira:  config.JiraConfig{Enabled: true},
		AI:    config.AIConfig{Enabled: false},
	}

	// Route from nested directory should find beads root
	router := NewTicketRouter(cfg, nestedDir, false)
	got := router.RouteTicket("rig-k6n")
	if got != TicketSourceBeads {
		t.Errorf("RouteTicket from nested dir = %v, want %v", got, TicketSourceBeads)
	}

	// JIRA ticket should still route to JIRA
	got = router.RouteTicket("PROJ-123")
	if got != TicketSourceJira {
		t.Errorf("RouteTicket JIRA from nested dir = %v, want %v", got, TicketSourceJira)
	}
}

func TestNewTicketRouter(t *testing.T) {
	cfg := &config.Config{
		Beads: config.BeadsConfig{Enabled: true, CliCommand: "bd"},
		Jira:  config.JiraConfig{Enabled: false, BaseURL: "https://jira.example.com"},
		AI:    config.AIConfig{Enabled: false},
	}

	router := NewTicketRouter(cfg, "/test/path", true)

	if !router.beadsEnabled {
		t.Error("expected beadsEnabled to be true")
	}
	if router.jiraEnabled {
		t.Error("expected jiraEnabled to be false")
	}
	if router.projectPath != "/test/path" {
		t.Errorf("projectPath = %q, want %q", router.projectPath, "/test/path")
	}
	if !router.verbose {
		t.Error("expected verbose to be true")
	}
}

func TestTicketSource_String(t *testing.T) {
	tests := []struct {
		source TicketSource
		want   string
	}{
		{TicketSourceUnknown, "unknown"},
		{TicketSourceBeads, "beads"},
		{TicketSourceJira, "jira"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.source.String(); got != tt.want {
				t.Errorf("TicketSource.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFindDash(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"PROJ-123", 4},
		{"A-1", 1},
		{"-123", 0},
		{"nodash", -1},
		{"", -1},
		{"multiple-dashes-here", 8},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := findDash(tt.input); got != tt.want {
				t.Errorf("findDash(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
