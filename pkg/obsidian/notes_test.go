package obsidian

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetVaultSubdir(t *testing.T) {
	nm := NewNoteManager("/vault", "templates", "areas", "daily", false)

	if nm.VaultSubdir != "" {
		t.Errorf("VaultSubdir should be empty after construction, got %q", nm.VaultSubdir)
	}

	nm.SetVaultSubdir("CustomDir")
	if nm.VaultSubdir != "CustomDir" {
		t.Errorf("SetVaultSubdir() = %q, want %q", nm.VaultSubdir, "CustomDir")
	}
}

func TestCreateTicketNote_SubdirSelection(t *testing.T) {
	// Create a temporary vault directory
	tmpDir, err := os.MkdirTemp("", "obsidian-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name           string
		ticketType     string
		configuredDir  string
		expectedSubdir string
	}{
		{
			name:           "default jira ticket type",
			ticketType:     "fraas",
			configuredDir:  "",
			expectedSubdir: "Jira",
		},
		{
			name:           "incident ticket type",
			ticketType:     "incident",
			configuredDir:  "",
			expectedSubdir: "Incidents",
		},
		{
			name:           "hack ticket type",
			ticketType:     "hack",
			configuredDir:  "",
			expectedSubdir: "Hacks",
		},
		{
			name:           "configured subdir overrides default",
			ticketType:     "fraas",
			configuredDir:  "CustomJira",
			expectedSubdir: "CustomJira",
		},
		{
			name:           "configured subdir overrides hack default",
			ticketType:     "hack",
			configuredDir:  "MyHacks",
			expectedSubdir: "MyHacks",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nm := NewNoteManager(tmpDir, "templates", "Areas", "Daily", false)
			if tt.configuredDir != "" {
				nm.SetVaultSubdir(tt.configuredDir)
			}

			notePath, err := nm.CreateTicketNote(tt.ticketType, "TEST-123", nil)
			if err != nil {
				t.Fatalf("CreateTicketNote() error: %v", err)
			}

			// Verify the path contains the expected subdirectory
			expectedPathPart := filepath.Join("Areas", tt.expectedSubdir, tt.ticketType)
			if !strings.Contains(notePath, expectedPathPart) {
				t.Errorf("CreateTicketNote() path = %q, expected to contain %q", notePath, expectedPathPart)
			}

			// Verify the file was created
			if _, err := os.Stat(notePath); os.IsNotExist(err) {
				t.Errorf("CreateTicketNote() file not created at %q", notePath)
			}

			// Clean up the created file for next test
			os.Remove(notePath)
		})
	}
}

