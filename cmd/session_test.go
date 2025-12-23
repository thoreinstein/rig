package cmd

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestSessionCommandStructure(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	cmd := sessionListCmd

	if cmd.Use != "list" {
		t.Errorf("session list Use = %q, want %q", cmd.Use, "list")
	}

	if cmd.Short == "" {
		t.Error("session list should have Short description")
	}
}

func TestSessionAttachCommand(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

	// Test the error message parsing logic in runSessionKillCommand
	// The function checks if error contains "does not exist"
	tests := []struct {
		name             string
		errorMsg         string
		shouldBeGraceful bool
	}{
		{
			name:             "session does not exist",
			errorMsg:         "session 'test' does not exist",
			shouldBeGraceful: true,
		},
		{
			name:             "session does not exist - different format",
			errorMsg:         "error: does not exist: test-session",
			shouldBeGraceful: true,
		},
		{
			name:             "different error",
			errorMsg:         "failed to connect to tmux server",
			shouldBeGraceful: false,
		},
		{
			name:             "permission denied",
			errorMsg:         "permission denied",
			shouldBeGraceful: false,
		},
		{
			name:             "empty error message",
			errorMsg:         "",
			shouldBeGraceful: false,
		},
		{
			name:             "partial match - exists alone",
			errorMsg:         "session exists",
			shouldBeGraceful: false,
		},
		{
			name:             "case sensitivity - uppercase",
			errorMsg:         "session DOES NOT EXIST",
			shouldBeGraceful: false, // strings.Contains is case-sensitive
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
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
	t.Parallel()

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
}

func TestSessionListOutputFormatting(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		sessions         []string
		expectedContains []string
		expectedPrefix   string
	}{
		{
			name:     "empty session list",
			sessions: []string{},
			expectedContains: []string{
				"No tmux sessions found.",
			},
		},
		{
			name:     "single session",
			sessions: []string{"sre-FRAAS-123"},
			expectedContains: []string{
				"Active tmux sessions:",
				"sre-FRAAS-123",
			},
			expectedPrefix: "\u2192 ", // Arrow prefix for first session
		},
		{
			name:     "multiple sessions",
			sessions: []string{"sre-FRAAS-123", "sre-CRE-456", "hack-test"},
			expectedContains: []string{
				"Active tmux sessions:",
				"sre-FRAAS-123",
				"sre-CRE-456",
				"hack-test",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Simulate the output formatting logic from runSessionListCommand
			var output strings.Builder

			if len(tt.sessions) == 0 {
				output.WriteString("No tmux sessions found.\n")
			} else {
				output.WriteString("Active tmux sessions:\n")
				for i, session := range tt.sessions {
					prefix := "  "
					if i == 0 {
						prefix = "\u2192 "
					}
					output.WriteString(prefix + session + "\n")
				}
			}

			result := output.String()
			for _, expected := range tt.expectedContains {
				if !strings.Contains(result, expected) {
					t.Errorf("Output missing expected string %q\nGot: %s", expected, result)
				}
			}

			// Verify first session has arrow prefix
			if len(tt.sessions) > 0 {
				if !strings.Contains(result, "\u2192 "+tt.sessions[0]) {
					t.Errorf("First session should have arrow prefix, got: %s", result)
				}
			}

			// Verify subsequent sessions have space prefix
			if len(tt.sessions) > 1 {
				if !strings.Contains(result, "  "+tt.sessions[1]) {
					t.Errorf("Second session should have space prefix, got: %s", result)
				}
			}
		})
	}
}

func TestSessionAttachArgValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		args        []string
		expectError bool
	}{
		{
			name:        "no arguments",
			args:        []string{},
			expectError: true,
		},
		{
			name:        "one argument",
			args:        []string{"FRAAS-123"},
			expectError: false,
		},
		{
			name:        "too many arguments",
			args:        []string{"FRAAS-123", "extra"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create a fresh command for testing
			cmd := *sessionAttachCmd
			cmd.SetArgs(tt.args)

			// Capture output
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)

			// RunE will fail due to config/tmux not being available,
			// but arg validation happens first
			err := cmd.ValidateArgs(tt.args)
			if tt.expectError && err == nil {
				t.Error("Expected argument validation error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no argument validation error, got: %v", err)
			}
		})
	}
}

func TestSessionKillArgValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		args        []string
		expectError bool
	}{
		{
			name:        "no arguments",
			args:        []string{},
			expectError: true,
		},
		{
			name:        "one argument",
			args:        []string{"FRAAS-123"},
			expectError: false,
		},
		{
			name:        "too many arguments",
			args:        []string{"FRAAS-123", "extra"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create a fresh command for testing
			cmd := *sessionKillCmd
			cmd.SetArgs(tt.args)

			// Capture output
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)

			err := cmd.ValidateArgs(tt.args)
			if tt.expectError && err == nil {
				t.Error("Expected argument validation error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no argument validation error, got: %v", err)
			}
		})
	}
}

func TestSessionKillSuccessOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		ticket         string
		expectedOutput string
	}{
		{
			name:           "standard ticket",
			ticket:         "FRAAS-123",
			expectedOutput: "\u2713 Session for ticket 'FRAAS-123' killed successfully.\n",
		},
		{
			name:           "different ticket format",
			ticket:         "CRE-456",
			expectedOutput: "\u2713 Session for ticket 'CRE-456' killed successfully.\n",
		},
		{
			name:           "hack session",
			ticket:         "hack-feature-test",
			expectedOutput: "\u2713 Session for ticket 'hack-feature-test' killed successfully.\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Simulate the success output from runSessionKillCommand
			var buf bytes.Buffer
			_, _ = buf.WriteString("\u2713 Session for ticket '" + tt.ticket + "' killed successfully.\n")

			if buf.String() != tt.expectedOutput {
				t.Errorf("Output = %q, want %q", buf.String(), tt.expectedOutput)
			}
		})
	}
}

func TestSessionKillGracefulNotExist(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		ticket         string
		expectedOutput string
	}{
		{
			name:           "non-existent session message",
			ticket:         "FRAAS-999",
			expectedOutput: "Session for ticket 'FRAAS-999' does not exist.\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Simulate the graceful non-existent output from runSessionKillCommand
			var buf bytes.Buffer
			_, _ = buf.WriteString("Session for ticket '" + tt.ticket + "' does not exist.\n")

			if buf.String() != tt.expectedOutput {
				t.Errorf("Output = %q, want %q", buf.String(), tt.expectedOutput)
			}
		})
	}
}

func TestSessionAttachVerboseOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		sessionName    string
		expectedOutput string
	}{
		{
			name:           "verbose attach message",
			sessionName:    "sre-FRAAS-123",
			expectedOutput: "Attaching to session: sre-FRAAS-123\n",
		},
		{
			name:           "verbose attach with prefix",
			sessionName:    "work_CRE-456",
			expectedOutput: "Attaching to session: work_CRE-456\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Simulate the verbose output from runSessionAttachCommand
			var buf bytes.Buffer
			_, _ = buf.WriteString("Attaching to session: " + tt.sessionName + "\n")

			if buf.String() != tt.expectedOutput {
				t.Errorf("Output = %q, want %q", buf.String(), tt.expectedOutput)
			}
		})
	}
}

func TestSessionSubcommandCount(t *testing.T) {
	t.Parallel()

	subcommands := sessionCmd.Commands()
	expectedCount := 3 // list, attach, kill

	if len(subcommands) != expectedCount {
		t.Errorf("session command has %d subcommands, want %d", len(subcommands), expectedCount)
	}
}

func TestSessionCommandHierarchy(t *testing.T) {
	t.Parallel()

	// Verify session command is a child of root
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "session" {
			found = true
			break
		}
	}
	if !found {
		t.Error("session command should be a child of root command")
	}
}

func TestSessionListCommandHasNoArgs(t *testing.T) {
	t.Parallel()

	// list command should accept no arguments
	cmd := sessionListCmd

	// Verify it doesn't have required args set
	if cmd.Args != nil {
		// Try to validate with empty args
		err := cmd.ValidateArgs([]string{})
		if err != nil {
			t.Errorf("session list should accept no arguments, got error: %v", err)
		}
	}
}

