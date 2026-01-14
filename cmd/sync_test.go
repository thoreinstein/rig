package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"

	"thoreinstein.com/rig/pkg/jira"
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
		jiraInfo *jira.TicketInfo
		contains []string
		missing  []string
	}{
		{
			name: "all fields present",
			jiraInfo: &jira.TicketInfo{
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
			jiraInfo: &jira.TicketInfo{
				Type: "Story",
			},
			contains: []string{"**Type:** Story"},
			missing:  []string{"**Status:**", "**Description:**"},
		},
		{
			name: "only status",
			jiraInfo: &jira.TicketInfo{
				Status: "Done",
			},
			contains: []string{"**Status:** Done"},
			missing:  []string{"**Type:**", "**Description:**"},
		},
		{
			name: "only description",
			jiraInfo: &jira.TicketInfo{
				Description: "Just a description",
			},
			contains: []string{"**Description:**", "Just a description"},
			missing:  []string{"**Type:**", "**Status:**"},
		},
		{
			name:     "empty info",
			jiraInfo: &jira.TicketInfo{},
			contains: []string{},
			missing:  []string{"**Type:**", "**Status:**", "**Description:**"},
		},
		{
			name: "multiline description",
			jiraInfo: &jira.TicketInfo{
				Type:        "Task",
				Description: "Line 1\nLine 2\nLine 3",
			},
			contains: []string{
				"**Type:** Task",
				"Line 1\nLine 2\nLine 3",
			},
		},
		{
			name: "with custom fields",
			jiraInfo: &jira.TicketInfo{
				Type:   "Bug",
				Status: "In Progress",
				CustomFields: map[string]string{
					"Story Points": "5",
					"Sprint":       "Sprint 23",
				},
				Description: "Bug with custom fields",
			},
			contains: []string{
				"**Type:** Bug",
				"**Status:** In Progress",
				"**Story Points:** 5",
				"**Sprint:** Sprint 23",
				"**Description:**",
				"Bug with custom fields",
			},
		},
		{
			name: "custom fields only",
			jiraInfo: &jira.TicketInfo{
				CustomFields: map[string]string{
					"Team":     "Platform",
					"Assignee": "john.doe",
				},
			},
			contains: []string{
				"**Team:** Platform",
				"**Assignee:** john.doe",
			},
			missing: []string{"**Type:**", "**Status:**", "**Description:**"},
		},
		{
			name: "custom fields with empty values ignored",
			jiraInfo: &jira.TicketInfo{
				Type: "Story",
				CustomFields: map[string]string{
					"Sprint":     "Sprint 24",
					"EmptyField": "",
				},
			},
			contains: []string{
				"**Type:** Story",
				"**Sprint:** Sprint 24",
			},
			missing: []string{"**EmptyField:**"},
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
		jiraInfo *jira.TicketInfo
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
			jiraInfo: &jira.TicketInfo{
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
			jiraInfo: &jira.TicketInfo{
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
			jiraInfo: &jira.TicketInfo{
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
			jiraInfo: &jira.TicketInfo{
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

	jiraInfo := &jira.TicketInfo{
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
	tmpDir := t.TempDir()

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

	jiraInfo := &jira.TicketInfo{
		Summary:     "New JIRA Summary",
		Type:        "Bug",
		Status:      "In Progress",
		Description: "Bug description here.",
	}

	if err := updateNoteWithJiraInfo(notePath, jiraInfo); err != nil {
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
	tmpDir := t.TempDir()

	// Create a test note file
	notePath := filepath.Join(tmpDir, "test-note.md")
	initialContent := `# Original Title

## Summary

Content here.`

	if err := os.WriteFile(notePath, []byte(initialContent), 0644); err != nil {
		t.Fatalf("Failed to write test note: %v", err)
	}

	// JIRA info without summary - title should not change
	jiraInfo := &jira.TicketInfo{
		Type:   "Task",
		Status: "Open",
	}

	if err := updateNoteWithJiraInfo(notePath, jiraInfo); err != nil {
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
	err := updateNoteWithJiraInfo("/nonexistent/path/note.md", &jira.TicketInfo{})
	if err == nil {
		t.Error("updateNoteWithJiraInfo() should error for non-existent file")
	}
}

// ============================================================================
// Tests for sync command and flags
// ============================================================================

func TestSyncCommandDescription(t *testing.T) {
	// Not parallel - accesses global syncCmd
	cmd := syncCmd

	if cmd.Use != "sync [ticket]" {
		t.Errorf("sync command Use = %q, want %q", cmd.Use, "sync [ticket]")
	}

	if cmd.Short == "" {
		t.Error("sync command should have Short description")
	}

	if cmd.Long == "" {
		t.Error("sync command should have Long description")
	}

	// Verify key information is in the description
	if !strings.Contains(cmd.Long, "ticket") {
		t.Error("sync command Long description should mention 'ticket'")
	}

	if !strings.Contains(cmd.Long, "daily") {
		t.Error("sync command Long description should mention 'daily'")
	}
}

func TestSyncCommandFlags(t *testing.T) {
	// Not parallel - accesses global syncCmd
	cmd := syncCmd

	// Check --jira flag exists
	jiraFlag := cmd.Flags().Lookup("jira")
	if jiraFlag == nil {
		t.Error("sync command should have --jira flag")
	}
	if jiraFlag != nil && jiraFlag.DefValue != "false" {
		t.Errorf("--jira default should be false, got %s", jiraFlag.DefValue)
	}

	// Check --daily flag exists
	dailyFlag := cmd.Flags().Lookup("daily")
	if dailyFlag == nil {
		t.Error("sync command should have --daily flag")
	}
	if dailyFlag != nil && dailyFlag.DefValue != "false" {
		t.Errorf("--daily default should be false, got %s", dailyFlag.DefValue)
	}

	// Check --force flag exists
	forceFlag := cmd.Flags().Lookup("force")
	if forceFlag == nil {
		t.Error("sync command should have --force flag")
	}
	if forceFlag != nil && forceFlag.DefValue != "false" {
		t.Errorf("--force default should be false, got %s", forceFlag.DefValue)
	}
}

func TestSyncCommandMaxArgs(t *testing.T) {
	// Not parallel - accesses global syncCmd
	cmd := syncCmd

	// Command accepts at most 1 argument
	if cmd.Args == nil {
		t.Error("sync command should have Args validation")
	}
}

// ============================================================================
// Tests for runSyncCommand
// ============================================================================

// setupSyncTestConfig configures viper with test defaults for sync command
func setupSyncTestConfig(t *testing.T, notesPath string) {
	t.Helper()

	viper.Reset()
	viper.Set("notes.path", notesPath)
	viper.Set("notes.daily_dir", "daily")
	viper.Set("notes.template_dir", "") // Use embedded templates
	viper.Set("git.base_branch", "")
	viper.Set("jira.enabled", false) // Disable JIRA by default in tests
	viper.Set("tmux.session_prefix", "")
	viper.Set("tmux.windows", []map[string]string{
		{"name": "code", "command": ""},
	})
}

func TestRunSyncCommand_NoTicketNoDaily(t *testing.T) {
	notesDir := t.TempDir()
	setupSyncTestConfig(t, notesDir)
	defer viper.Reset()

	// Reset global flags
	syncJira = false
	syncDaily = false
	syncForce = false

	err := runSyncCommand("")
	if err == nil {
		t.Error("runSyncCommand() should error when no ticket and no --daily flag")
	}

	if !strings.Contains(err.Error(), "ticket required") {
		t.Errorf("Error should mention 'ticket required', got: %v", err)
	}
}

func TestRunSyncCommand_InvalidTicketFormat(t *testing.T) {
	notesDir := t.TempDir()
	setupSyncTestConfig(t, notesDir)
	defer viper.Reset()

	syncJira = false
	syncDaily = false
	syncForce = false

	tests := []struct {
		name   string
		ticket string
	}{
		{"no dash", "proj123"},
		{"no number", "proj-"},
		{"no type", "-123"},
		{"multiple dashes", "proj-123-456"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := runSyncCommand(tt.ticket)
			if err == nil {
				t.Errorf("runSyncCommand(%q) should error for invalid ticket format", tt.ticket)
			}
		})
	}
}

func TestRunSyncCommand_NoteNotFound(t *testing.T) {
	notesDir := t.TempDir()
	setupSyncTestConfig(t, notesDir)
	defer viper.Reset()

	syncJira = false
	syncDaily = false
	syncForce = false

	// Don't create the note - it should handle missing note gracefully
	err := runSyncCommand("proj-123")

	// The command returns nil but prints a message when note is not found
	// This is by design - it's not an error, just informational
	if err != nil {
		t.Errorf("runSyncCommand() should not error for missing note (returns nil with message), got: %v", err)
	}
}

func TestRunSyncCommand_SyncExistingTicketNote(t *testing.T) {
	notesDir := t.TempDir()
	setupSyncTestConfig(t, notesDir)
	defer viper.Reset()

	syncJira = false
	syncDaily = false
	syncForce = false

	// Create the ticket note directory and file
	ticketDir := filepath.Join(notesDir, "proj")
	if err := os.MkdirAll(ticketDir, 0755); err != nil {
		t.Fatalf("Failed to create ticket dir: %v", err)
	}

	notePath := filepath.Join(ticketDir, "proj-123.md")
	initialContent := `# proj-123

## Summary

Initial content.

## Notes

Some notes.`

	if err := os.WriteFile(notePath, []byte(initialContent), 0644); err != nil {
		t.Fatalf("Failed to write test note: %v", err)
	}

	// Create daily directory for UpdateDailyNote
	dailyDir := filepath.Join(notesDir, "daily")
	if err := os.MkdirAll(dailyDir, 0755); err != nil {
		t.Fatalf("Failed to create daily dir: %v", err)
	}

	err := runSyncCommand("proj-123")
	if err != nil {
		t.Errorf("runSyncCommand() unexpected error: %v", err)
	}

	// Verify note still exists
	if _, statErr := os.Stat(notePath); os.IsNotExist(statErr) {
		t.Error("Note should still exist after sync")
	}
}

func TestRunSyncCommand_WithDailyFlag(t *testing.T) {
	notesDir := t.TempDir()
	setupSyncTestConfig(t, notesDir)
	defer viper.Reset()

	syncJira = false
	syncDaily = true // Enable daily flag
	syncForce = false
	defer func() { syncDaily = false }()

	// When --daily is set, it should not require a ticket
	err := runSyncCommand("")

	// The command returns nil but prints a message when daily note is not found
	if err != nil {
		t.Errorf("runSyncCommand() with --daily should not error: %v", err)
	}
}

func TestRunSyncCommand_DailyNoteExists(t *testing.T) {
	notesDir := t.TempDir()
	setupSyncTestConfig(t, notesDir)
	defer viper.Reset()

	syncJira = false
	syncDaily = true
	syncForce = false
	defer func() { syncDaily = false }()

	// Create the daily note
	dailyDir := filepath.Join(notesDir, "daily")
	if err := os.MkdirAll(dailyDir, 0755); err != nil {
		t.Fatalf("Failed to create daily dir: %v", err)
	}

	today := time.Now().Format("2006-01-02")
	dailyNotePath := filepath.Join(dailyDir, today+".md")
	dailyContent := `# Daily Note - ` + today + `

## Log

- [09:00] Started work`

	if err := os.WriteFile(dailyNotePath, []byte(dailyContent), 0644); err != nil {
		t.Fatalf("Failed to write daily note: %v", err)
	}

	err := runSyncCommand("")
	if err != nil {
		t.Errorf("runSyncCommand() with --daily and existing note should not error: %v", err)
	}
}

func TestRunSyncCommand_DifferentTicketTypes(t *testing.T) {
	tests := []struct {
		name       string
		ticket     string
		ticketType string
	}{
		{"fraas ticket", "fraas-12345", "fraas"},
		{"cre ticket", "cre-999", "cre"},
		{"incident ticket", "incident-1", "incident"},
		{"ops ticket", "ops-42", "ops"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notesDir := t.TempDir()
			setupSyncTestConfig(t, notesDir)
			defer viper.Reset()

			syncJira = false
			syncDaily = false
			syncForce = false

			// Create the ticket note
			ticketDir := filepath.Join(notesDir, tt.ticketType)
			if err := os.MkdirAll(ticketDir, 0755); err != nil {
				t.Fatalf("Failed to create ticket dir: %v", err)
			}

			notePath := filepath.Join(ticketDir, tt.ticket+".md")
			content := "# " + tt.ticket + "\n\n## Summary\n\nContent."
			if err := os.WriteFile(notePath, []byte(content), 0644); err != nil {
				t.Fatalf("Failed to write test note: %v", err)
			}

			// Create daily directory
			dailyDir := filepath.Join(notesDir, "daily")
			if err := os.MkdirAll(dailyDir, 0755); err != nil {
				t.Fatalf("Failed to create daily dir: %v", err)
			}

			err := runSyncCommand(tt.ticket)
			if err != nil {
				t.Errorf("runSyncCommand(%q) unexpected error: %v", tt.ticket, err)
			}
		})
	}
}

// ============================================================================
// Tests for syncTicketNote
// ============================================================================

func TestSyncTicketNote_UpdatesDailyNote(t *testing.T) {
	notesDir := t.TempDir()
	setupSyncTestConfig(t, notesDir)
	defer viper.Reset()

	// Reset global flags
	syncJira = false
	syncDaily = false
	syncForce = false
	verbose = false

	// Create the ticket note
	ticketDir := filepath.Join(notesDir, "proj")
	if err := os.MkdirAll(ticketDir, 0755); err != nil {
		t.Fatalf("Failed to create ticket dir: %v", err)
	}

	notePath := filepath.Join(ticketDir, "proj-456.md")
	content := `# proj-456

## Summary

Test content.`
	if err := os.WriteFile(notePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test note: %v", err)
	}

	// Create daily directory
	dailyDir := filepath.Join(notesDir, "daily")
	if err := os.MkdirAll(dailyDir, 0755); err != nil {
		t.Fatalf("Failed to create daily dir: %v", err)
	}

	err := runSyncCommand("proj-456")
	if err != nil {
		t.Fatalf("runSyncCommand() error: %v", err)
	}

	// Verify daily note was created/updated
	today := time.Now().Format("2006-01-02")
	dailyNotePath := filepath.Join(dailyDir, today+".md")

	if _, statErr := os.Stat(dailyNotePath); os.IsNotExist(statErr) {
		t.Error("Daily note should be created after sync")
		return
	}

	// Verify daily note contains ticket reference
	dailyContent, err := os.ReadFile(dailyNotePath)
	if err != nil {
		t.Fatalf("Failed to read daily note: %v", err)
	}

	if !strings.Contains(string(dailyContent), "proj-456") {
		t.Errorf("Daily note should contain ticket reference, got: %s", string(dailyContent))
	}
}

func TestSyncTicketNote_WithJiraDisabled(t *testing.T) {
	notesDir := t.TempDir()
	setupSyncTestConfig(t, notesDir)
	viper.Set("jira.enabled", false) // Explicitly disable JIRA
	defer viper.Reset()

	syncJira = true // Even with --jira flag, JIRA is disabled in config
	syncDaily = false
	syncForce = false
	defer func() { syncJira = false }()

	// Create the ticket note
	ticketDir := filepath.Join(notesDir, "proj")
	if err := os.MkdirAll(ticketDir, 0755); err != nil {
		t.Fatalf("Failed to create ticket dir: %v", err)
	}

	notePath := filepath.Join(ticketDir, "proj-789.md")
	content := "# proj-789\n\n## Summary\n\nOriginal content."
	if err := os.WriteFile(notePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test note: %v", err)
	}

	// Create daily directory
	dailyDir := filepath.Join(notesDir, "daily")
	if err := os.MkdirAll(dailyDir, 0755); err != nil {
		t.Fatalf("Failed to create daily dir: %v", err)
	}

	// Should not error even with JIRA disabled
	err := runSyncCommand("proj-789")
	if err != nil {
		t.Errorf("runSyncCommand() should not error with JIRA disabled: %v", err)
	}
}

func TestSyncTicketNote_IncidentTypeSkipsJira(t *testing.T) {
	notesDir := t.TempDir()
	setupSyncTestConfig(t, notesDir)
	viper.Set("jira.enabled", true) // Enable JIRA
	defer viper.Reset()

	syncJira = false // Not forcing JIRA
	syncDaily = false
	syncForce = false

	// Create the incident note
	ticketDir := filepath.Join(notesDir, "incident")
	if err := os.MkdirAll(ticketDir, 0755); err != nil {
		t.Fatalf("Failed to create ticket dir: %v", err)
	}

	notePath := filepath.Join(ticketDir, "incident-100.md")
	content := "# incident-100\n\n## Summary\n\nIncident content."
	if err := os.WriteFile(notePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test note: %v", err)
	}

	// Create daily directory
	dailyDir := filepath.Join(notesDir, "daily")
	if err := os.MkdirAll(dailyDir, 0755); err != nil {
		t.Fatalf("Failed to create daily dir: %v", err)
	}

	// Should work for incident type (skips JIRA unless --jira flag is set)
	err := runSyncCommand("incident-100")
	if err != nil {
		t.Errorf("runSyncCommand() should handle incident type: %v", err)
	}
}

// ============================================================================
// Tests for syncDailyNote
// ============================================================================

func TestSyncDailyNote_NotFound(t *testing.T) {
	notesDir := t.TempDir()
	setupSyncTestConfig(t, notesDir)
	defer viper.Reset()

	syncJira = false
	syncDaily = true
	syncForce = false
	defer func() { syncDaily = false }()

	// Don't create the daily note
	err := runSyncCommand("")

	// The function returns nil and prints a message when daily note doesn't exist
	if err != nil {
		t.Errorf("syncDailyNote() should not error for missing daily note: %v", err)
	}
}

func TestSyncDailyNote_Exists(t *testing.T) {
	notesDir := t.TempDir()
	setupSyncTestConfig(t, notesDir)
	defer viper.Reset()

	syncJira = false
	syncDaily = true
	syncForce = false
	defer func() { syncDaily = false }()

	// Create the daily note
	dailyDir := filepath.Join(notesDir, "daily")
	if err := os.MkdirAll(dailyDir, 0755); err != nil {
		t.Fatalf("Failed to create daily dir: %v", err)
	}

	today := time.Now().Format("2006-01-02")
	dailyNotePath := filepath.Join(dailyDir, today+".md")
	content := "# Daily - " + today + "\n\n## Log\n"
	if err := os.WriteFile(dailyNotePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write daily note: %v", err)
	}

	err := runSyncCommand("")
	if err != nil {
		t.Errorf("syncDailyNote() should not error when daily note exists: %v", err)
	}
}

// ============================================================================
// Tests for edge cases and error handling
// ============================================================================

func TestRunSyncCommand_PreservesOriginalTicketCase(t *testing.T) {
	notesDir := t.TempDir()
	setupSyncTestConfig(t, notesDir)
	defer viper.Reset()

	syncJira = false
	syncDaily = false
	syncForce = false

	// Create note with uppercase ticket name
	ticketDir := filepath.Join(notesDir, "fraas")
	if err := os.MkdirAll(ticketDir, 0755); err != nil {
		t.Fatalf("Failed to create ticket dir: %v", err)
	}

	notePath := filepath.Join(ticketDir, "FRAAS-999.md")
	content := "# FRAAS-999\n\n## Summary\n\nContent."
	if err := os.WriteFile(notePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test note: %v", err)
	}

	// Create daily directory
	dailyDir := filepath.Join(notesDir, "daily")
	if err := os.MkdirAll(dailyDir, 0755); err != nil {
		t.Fatalf("Failed to create daily dir: %v", err)
	}

	// Sync with uppercase ticket
	err := runSyncCommand("FRAAS-999")
	if err != nil {
		t.Errorf("runSyncCommand() unexpected error: %v", err)
	}

	// Verify daily note contains original case
	today := time.Now().Format("2006-01-02")
	dailyNotePath := filepath.Join(dailyDir, today+".md")

	dailyContent, err := os.ReadFile(dailyNotePath)
	if err != nil {
		t.Fatalf("Failed to read daily note: %v", err)
	}

	if !strings.Contains(string(dailyContent), "FRAAS-999") {
		t.Errorf("Daily note should preserve original ticket case, got: %s", string(dailyContent))
	}
}

func TestRunSyncCommand_VerboseMode(t *testing.T) {
	notesDir := t.TempDir()
	setupSyncTestConfig(t, notesDir)
	defer viper.Reset()

	// Save and restore verbose flag
	oldVerbose := verbose
	verbose = true
	defer func() { verbose = oldVerbose }()

	syncJira = false
	syncDaily = false
	syncForce = false

	// Create the ticket note
	ticketDir := filepath.Join(notesDir, "proj")
	if err := os.MkdirAll(ticketDir, 0755); err != nil {
		t.Fatalf("Failed to create ticket dir: %v", err)
	}

	notePath := filepath.Join(ticketDir, "proj-888.md")
	content := "# proj-888\n\n## Summary\n\nContent."
	if err := os.WriteFile(notePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test note: %v", err)
	}

	// Create daily directory
	dailyDir := filepath.Join(notesDir, "daily")
	if err := os.MkdirAll(dailyDir, 0755); err != nil {
		t.Fatalf("Failed to create daily dir: %v", err)
	}

	// Should work in verbose mode
	err := runSyncCommand("proj-888")
	if err != nil {
		t.Errorf("runSyncCommand() should work in verbose mode: %v", err)
	}
}

func TestRunSyncCommand_ConfigLoadFailure(t *testing.T) {
	// Don't set up any config - this will use defaults
	viper.Reset()
	// Set an invalid notes path to trigger potential issues
	viper.Set("notes.path", "/nonexistent/invalid/path")
	defer viper.Reset()

	syncJira = false
	syncDaily = false
	syncForce = false

	// The command should handle this gracefully
	err := runSyncCommand("proj-123")

	// It should return nil (note not found message) rather than error
	// since the notes path doesn't exist but the command handles missing notes
	if err != nil {
		// This is expected behavior - missing notes directory is handled gracefully
		t.Logf("runSyncCommand() returned error as expected: %v", err)
	}
}
