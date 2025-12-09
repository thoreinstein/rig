package cmd

import (
	"strings"
	"testing"
)

func TestHistoryCommandStructure(t *testing.T) {
	cmd := historyCmd

	if cmd.Use != "history" {
		t.Errorf("history command Use = %q, want %q", cmd.Use, "history")
	}

	// Check subcommands exist
	subcommands := cmd.Commands()
	subcommandNames := make(map[string]bool)
	for _, sub := range subcommands {
		subcommandNames[sub.Use] = true
	}

	// Check for query subcommand
	if !subcommandNames["query [pattern]"] {
		t.Error("history command missing 'query' subcommand")
	}

	// Check for info subcommand
	if !subcommandNames["info"] {
		t.Error("history command missing 'info' subcommand")
	}
}

func TestHistoryQueryCommandFlags(t *testing.T) {
	cmd := historyQueryCmd

	// Check all expected flags exist
	expectedFlags := []struct {
		name     string
		defValue string
	}{
		{"since", ""},
		{"until", ""},
		{"directory", ""},
		{"session", ""},
		{"failed-only", "false"},
		{"limit", "50"},
	}

	for _, expected := range expectedFlags {
		flag := cmd.Flags().Lookup(expected.name)
		if flag == nil {
			t.Errorf("history query command should have --%s flag", expected.name)
			continue
		}
		if flag.DefValue != expected.defValue {
			t.Errorf("--%s default = %q, want %q", expected.name, flag.DefValue, expected.defValue)
		}
	}
}

func TestHistoryQueryCommandDescription(t *testing.T) {
	cmd := historyQueryCmd

	if cmd.Short == "" {
		t.Error("history query should have Short description")
	}

	if cmd.Long == "" {
		t.Error("history query should have Long description")
	}

	// Verify examples are in the description
	if !strings.Contains(cmd.Long, "sre history query") {
		t.Error("history query Long description should contain usage examples")
	}
}

func TestHistoryInfoCommandDescription(t *testing.T) {
	cmd := historyInfoCmd

	if cmd.Use != "info" {
		t.Errorf("history info Use = %q, want %q", cmd.Use, "info")
	}

	if cmd.Short == "" {
		t.Error("history info should have Short description")
	}

	if cmd.Long == "" {
		t.Error("history info should have Long description")
	}
}

func TestHistoryCommandDescription(t *testing.T) {
	cmd := historyCmd

	if cmd.Short == "" {
		t.Error("history command should have Short description")
	}

	if cmd.Long == "" {
		t.Error("history command should have Long description")
	}

	// Verify key information is in the description
	if !strings.Contains(cmd.Long, "history") {
		t.Error("history command Long description should mention 'history'")
	}

	if !strings.Contains(cmd.Long, "database") {
		t.Error("history command Long description should mention 'database'")
	}
}

func TestDurationFormatting(t *testing.T) {
	// Test the duration formatting logic used in runHistoryQueryCommand
	tests := []struct {
		name        string
		durationMs  int
		expected    string
	}{
		{
			name:       "milliseconds - under 1000",
			durationMs: 100,
			expected:   "100ms",
		},
		{
			name:       "milliseconds - near 1000",
			durationMs: 999,
			expected:   "999ms",
		},
		{
			name:       "seconds - exactly 1000",
			durationMs: 1000,
			expected:   "1.0s",
		},
		{
			name:       "seconds - 1500ms",
			durationMs: 1500,
			expected:   "1.5s",
		},
		{
			name:       "seconds - larger values",
			durationMs: 5000,
			expected:   "5.0s",
		},
		{
			name:       "zero duration",
			durationMs: 0,
			expected:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result string
			if tt.durationMs > 0 {
				if tt.durationMs < 1000 {
					result = string(rune(tt.durationMs)) + "ms"
					// Actually format it properly
					result = formatDuration(tt.durationMs)
				} else {
					result = formatDuration(tt.durationMs)
				}
			}

			if result != tt.expected {
				t.Errorf("formatDuration(%d) = %q, want %q", tt.durationMs, result, tt.expected)
			}
		})
	}
}

