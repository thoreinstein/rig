package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTimelineCommandStructure(t *testing.T) {
	// Not parallel - accesses global timelineCmd
	cmd := timelineCmd

	if cmd.Use != "timeline <ticket>" {
		t.Errorf("timeline command Use = %q, want %q", cmd.Use, "timeline <ticket>")
	}

	if cmd.Short == "" {
		t.Error("timeline command should have Short description")
	}

	if cmd.Long == "" {
		t.Error("timeline command should have Long description")
	}

	// Verify key information is in the description
	if !strings.Contains(cmd.Long, "timeline") {
		t.Error("timeline command Long description should mention 'timeline'")
	}

	if !strings.Contains(cmd.Long, "history") {
		t.Error("timeline command Long description should mention 'history'")
	}

	// Verify examples are in the description
	if !strings.Contains(cmd.Long, "rig timeline") {
		t.Error("timeline command Long description should contain usage examples")
	}
}

func TestTimelineCommandFlags(t *testing.T) {
	// Not parallel - accesses global timelineCmd
	cmd := timelineCmd

	expectedFlags := []struct {
		name     string
		defValue string
	}{
		{"since", ""},
		{"until", ""},
		{"directory", ""},
		{"failed-only", "false"},
		{"limit", "1000"},
		{"output", ""},
		{"no-update", "false"},
	}

	for _, expected := range expectedFlags {
		flag := cmd.Flags().Lookup(expected.name)
		if flag == nil {
			t.Errorf("timeline command should have --%s flag", expected.name)
			continue
		}
		if flag.DefValue != expected.defValue {
			t.Errorf("--%s default = %q, want %q", expected.name, flag.DefValue, expected.defValue)
		}
	}
}

func TestTimelineCommandArgs(t *testing.T) {
	// Not parallel - accesses global timelineCmd
	cmd := timelineCmd

	// Command should require exactly 1 argument
	if cmd.Args == nil {
		t.Error("timeline command should have Args validation")
	}
}

func TestParseTimeString(t *testing.T) {
	t.Parallel()

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
			t.Parallel()

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

func TestRemoveExistingTimeline(t *testing.T) {
	t.Parallel()

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
			t.Parallel()

			result := removeExistingTimeline(tt.content)
			if result != tt.expected {
				t.Errorf("removeExistingTimeline() = %q, want %q", result, tt.expected)
			}
		})
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

func TestValidateOutputPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		path    string
		wantErr bool
		errMsg  string
	}{
		// Valid cases
		{
			name:    "valid relative path",
			path:    "output.md",
			wantErr: false,
		},
		{
			name:    "valid nested relative path",
			path:    "reports/timeline.md",
			wantErr: false,
		},
		{
			name:    "valid .txt extension",
			path:    "timeline.txt",
			wantErr: false,
		},
		{
			name:    "valid .json extension",
			path:    "timeline.json",
			wantErr: false,
		},
		{
			name:    "no extension allowed",
			path:    "timeline",
			wantErr: false,
		},
		// Invalid cases
		{
			name:    "empty path",
			path:    "",
			wantErr: true,
			errMsg:  "cannot be empty",
		},
		{
			name:    "path traversal with ..",
			path:    "../../../etc/passwd",
			wantErr: true,
			errMsg:  "path traversal",
		},
		{
			name:    "path traversal hidden in middle",
			path:    "foo/../../../etc/passwd",
			wantErr: true,
			errMsg:  "path traversal",
		},
		{
			name:    "absolute path outside home",
			path:    "/etc/cron.d/malicious",
			wantErr: true,
			errMsg:  "must be within home directory",
		},
		{
			name:    "sensitive file .ssh",
			path:    ".ssh/authorized_keys",
			wantErr: true,
			errMsg:  "sensitive file",
		},
		{
			name:    "sensitive file .env",
			path:    "config/.env",
			wantErr: true,
			errMsg:  "sensitive file",
		},
		{
			name:    "sensitive file credentials",
			path:    "credentials.json",
			wantErr: true,
			errMsg:  "sensitive file",
		},
		{
			name:    "dangerous extension .sh",
			path:    "script.sh",
			wantErr: true,
			errMsg:  "safe extension",
		},
		{
			name:    "dangerous extension .exe",
			path:    "program.exe",
			wantErr: true,
			errMsg:  "safe extension",
		},
		{
			name:    "dangerous extension .py",
			path:    "script.py",
			wantErr: true,
			errMsg:  "safe extension",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateOutputPath(tt.path)
			if tt.wantErr {
				if err == nil {
					t.Errorf("validateOutputPath(%q) should have returned an error", tt.path)
					return
				}
				if !containsSubstring(err.Error(), tt.errMsg) {
					t.Errorf("validateOutputPath(%q) error = %q, should contain %q", tt.path, err.Error(), tt.errMsg)
				}
			} else if err != nil {
				t.Errorf("validateOutputPath(%q) returned unexpected error: %v", tt.path, err)
			}
		})
	}
}

func TestValidateOutputPath_AbsolutePathInHome(t *testing.T) {
	t.Parallel()

	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home directory")
	}

	// Path within home directory should be valid
	validPath := filepath.Join(homeDir, "Documents", "timeline.md")
	if err := validateOutputPath(validPath); err != nil {
		t.Errorf("validateOutputPath(%q) should be valid for path in home dir: %v", validPath, err)
	}
}

