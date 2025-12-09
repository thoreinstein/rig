package obsidian

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func TestUpdateDailyNote_ExistingNote(t *testing.T) {
	// Create a temporary vault directory
	tmpDir, err := os.MkdirTemp("", "obsidian-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create Daily directory
	dailyDir := filepath.Join(tmpDir, "Daily")
	if err := os.MkdirAll(dailyDir, 0755); err != nil {
		t.Fatalf("Failed to create daily dir: %v", err)
	}

	// Create a daily note for today
	today := getTodayDate()
	dailyNotePath := filepath.Join(dailyDir, today+".md")
	initialContent := `# Daily Note

## Tasks

- Task 1
- Task 2

## Log

- [09:00] Started work
`
	if err := os.WriteFile(dailyNotePath, []byte(initialContent), 0644); err != nil {
		t.Fatalf("Failed to write daily note: %v", err)
	}

	nm := NewNoteManager(tmpDir, "templates", "Areas", "Daily", false)

	err = nm.UpdateDailyNote("FRAAS-123")
	if err != nil {
		t.Fatalf("UpdateDailyNote() error: %v", err)
	}

	// Read the updated content
	content, err := os.ReadFile(dailyNotePath)
	if err != nil {
		t.Fatalf("Failed to read updated note: %v", err)
	}

	// Verify the log entry was added
	if !strings.Contains(string(content), "[[FRAAS-123]]") {
		t.Error("UpdateDailyNote() did not add ticket link to daily note")
	}

	// Verify the original content is still there
	if !strings.Contains(string(content), "Started work") {
		t.Error("UpdateDailyNote() removed original content")
	}
}

func TestUpdateDailyNote_NonExistentNote(t *testing.T) {
	// Create a temporary vault directory
	tmpDir, err := os.MkdirTemp("", "obsidian-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create Daily directory but no daily note
	dailyDir := filepath.Join(tmpDir, "Daily")
	if err := os.MkdirAll(dailyDir, 0755); err != nil {
		t.Fatalf("Failed to create daily dir: %v", err)
	}

	nm := NewNoteManager(tmpDir, "templates", "Areas", "Daily", false)

	// Should not error when daily note doesn't exist
	err = nm.UpdateDailyNote("FRAAS-456")
	if err != nil {
		t.Errorf("UpdateDailyNote() should not error for non-existent daily note: %v", err)
	}
}

func TestUpdateDailyNote_NoLogSection(t *testing.T) {
	// Create a temporary vault directory
	tmpDir, err := os.MkdirTemp("", "obsidian-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create Daily directory
	dailyDir := filepath.Join(tmpDir, "Daily")
	if err := os.MkdirAll(dailyDir, 0755); err != nil {
		t.Fatalf("Failed to create daily dir: %v", err)
	}

	// Create a daily note without a Log section
	today := getTodayDate()
	dailyNotePath := filepath.Join(dailyDir, today+".md")
	initialContent := `# Daily Note

## Tasks

- Task 1
- Task 2
`
	if err := os.WriteFile(dailyNotePath, []byte(initialContent), 0644); err != nil {
		t.Fatalf("Failed to write daily note: %v", err)
	}

	nm := NewNoteManager(tmpDir, "templates", "Areas", "Daily", false)

	err = nm.UpdateDailyNote("FRAAS-789")
	if err != nil {
		t.Fatalf("UpdateDailyNote() error: %v", err)
	}

	// Read the updated content
	content, err := os.ReadFile(dailyNotePath)
	if err != nil {
		t.Fatalf("Failed to read updated note: %v", err)
	}

	// Should add a Log section
	if !strings.Contains(string(content), "## Log") {
		t.Error("UpdateDailyNote() did not add Log section")
	}

	// Should add the ticket link
	if !strings.Contains(string(content), "[[FRAAS-789]]") {
		t.Error("UpdateDailyNote() did not add ticket link")
	}
}

func TestCreateJiraNote_WithTemplate(t *testing.T) {
	// Create a temporary vault directory
	tmpDir, err := os.MkdirTemp("", "obsidian-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create templates directory
	templatesDir := filepath.Join(tmpDir, "templates")
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("Failed to create templates dir: %v", err)
	}

	// Create a Jira template
	templateContent := `# <Insert ticket title or short summary here>

Created: <% tp.date.now("YYYY-MM-DD") %>

## Summary

Write summary here.

## Notes

`
	templatePath := filepath.Join(templatesDir, "Jira.md")
	if err := os.WriteFile(templatePath, []byte(templateContent), 0644); err != nil {
		t.Fatalf("Failed to write template: %v", err)
	}

	nm := NewNoteManager(tmpDir, "templates", "Areas", "Daily", false)

	jiraInfo := &JiraInfo{
		Type:        "Bug",
		Summary:     "Fix login issue",
		Status:      "Open",
		Description: "Users cannot log in",
	}

	content, err := nm.createJiraNote("FRAAS-100", jiraInfo)
	if err != nil {
		t.Fatalf("createJiraNote() error: %v", err)
	}

	// Summary should be replaced
	if !strings.Contains(content, "# Fix login issue") {
		t.Error("createJiraNote() did not replace summary placeholder")
	}

	// Date placeholder should be replaced with actual date
	if strings.Contains(content, "tp.date.now") {
		t.Error("createJiraNote() did not replace date placeholder")
	}
}

func TestCreateJiraNote_WithoutTemplate(t *testing.T) {
	// Create a temporary vault directory with no template
	tmpDir, err := os.MkdirTemp("", "obsidian-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	nm := NewNoteManager(tmpDir, "templates", "Areas", "Daily", false)

	jiraInfo := &JiraInfo{
		Type:        "Story",
		Summary:     "New feature",
		Status:      "In Progress",
		Description: "Implement new feature",
	}

	content, err := nm.createJiraNote("FRAAS-200", jiraInfo)
	if err != nil {
		t.Fatalf("createJiraNote() error: %v", err)
	}

	// Should create default note with JIRA info
	if !strings.Contains(content, "# New feature") {
		t.Error("createJiraNote() missing title")
	}
	if !strings.Contains(content, "## Summary") {
		t.Error("createJiraNote() missing Summary section")
	}
	if !strings.Contains(content, "## JIRA Details") {
		t.Error("createJiraNote() missing JIRA Details section")
	}
	if !strings.Contains(content, "**Type:** Story") {
		t.Error("createJiraNote() missing Type")
	}
	if !strings.Contains(content, "**Status:** In Progress") {
		t.Error("createJiraNote() missing Status")
	}
}

func TestCreateDefaultJiraNote(t *testing.T) {
	nm := NewNoteManager("/vault", "templates", "areas", "daily", false)

	jiraInfo := &JiraInfo{
		Type:        "Task",
		Summary:     "Do something",
		Status:      "Done",
		Description: "Task description",
	}

	content := nm.createDefaultJiraNote("TEST-999", jiraInfo)

	if !strings.Contains(content, "# Do something") {
		t.Error("createDefaultJiraNote() missing title (should use summary)")
	}
	if !strings.Contains(content, "## Summary") {
		t.Error("createDefaultJiraNote() missing Summary section")
	}
	if !strings.Contains(content, "## JIRA Details") {
		t.Error("createDefaultJiraNote() missing JIRA Details")
	}
	if !strings.Contains(content, "## Notes") {
		t.Error("createDefaultJiraNote() missing Notes section")
	}
	if !strings.Contains(content, "## Log") {
		t.Error("createDefaultJiraNote() missing Log section")
	}
}

func TestCreateDefaultJiraNote_NoSummary(t *testing.T) {
	nm := NewNoteManager("/vault", "templates", "areas", "daily", false)

	jiraInfo := &JiraInfo{
		Type:   "Task",
		Status: "Open",
	}

	content := nm.createDefaultJiraNote("TEST-888", jiraInfo)

	// Should use ticket as title when no summary
	if !strings.Contains(content, "# TEST-888") {
		t.Error("createDefaultJiraNote() should use ticket as title when no summary")
	}
}

func TestBuildJiraSection_PartialInfo(t *testing.T) {
	nm := NewNoteManager("/vault", "templates", "areas", "daily", false)

	tests := []struct {
		name     string
		jiraInfo *JiraInfo
		contains []string
		missing  []string
	}{
		{
			name: "only type",
			jiraInfo: &JiraInfo{
				Type: "Bug",
			},
			contains: []string{"**Type:** Bug"},
			missing:  []string{"**Status:**", "**Description:**"},
		},
		{
			name: "only status",
			jiraInfo: &JiraInfo{
				Status: "Open",
			},
			contains: []string{"**Status:** Open"},
			missing:  []string{"**Type:**", "**Description:**"},
		},
		{
			name: "only description",
			jiraInfo: &JiraInfo{
				Description: "Some description",
			},
			contains: []string{"**Description:**", "Some description"},
			missing:  []string{"**Type:**", "**Status:**"},
		},
		{
			name:     "empty info",
			jiraInfo: &JiraInfo{},
			contains: []string{"## JIRA Details"},
			missing:  []string{"**Type:**", "**Status:**", "**Description:**"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := nm.buildJiraSection(tt.jiraInfo)

			for _, s := range tt.contains {
				if !strings.Contains(result, s) {
					t.Errorf("buildJiraSection() should contain %q", s)
				}
			}

			for _, s := range tt.missing {
				if strings.Contains(result, s) {
					t.Errorf("buildJiraSection() should not contain %q", s)
				}
			}
		})
	}
}

// Helper function to get today's date in YYYY-MM-DD format
func getTodayDate() string {
	return time.Now().Format("2006-01-02")
}