func TestCreateTicketNote_ExistingNote(t *testing.T) {
	// Create a temporary vault directory
	tmpDir, err := os.MkdirTemp("", "obsidian-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	nm := NewNoteManager(tmpDir, "templates", "Areas", "Daily", false)

	// Create the first note
	notePath1, err := nm.CreateTicketNote("jira", "TEST-456", nil)
	if err != nil {
		t.Fatalf("CreateTicketNote() first call error: %v", err)
	}

	// Create the same note again
	notePath2, err := nm.CreateTicketNote("jira", "TEST-456", nil)
	if err != nil {
		t.Fatalf("CreateTicketNote() second call error: %v", err)
	}

	// Paths should be the same
	if notePath1 != notePath2 {
		t.Errorf("CreateTicketNote() returned different paths: %q vs %q", notePath1, notePath2)
	}
}

func TestCreateTicketNote_MissingVault(t *testing.T) {
	nm := NewNoteManager("/nonexistent/vault/path", "templates", "Areas", "Daily", false)

	_, err := nm.CreateTicketNote("jira", "TEST-789", nil)
	if err == nil {
		t.Error("CreateTicketNote() expected error for missing vault, got nil")
	}
}

func TestCreateBasicNote(t *testing.T) {
	nm := NewNoteManager("/vault", "templates", "areas", "daily", false)

	content, err := nm.createBasicNote("TEST-123", "fraas")
	if err != nil {
		t.Fatalf("createBasicNote() error: %v", err)
	}

	// Check that content contains expected elements
	if !strings.Contains(content, "# TEST-123") {
		t.Error("createBasicNote() missing ticket title")
	}
	if !strings.Contains(content, "## Summary") {
		t.Error("createBasicNote() missing Summary section")
	}
	if !strings.Contains(content, "## Notes") {
		t.Error("createBasicNote() missing Notes section")
	}
	if !strings.Contains(content, "## Log") {
		t.Error("createBasicNote() missing Log section")
	}
}

func TestBuildJiraSection(t *testing.T) {
	nm := NewNoteManager("/vault", "templates", "areas", "daily", false)

	jiraInfo := &JiraInfo{
		Type:        "Bug",
		Status:      "In Progress",
		Description: "Fix the widget",
	}

	section := nm.buildJiraSection(jiraInfo)

	if !strings.Contains(section, "## JIRA Details") {
		t.Error("buildJiraSection() missing header")
	}
	if !strings.Contains(section, "**Type:** Bug") {
		t.Error("buildJiraSection() missing Type")
	}
	if !strings.Contains(section, "**Status:** In Progress") {
		t.Error("buildJiraSection() missing Status")
	}
	if !strings.Contains(section, "Fix the widget") {
		t.Error("buildJiraSection() missing Description")
	}
}

func TestInsertAfterSummary(t *testing.T) {
	nm := NewNoteManager("/vault", "templates", "areas", "daily", false)

	content := `# Ticket

## Summary

This is the summary.

## Notes

Some notes here.`

	insertion := "## JIRA Details\n\nNew content here."

	result := nm.insertAfterSummary(content, insertion)

	// Check that insertion appears after Summary and before Notes
	summaryIdx := strings.Index(result, "## Summary")
	jiraIdx := strings.Index(result, "## JIRA Details")
	notesIdx := strings.Index(result, "## Notes")

	if summaryIdx == -1 || jiraIdx == -1 || notesIdx == -1 {
		t.Fatalf("insertAfterSummary() missing sections: summary=%d, jira=%d, notes=%d",
			summaryIdx, jiraIdx, notesIdx)
	}

	if !(summaryIdx < jiraIdx && jiraIdx < notesIdx) {
		t.Errorf("insertAfterSummary() wrong order: summary=%d, jira=%d, notes=%d",
			summaryIdx, jiraIdx, notesIdx)
	}
}

func TestInsertLogEntry(t *testing.T) {
	nm := NewNoteManager("/vault", "templates", "areas", "daily", false)

	tests := []struct {
		name     string
		content  string
		logEntry string
	}{
		{
			name: "insert into existing log section",
			content: `# Daily

## Tasks

- Do something

## Log

- [10:00] Previous entry`,
			logEntry: "- [14:30] [[NEW-TICKET]]",
		},
		{
			name: "append when no log section",
			content: `# Daily

## Tasks

- Do something`,
			logEntry: "- [14:30] [[NEW-TICKET]]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := nm.insertLogEntry(tt.content, tt.logEntry)

			if !strings.Contains(result, tt.logEntry) {
				t.Errorf("insertLogEntry() result missing log entry: %q", tt.logEntry)
			}
			if !strings.Contains(result, "## Log") {
				t.Error("insertLogEntry() result missing Log section")
			}
		})
	}
}

func TestVaultExists(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "vault-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name      string
		vaultPath string
		expected  bool
	}{
		{
			name:      "existing vault",
			vaultPath: tmpDir,
			expected:  true,
		},
		{
			name:      "nonexistent vault",
			vaultPath: "/nonexistent/path/to/vault",
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nm := NewNoteManager(tt.vaultPath, "templates", "areas", "daily", false)
			result := nm.vaultExists()
			if result != tt.expected {
				t.Errorf("vaultExists() = %v, want %v", result, tt.expected)
			}
		})
	}
}
