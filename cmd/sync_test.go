package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"thoreinstein.com/sre/pkg/obsidian"
)

func TestUpdateNoteTitle(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		summary  string
		expected string
	}{
		{
			name:     "replace existing heading",
			content:  "# Old Title\n\nSome content here.",
			summary:  "New Title",
			expected: "# New Title\n\nSome content here.",
		},
		{
			name:     "heading with extra content after",
			content:  "# Old Title\n\n## Summary\n\nDetails here.\n\n## Notes\n\nMore notes.",
			summary:  "Updated Summary",
			expected: "# Updated Summary\n\n## Summary\n\nDetails here.\n\n## Notes\n\nMore notes.",
		},
		{
			name:     "no heading in content",
			content:  "Just some text without a heading.\n\nMore text.",
			summary:  "New Title",
			expected: "Just some text without a heading.\n\nMore text.",
		},
		{
			name:     "empty content",
			content:  "",
			summary:  "New Title",
			expected: "",
		},
		{
			name:     "heading only",
			content:  "# Old Title",
			summary:  "New Title",
			expected: "# New Title",
		},
		{
			name:     "multiple H1 headings - only first replaced",
			content:  "# First Title\n\nContent\n\n# Second Title\n\nMore content",
			summary:  "Updated First",
			expected: "# Updated First\n\nContent\n\n# Second Title\n\nMore content",
		},
		{
			name:     "H2 heading not affected",
			content:  "## Subheading\n\nContent here.",
			summary:  "New Title",
			expected: "## Subheading\n\nContent here.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := updateNoteTitle(tt.content, tt.summary)
			if result != tt.expected {
				t.Errorf("updateNoteTitle() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestBuildJiraDetailsSection(t *testing.T) {
	tests := []struct {
		name     string
		jiraInfo *obsidian.JiraInfo
		contains []string
		missing  []string
	}{
		{
			name: "all fields present",
			jiraInfo: &obsidian.JiraInfo{
				Type:        "Bug",
				Status:      "In Progress",
				Description: "This is a bug description.",
			},
			contains: []string{
				"**Type:** Bug",
				"**Status:** In Progress",
				"**Description:**",
				"This is a bug description.",
			},
			missing: []string{},
		},
		{
			name: "only type",
			jiraInfo: &obsidian.JiraInfo{
				Type: "Story",
			},
			contains: []string{"**Type:** Story"},
			missing:  []string{"**Status:**", "**Description:**"},
		},
		{
			name: "only status",
			jiraInfo: &obsidian.JiraInfo{
				Status: "Done",
			},
			contains: []string{"**Status:** Done"},
			missing:  []string{"**Type:**", "**Description:**"},
		},
		{
			name: "only description",
			jiraInfo: &obsidian.JiraInfo{
				Description: "Just a description",
			},
			contains: []string{"**Description:**", "Just a description"},
			missing:  []string{"**Type:**", "**Status:**"},
		},
		{
			name:     "empty info",
			jiraInfo: &obsidian.JiraInfo{},
			contains: []string{},
			missing:  []string{"**Type:**", "**Status:**", "**Description:**"},
		},
		{
			name: "multiline description",
			jiraInfo: &obsidian.JiraInfo{
				Type:        "Task",
				Description: "Line 1\nLine 2\nLine 3",
			},
			contains: []string{
				"**Type:** Task",
				"Line 1\nLine 2\nLine 3",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildJiraDetailsSection(tt.jiraInfo)

			for _, s := range tt.contains {
				if !strings.Contains(result, s) {
					t.Errorf("buildJiraDetailsSection() should contain %q, got %q", s, result)
				}
			}

			for _, s := range tt.missing {
				if strings.Contains(result, s) {
					t.Errorf("buildJiraDetailsSection() should not contain %q, got %q", s, result)
				}
			}
		})
	}
}

func TestUpdateJiraDetailsSection(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		jiraInfo *obsidian.JiraInfo
		contains []string
	}{
		{
			name: "update existing JIRA section",
			content: `# Ticket Title

## Summary

Some summary.

## JIRA Details

**Type:** Old Type
**Status:** Old Status

## Notes

Some notes.`,
			jiraInfo: &obsidian.JiraInfo{
				Type:   "New Type",
				Status: "New Status",
			},
			contains: []string{
				"## JIRA Details",
				"**Type:** New Type",
				"**Status:** New Status",
				"## Notes",
				"Some notes.",
			},
		},
		{
			name: "insert JIRA section after Summary",
			content: `# Ticket Title

## Summary

Some summary here.

## Notes

Some notes.`,
			jiraInfo: &obsidian.JiraInfo{
				Type:   "Bug",
				Status: "Open",
			},
			contains: []string{
				"## Summary",
				"## JIRA Details",
				"**Type:** Bug",
				"**Status:** Open",
				"## Notes",
			},
		},
		{
			name: "append JIRA section at end",
			content: `# Ticket Title

Just some content without Summary section.`,
			jiraInfo: &obsidian.JiraInfo{
				Type: "Task",
			},
			contains: []string{
				"# Ticket Title",
				"## JIRA Details",
				"**Type:** Task",
			},
		},
		{
			name:    "empty content",
			content: "",
			jiraInfo: &obsidian.JiraInfo{
				Type: "Story",
			},
			contains: []string{
				"## JIRA Details",
				"**Type:** Story",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := updateJiraDetailsSection(tt.content, tt.jiraInfo)

			for _, s := range tt.contains {
				if !strings.Contains(result, s) {
					t.Errorf("updateJiraDetailsSection() should contain %q\nGot:\n%s", s, result)
				}
			}
		})
	}
}

func TestUpdateJiraDetailsSection_PreservesOtherContent(t *testing.T) {
	content := `# My Ticket

## Summary

Important summary that should be preserved.

## Notes

These notes should stay intact.

## Log

- Entry 1
- Entry 2`

	jiraInfo := &obsidian.JiraInfo{
		Type:   "Bug",
		Status: "In Progress",
	}

	result := updateJiraDetailsSection(content, jiraInfo)

	// All original sections should be preserved
	preservedContent := []string{
		"# My Ticket",
		"## Summary",
		"Important summary that should be preserved.",
		"## Notes",
		"These notes should stay intact.",
		"## Log",
		"- Entry 1",
		"- Entry 2",
	}

	for _, s := range preservedContent {
		if !strings.Contains(result, s) {
			t.Errorf("Content should be preserved: %q\nGot:\n%s", s, result)
		}
	}
}

func TestUpdateNoteWithJiraInfo(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "sync-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a test note file
	notePath := filepath.Join(tmpDir, "test-note.md")
	initialContent := `# Old Title

## Summary

Some summary content.

## Notes

Some notes here.`

	if err := os.WriteFile(notePath, []byte(initialContent), 0644); err != nil {
		t.Fatalf("Failed to write test note: %v", err)
	}

	jiraInfo := &obsidian.JiraInfo{
		Summary:     "New JIRA Summary",
		Type:        "Bug",
		Status:      "In Progress",
		Description: "Bug description here.",
	}

	err = updateNoteWithJiraInfo(notePath, jiraInfo)
	if err != nil {
		t.Fatalf("updateNoteWithJiraInfo() error: %v", err)
	}

	// Read the updated content
	content, err := os.ReadFile(notePath)
	if err != nil {
		t.Fatalf("Failed to read updated note: %v", err)
	}

	contentStr := string(content)

	// Title should be updated
	if !strings.Contains(contentStr, "# New JIRA Summary") {
		t.Error("Title should be updated to JIRA summary")
	}

	// JIRA details should be added
	if !strings.Contains(contentStr, "## JIRA Details") {
		t.Error("JIRA Details section should be added")
	}

	if !strings.Contains(contentStr, "**Type:** Bug") {
		t.Error("Type should be in JIRA section")
	}

	if !strings.Contains(contentStr, "**Status:** In Progress") {
		t.Error("Status should be in JIRA section")
	}

	// Original content should be preserved
	if !strings.Contains(contentStr, "## Notes") {
		t.Error("Notes section should be preserved")
	}
}

func TestUpdateNoteWithJiraInfo_NoSummary(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "sync-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a test note file
	notePath := filepath.Join(tmpDir, "test-note.md")
	initialContent := `# Original Title

## Summary

Content here.`

	if err := os.WriteFile(notePath, []byte(initialContent), 0644); err != nil {
		t.Fatalf("Failed to write test note: %v", err)
	}

	// JIRA info without summary - title should not change
	jiraInfo := &obsidian.JiraInfo{
		Type:   "Task",
		Status: "Open",
	}

	err = updateNoteWithJiraInfo(notePath, jiraInfo)
	if err != nil {
		t.Fatalf("updateNoteWithJiraInfo() error: %v", err)
	}

	content, err := os.ReadFile(notePath)
	if err != nil {
		t.Fatalf("Failed to read updated note: %v", err)
	}

	// Title should remain unchanged
	if !strings.Contains(string(content), "# Original Title") {
		t.Error("Title should remain unchanged when no summary provided")
	}
}

func TestUpdateNoteWithJiraInfo_NonExistentFile(t *testing.T) {
	err := updateNoteWithJiraInfo("/nonexistent/path/note.md", &obsidian.JiraInfo{})
	if err == nil {
		t.Error("updateNoteWithJiraInfo() should error for non-existent file")
	}
}