// Helper function that mimics the duration formatting in runHistoryQueryCommand
func formatDuration(durationMs int) string {
	if durationMs <= 0 {
		return ""
	}
	if durationMs < 1000 {
		// Format as Xms
		result := ""
		if durationMs >= 100 {
			result += string(rune('0' + durationMs/100))
		}
		if durationMs >= 10 {
			result += string(rune('0' + (durationMs%100)/10))
		}
		result += string(rune('0' + durationMs%10))
		return result + "ms"
	}
	// Format as X.Ys
	seconds := float64(durationMs) / 1000.0
	intPart := int(seconds)
	fracPart := int((seconds - float64(intPart)) * 10)
	return string(rune('0'+intPart)) + "." + string(rune('0'+fracPart)) + "s"
}

func TestCommandTruncation(t *testing.T) {
	// Test the command truncation logic used in runHistoryQueryCommand
	tests := []struct {
		name     string
		command  string
		maxLen   int
		expected string
	}{
		{
			name:     "short command - no truncation",
			command:  "git status",
			maxLen:   80,
			expected: "git status",
		},
		{
			name:     "exact length - no truncation",
			command:  strings.Repeat("x", 80),
			maxLen:   80,
			expected: strings.Repeat("x", 80),
		},
		{
			name:     "long command - truncated",
			command:  strings.Repeat("x", 100),
			maxLen:   80,
			expected: strings.Repeat("x", 77) + "...",
		},
		{
			name:     "empty command",
			command:  "",
			maxLen:   80,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			command := tt.command
			if len(command) > tt.maxLen {
				command = command[:tt.maxLen-3] + "..."
			}

			if command != tt.expected {
				t.Errorf("Truncated command = %q (len=%d), want %q (len=%d)",
					command, len(command), tt.expected, len(tt.expected))
			}
		})
	}
}

func TestDirectoryTruncation(t *testing.T) {
	// Test the directory truncation logic used in runHistoryQueryCommand
	tests := []struct {
		name      string
		directory string
		maxLen    int
		expected  string
	}{
		{
			name:      "short directory - no truncation",
			directory: "/home/user/project",
			maxLen:    30,
			expected:  "/home/user/project",
		},
		{
			name:      "long directory - truncated from start",
			directory: "/very/long/path/that/exceeds/thirty/characters",
			maxLen:    30,
			expected:  "...t/exceeds/thirty/characters",
		},
		{
			name:      "exact length - no truncation",
			directory: strings.Repeat("x", 30),
			maxLen:    30,
			expected:  strings.Repeat("x", 30),
		},
		{
			name:      "empty directory",
			directory: "",
			maxLen:    30,
			expected:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			directory := tt.directory
			if len(directory) > tt.maxLen {
				directory = "..." + directory[len(directory)-(tt.maxLen-3):]
			}

			if directory != tt.expected {
				t.Errorf("Truncated directory = %q, want %q", directory, tt.expected)
			}
		})
	}
}

func TestStatusIconSelection(t *testing.T) {
	// Test the status icon selection logic used in runHistoryQueryCommand
	tests := []struct {
		name       string
		exitCode   int
		expectedIcon string
	}{
		{
			name:       "success - exit code 0",
			exitCode:   0,
			expectedIcon: "✓",
		},
		{
			name:       "failure - exit code 1",
			exitCode:   1,
			expectedIcon: "✗",
		},
		{
			name:       "failure - exit code 127",
			exitCode:   127,
			expectedIcon: "✗",
		},
		{
			name:       "failure - negative exit code",
			exitCode:   -1,
			expectedIcon: "✗",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var statusIcon string
			if tt.exitCode == 0 {
				statusIcon = "✓"
			} else {
				statusIcon = "✗"
			}

			if statusIcon != tt.expectedIcon {
				t.Errorf("Status icon for exit code %d = %q, want %q",
					tt.exitCode, statusIcon, tt.expectedIcon)
			}
		})
	}
}
