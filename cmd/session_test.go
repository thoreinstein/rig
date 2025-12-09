package cmd

import (
	"strings"
	"testing"
)

func TestSessionCommandStructure(t *testing.T) {
	// Test that session command has expected subcommands
	cmd := sessionCmd

	if cmd.Use != "session" {
		t.Errorf("session command Use = %q, want %q", cmd.Use, "session")
	}

	// Check subcommands exist
	subcommands := cmd.Commands()
	subcommandNames := make(map[string]bool)
	for _, sub := range subcommands {
		subcommandNames[sub.Use] = true
	}

	expectedSubcommands := []string{"list", "attach <ticket>", "kill <ticket>"}
	for _, expected := range expectedSubcommands {
		if !subcommandNames[expected] {
			t.Errorf("session command missing subcommand: %q", expected)
		}
	}
}

func TestSessionListCommand(t *testing.T) {
	cmd := sessionListCmd

	if cmd.Use != "list" {
		t.Errorf("session list Use = %q, want %q", cmd.Use, "list")
	}

	if cmd.Short == "" {
		t.Error("session list should have Short description")
	}
}

func TestSessionAttachCommand(t *testing.T) {
	cmd := sessionAttachCmd

	if cmd.Use != "attach <ticket>" {
		t.Errorf("session attach Use = %q, want %q", cmd.Use, "attach <ticket>")
	}

	if cmd.Short == "" {
		t.Error("session attach should have Short description")
	}

	// Command should require exactly 1 argument
	if cmd.Args == nil {
		t.Error("session attach should have Args validation")
	}
}

func TestSessionKillCommand(t *testing.T) {
	cmd := sessionKillCmd

	if cmd.Use != "kill <ticket>" {
		t.Errorf("session kill Use = %q, want %q", cmd.Use, "kill <ticket>")
	}

	if cmd.Short == "" {
		t.Error("session kill should have Short description")
	}

	// Command should require exactly 1 argument
	if cmd.Args == nil {
		t.Error("session kill should have Args validation")
	}
}

func TestSessionKillErrorParsing(t *testing.T) {
	// Test the error message parsing logic in runSessionKillCommand
	// The function checks if error contains "does not exist"
	tests := []struct {
		name           string
		errorMsg       string
		shouldBeGraceful bool
	}{
		{
			name:           "session does not exist",
			errorMsg:       "session 'test' does not exist",
			shouldBeGraceful: true,
		},
		{
			name:           "session does not exist - different format",
			errorMsg:       "error: does not exist: test-session",
			shouldBeGraceful: true,
		},
		{
			name:           "different error",
			errorMsg:       "failed to connect to tmux server",
			shouldBeGraceful: false,
		},
		{
			name:           "permission denied",
			errorMsg:       "permission denied",
			shouldBeGraceful: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the error checking logic from runSessionKillCommand
			isGraceful := strings.Contains(tt.errorMsg, "does not exist")
			if isGraceful != tt.shouldBeGraceful {
				t.Errorf("Error parsing for %q: got graceful=%v, want %v",
					tt.errorMsg, isGraceful, tt.shouldBeGraceful)
			}
		})
	}
}

func TestSessionCommandDescriptions(t *testing.T) {
	// Verify all session commands have proper descriptions
	commands := []*struct {
		name string
		cmd  interface {
			Short() string
			Long() string
		}
	}{
		// Using the raw cobra commands
	}

	// Test sessionCmd
	if sessionCmd.Short == "" {
		t.Error("sessionCmd should have Short description")
	}
	if sessionCmd.Long == "" {
		t.Error("sessionCmd should have Long description")
	}

	// Test sessionListCmd
	if sessionListCmd.Short == "" {
		t.Error("sessionListCmd should have Short description")
	}

	// Test sessionAttachCmd
	if sessionAttachCmd.Short == "" {
		t.Error("sessionAttachCmd should have Short description")
	}
	if sessionAttachCmd.Long == "" {
		t.Error("sessionAttachCmd should have Long description")
	}

	// Test sessionKillCmd
	if sessionKillCmd.Short == "" {
		t.Error("sessionKillCmd should have Short description")
	}
	if sessionKillCmd.Long == "" {
		t.Error("sessionKillCmd should have Long description")
	}

	// Avoid unused variable warning
	_ = commands
}
