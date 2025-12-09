package cmd

import (
	"testing"
	"time"

	"thoreinstein.com/sre/pkg/history"
)

func TestParseTimeString(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
		checkYear   int
		checkMonth  time.Month
		checkDay    int
	}{
		{
			name:       "date only",
			input:      "2025-08-10",
			checkYear:  2025,
			checkMonth: time.August,
			checkDay:   10,
		},
		{
			name:       "date and time",
			input:      "2025-08-10 09:30",
			checkYear:  2025,
			checkMonth: time.August,
			checkDay:   10,
		},
		{
			name:       "date and time with seconds",
			input:      "2025-08-10 09:30:45",
			checkYear:  2025,
			checkMonth: time.August,
			checkDay:   10,
		},
		{
			name:       "RFC3339 format",
			input:      "2025-08-10T09:30:00Z",
			checkYear:  2025,
			checkMonth: time.August,
			checkDay:   10,
		},
		{
			name:        "invalid format",
			input:       "August 10, 2025",
			expectError: true,
		},
		{
			name:        "empty string",
			input:       "",
			expectError: true,
		},
		{
			name:        "just time",
			input:       "09:30",
			expectError: true,
		},
		{
			name:        "garbage",
			input:       "not a date",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseTimeString(tt.input)

			if tt.expectError {
				if err == nil {
					t.Errorf("parseTimeString(%q) expected error, got nil", tt.input)
				}
				return
			}

			if err != nil {
				t.Fatalf("parseTimeString(%q) unexpected error: %v", tt.input, err)
			}

			if result.Year() != tt.checkYear {
				t.Errorf("Year = %d, want %d", result.Year(), tt.checkYear)
			}
			if result.Month() != tt.checkMonth {
				t.Errorf("Month = %v, want %v", result.Month(), tt.checkMonth)
			}
			if result.Day() != tt.checkDay {
				t.Errorf("Day = %d, want %d", result.Day(), tt.checkDay)
			}
		})
	}
}

func TestGenerateTimelineMarkdown(t *testing.T) {
	commands := []history.Command{
		{
			ID:        1,
			Command:   "git status",
			Timestamp: time.Date(2025, 8, 10, 9, 0, 0, 0, time.UTC),
			Duration:  100,
			ExitCode:  0,
			Directory: "/home/user/project",
		},
		{
			ID:        2,
			Command:   "git commit -m 'test'",
			Timestamp: time.Date(2025, 8, 10, 9, 5, 0, 0, time.UTC),
			Duration:  500,
			ExitCode:  0,
			Directory: "/home/user/project",
		},
		{
			ID:        3,
			Command:   "make build",
			Timestamp: time.Date(2025, 8, 10, 10, 0, 0, 0, time.UTC),
			Duration:  5000,
			ExitCode:  1,
			Directory: "/home/user/project",
		},
	}

	result := generateTimelineMarkdown(commands, "FRAAS-123")

	// Check header
	if !containsSubstring(result, "## Command Timeline - FRAAS-123") {
		t.Error("Timeline missing header")
	}

	// Check command count
	if !containsSubstring(result, "Commands: 3") {
		t.Error("Timeline missing command count")
	}

	// Check date section
	if !containsSubstring(result, "### 2025-08-10") {
		t.Error("Timeline missing date section")
	}

	// Check commands appear
	if !containsSubstring(result, "git status") {
		t.Error("Timeline missing first command")
	}
	if !containsSubstring(result, "git commit") {
		t.Error("Timeline missing second command")
	}
	if !containsSubstring(result, "make build") {
		t.Error("Timeline missing third command")
	}

	// Check failed command shows exit code
	if !containsSubstring(result, "[Exit: 1]") {
		t.Error("Timeline missing exit code for failed command")
	}
}

func TestGenerateTimelineMarkdown_EmptyCommands(t *testing.T) {
	result := generateTimelineMarkdown([]history.Command{}, "FRAAS-456")

	if !containsSubstring(result, "## Command Timeline - FRAAS-456") {
		t.Error("Timeline should have header even with empty commands")
	}
	if !containsSubstring(result, "Commands: 0") {
		t.Error("Timeline should show 0 commands")
	}
}

func TestGenerateTimelineMarkdown_MultipleDays(t *testing.T) {
	commands := []history.Command{
		{
			Command:   "command1",
			Timestamp: time.Date(2025, 8, 10, 9, 0, 0, 0, time.UTC),
		},
		{
			Command:   "command2",
			Timestamp: time.Date(2025, 8, 11, 10, 0, 0, 0, time.UTC),
		},
		{
			Command:   "command3",
			Timestamp: time.Date(2025, 8, 12, 11, 0, 0, 0, time.UTC),
		},
	}

	result := generateTimelineMarkdown(commands, "TEST-789")

	// Check all three days appear
	if !containsSubstring(result, "### 2025-08-10") {
		t.Error("Timeline missing first day")
	}
	if !containsSubstring(result, "### 2025-08-11") {
		t.Error("Timeline missing second day")
	}
	if !containsSubstring(result, "### 2025-08-12") {
		t.Error("Timeline missing third day")
	}
}

func TestRemoveExistingTimeline(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name: "content with timeline section",
			content: `# Ticket Note

## Summary

Some summary here.

## Command Timeline - FRAAS-123

Commands: 10
### 2025-08-10
- command1
- command2

## Notes

Some notes here.`,
			expected: `# Ticket Note

## Summary

Some summary here.

## Notes

Some notes here.`,
		},
		{
			name: "content without timeline",
			content: `# Ticket Note

## Summary

Some summary here.

## Notes

Some notes here.`,
			expected: `# Ticket Note

## Summary

Some summary here.

## Notes

Some notes here.`,
		},
		{
			name: "timeline at end of file",
			content: `# Ticket Note

## Summary

Summary.

## Command Timeline - TEST-456

Commands: 5`,
			expected: `# Ticket Note

## Summary

Summary.
`,
		},
		{
			name:     "empty content",
			content:  "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := removeExistingTimeline(tt.content)
			if result != tt.expected {
				t.Errorf("removeExistingTimeline() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestGenerateTimelineMarkdown_LongDirectory(t *testing.T) {
	commands := []history.Command{
		{
			Command:   "ls",
			Timestamp: time.Now(),
			Directory: "/very/long/path/that/exceeds/fifty/characters/in/length/to/test/truncation",
		},
	}

	result := generateTimelineMarkdown(commands, "TEST")

	// The directory should be truncated (last 30 chars shown with ...)
	if !containsSubstring(result, "...") {
		t.Error("Long directory should be truncated with ...")
	}
}

// Helper function to check if string contains substring
func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