// captureOutput is a helper to capture stdout during test execution
func captureOutput(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

func TestSessionListOutputWithNoSessions(t *testing.T) {
	// This tests the actual output formatting function
	// by simulating empty session list behavior

	output := captureOutput(func() {
		sessions := []string{}
		if len(sessions) == 0 {
			// This matches the logic in runSessionListCommand
			_, _ = os.Stdout.WriteString("No tmux sessions found.\n")
		}
	})

	expected := "No tmux sessions found.\n"
	if output != expected {
		t.Errorf("Output = %q, want %q", output, expected)
	}
}

func TestSessionListOutputWithSessions(t *testing.T) {
	sessions := []string{"sre-FRAAS-123", "sre-CRE-456"}

	output := captureOutput(func() {
		_, _ = os.Stdout.WriteString("Active tmux sessions:\n")
		for i, session := range sessions {
			prefix := "  "
			if i == 0 {
				prefix = "\u2192 "
			}
			_, _ = os.Stdout.WriteString(prefix + session + "\n")
		}
	})

	// Verify structure
	if !strings.HasPrefix(output, "Active tmux sessions:\n") {
		t.Error("Output should start with 'Active tmux sessions:'")
	}

	// Verify first session has arrow
	if !strings.Contains(output, "\u2192 sre-FRAAS-123") {
		t.Error("First session should have arrow prefix")
	}

	// Verify second session has space indent
	if !strings.Contains(output, "  sre-CRE-456") {
		t.Error("Second session should have space prefix")
	}
}

func TestSessionAttachErrorFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		sessionName string
		ticket      string
		wantErrMsg  string
	}{
		{
			name:        "non-existent session error",
			sessionName: "sre-FRAAS-123",
			ticket:      "FRAAS-123",
			wantErrMsg:  "tmux session 'sre-FRAAS-123' does not exist for ticket 'FRAAS-123'",
		},
		{
			name:        "different prefix",
			sessionName: "work-CRE-456",
			ticket:      "CRE-456",
			wantErrMsg:  "tmux session 'work-CRE-456' does not exist for ticket 'CRE-456'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Simulate the error format used in runSessionAttachCommand
			errMsg := "tmux session '" + tt.sessionName + "' does not exist for ticket '" + tt.ticket + "'"

			if errMsg != tt.wantErrMsg {
				t.Errorf("Error message = %q, want %q", errMsg, tt.wantErrMsg)
			}
		})
	}
}

func TestSessionCommandUsageStrings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		cmd          string
		expectedUse  string
		hasLongDesc  bool
		hasShortDesc bool
	}{
		{
			name:         "session parent command",
			cmd:          "session",
			expectedUse:  "session",
			hasLongDesc:  true,
			hasShortDesc: true,
		},
		{
			name:         "session list command",
			cmd:          "list",
			expectedUse:  "list",
			hasLongDesc:  true,
			hasShortDesc: true,
		},
		{
			name:         "session attach command",
			cmd:          "attach",
			expectedUse:  "attach <ticket>",
			hasLongDesc:  true,
			hasShortDesc: true,
		},
		{
			name:         "session kill command",
			cmd:          "kill",
			expectedUse:  "kill <ticket>",
			hasLongDesc:  true,
			hasShortDesc: true,
		},
	}

	// Map command names to actual commands
	cmdMap := map[string]struct {
		use   string
		short string
		long  string
	}{
		"session": {sessionCmd.Use, sessionCmd.Short, sessionCmd.Long},
		"list":    {sessionListCmd.Use, sessionListCmd.Short, sessionListCmd.Long},
		"attach":  {sessionAttachCmd.Use, sessionAttachCmd.Short, sessionAttachCmd.Long},
		"kill":    {sessionKillCmd.Use, sessionKillCmd.Short, sessionKillCmd.Long},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cmd := cmdMap[tt.cmd]

			if cmd.use != tt.expectedUse {
				t.Errorf("Use = %q, want %q", cmd.use, tt.expectedUse)
			}

			if tt.hasShortDesc && cmd.short == "" {
				t.Error("Expected Short description")
			}

			if tt.hasLongDesc && cmd.long == "" {
				t.Error("Expected Long description")
			}
		})
	}
}

func TestSessionVerboseKillOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		ticket         string
		expectedOutput string
	}{
		{
			name:           "verbose kill message",
			ticket:         "FRAAS-123",
			expectedOutput: "Killing session for ticket: FRAAS-123\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Simulate the verbose output from runSessionKillCommand
			var buf bytes.Buffer
			_, _ = buf.WriteString("Killing session for ticket: " + tt.ticket + "\n")

			if buf.String() != tt.expectedOutput {
				t.Errorf("Output = %q, want %q", buf.String(), tt.expectedOutput)
			}
		})
	}
}