func TestValidateOutputPath_AbsolutePathInTemp(t *testing.T) {
	t.Parallel()

	// Using os.TempDir() intentionally - we're testing that validateOutputPath
	// accepts paths in the system temp directory, not creating test files
	tempDir := os.TempDir() //nolint:usetesting // need actual system temp path for validation test

	// Path within temp directory should be valid
	validPath := filepath.Join(tempDir, "timeline.md")
	if err := validateOutputPath(validPath); err != nil {
		t.Errorf("validateOutputPath(%q) should be valid for path in temp dir: %v", validPath, err)
	}
}

func TestWriteTimelineToFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "timeline.md")
	content := "## Command Timeline - TEST-123\n\nCommands: 5\n"

	err := writeTimelineToFile(content, testFile)
	if err != nil {
		t.Fatalf("writeTimelineToFile() unexpected error: %v", err)
	}

	// Verify file was created
	if _, statErr := os.Stat(testFile); os.IsNotExist(statErr) {
		t.Error("writeTimelineToFile() should create the file")
	}

	// Verify content
	readContent, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read written file: %v", err)
	}

	if string(readContent) != content {
		t.Errorf("writeTimelineToFile() content = %q, want %q", string(readContent), content)
	}

	// Verify permissions (0600)
	info, err := os.Stat(testFile)
	if err != nil {
		t.Fatalf("Failed to stat written file: %v", err)
	}

	// Check that file is not world-readable (owner read/write only)
	perm := info.Mode().Perm()
	if perm&0077 != 0 {
		t.Errorf("writeTimelineToFile() permissions = %o, want 0600 (no group/world access)", perm)
	}
}

func TestWriteTimelineToFile_NestedDirectory(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	nestedDir := filepath.Join(tmpDir, "reports", "timelines")
	testFile := filepath.Join(nestedDir, "timeline.md")
	content := "## Timeline\n"

	// Create nested directory first
	if err := os.MkdirAll(nestedDir, 0755); err != nil {
		t.Fatalf("Failed to create nested directory: %v", err)
	}

	err := writeTimelineToFile(content, testFile)
	if err != nil {
		t.Fatalf("writeTimelineToFile() unexpected error: %v", err)
	}

	// Verify file was created
	if _, statErr := os.Stat(testFile); os.IsNotExist(statErr) {
		t.Error("writeTimelineToFile() should create the file in nested directory")
	}
}

func TestWriteTimelineToFile_OverwritesExisting(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "timeline.md")

	// Write initial content
	initialContent := "Old content"
	if err := os.WriteFile(testFile, []byte(initialContent), 0644); err != nil {
		t.Fatalf("Failed to create initial file: %v", err)
	}

	// Overwrite with new content
	newContent := "New timeline content"
	err := writeTimelineToFile(newContent, testFile)
	if err != nil {
		t.Fatalf("writeTimelineToFile() unexpected error: %v", err)
	}

	// Verify new content
	readContent, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read written file: %v", err)
	}

	if string(readContent) != newContent {
		t.Errorf("writeTimelineToFile() should overwrite, got %q, want %q", string(readContent), newContent)
	}
}

func TestRemoveExistingTimeline_MultipleTimelineSections(t *testing.T) {
	t.Parallel()

	// Edge case: multiple timeline sections (shouldn't happen, but test behavior)
	content := `# Note

## Summary

Some summary.

## Command Timeline - TICKET-1

First timeline content

## Command Timeline - TICKET-2

Second timeline content

## Summary

Actual content`

	result := removeExistingTimeline(content)

	// Should remove first timeline section but keep Summary
	if !strings.Contains(result, "## Summary") {
		t.Error("removeExistingTimeline() should preserve Summary section")
	}
	if !strings.Contains(result, "Actual content") {
		t.Error("removeExistingTimeline() should preserve content after Summary")
	}
}

func TestRemoveExistingTimeline_TimelineWithSubsections(t *testing.T) {
	t.Parallel()

	content := `# Note

## Summary

Some summary.

## Command Timeline - TEST-123

Commands: 10

### 2025-08-10

- command 1
- command 2

### 2025-08-11

- command 3

## Notes

Final notes.`

	result := removeExistingTimeline(content)

	// Timeline and its subsections should be removed
	if strings.Contains(result, "Command Timeline") {
		t.Error("removeExistingTimeline() should remove timeline header")
	}
	if strings.Contains(result, "### 2025-08-10") {
		t.Error("removeExistingTimeline() should remove timeline day subsections")
	}
	if strings.Contains(result, "command 1") {
		t.Error("removeExistingTimeline() should remove timeline commands")
	}

	// Other sections should remain
	if !strings.Contains(result, "## Summary") {
		t.Error("removeExistingTimeline() should preserve Summary")
	}
	if !strings.Contains(result, "## Notes") {
		t.Error("removeExistingTimeline() should preserve Notes")
	}
	if !strings.Contains(result, "Final notes") {
		t.Error("removeExistingTimeline() should preserve Notes content")
	}
}
