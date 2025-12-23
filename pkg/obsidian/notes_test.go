package obsidian

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewNoteManager(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		vaultPath    string
		templatesDir string
		areasDir     string
		dailyDir     string
		verbose      bool
	}{
		{
			name:         "all fields populated",
			vaultPath:    "/vault",
			templatesDir: "templates",
			areasDir:     "areas",
			dailyDir:     "daily",
			verbose:      true,
		},
		{
			name:         "verbose disabled",
			vaultPath:    "/other/vault",
			templatesDir: "tmpl",
			areasDir:     "Areas",
			dailyDir:     "Daily",
			verbose:      false,
		},
		{
			name:         "empty paths",
			vaultPath:    "",
			templatesDir: "",
			areasDir:     "",
			dailyDir:     "",
			verbose:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			nm := NewNoteManager(tt.vaultPath, tt.templatesDir, tt.areasDir, tt.dailyDir, tt.verbose)

			if nm.VaultPath != tt.vaultPath {
				t.Errorf("VaultPath = %q, want %q", nm.VaultPath, tt.vaultPath)
			}
			if nm.TemplatesDir != tt.templatesDir {
				t.Errorf("TemplatesDir = %q, want %q", nm.TemplatesDir, tt.templatesDir)
			}
			if nm.AreasDir != tt.areasDir {
				t.Errorf("AreasDir = %q, want %q", nm.AreasDir, tt.areasDir)
			}
			if nm.DailyDir != tt.dailyDir {
				t.Errorf("DailyDir = %q, want %q", nm.DailyDir, tt.dailyDir)
			}
			if nm.Verbose != tt.verbose {
				t.Errorf("Verbose = %v, want %v", nm.Verbose, tt.verbose)
			}
			if nm.VaultSubdir != "" {
				t.Errorf("VaultSubdir should be empty after construction, got %q", nm.VaultSubdir)
			}
		})
	}
}

func TestSetVaultSubdir(t *testing.T) {
	t.Parallel()

	nm := NewNoteManager("/vault", "templates", "areas", "daily", false)

	if nm.VaultSubdir != "" {
		t.Errorf("VaultSubdir should be empty after construction, got %q", nm.VaultSubdir)
	}

	nm.SetVaultSubdir("CustomDir")
	if nm.VaultSubdir != "CustomDir" {
		t.Errorf("SetVaultSubdir() = %q, want %q", nm.VaultSubdir, "CustomDir")
	}

	// Test overwriting
	nm.SetVaultSubdir("AnotherDir")
	if nm.VaultSubdir != "AnotherDir" {
		t.Errorf("SetVaultSubdir() after overwrite = %q, want %q", nm.VaultSubdir, "AnotherDir")
	}

	// Test empty string
	nm.SetVaultSubdir("")
	if nm.VaultSubdir != "" {
		t.Errorf("SetVaultSubdir() after clearing = %q, want empty", nm.VaultSubdir)
	}
}

func TestCreateTicketNote_SubdirSelection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		ticketType     string
		vaultSubdir    string
		expectedSubdir string
	}{
		{
			name:           "tickets subdir",
			ticketType:     "proj",
			vaultSubdir:    "Tickets",
			expectedSubdir: "Tickets",
		},
		{
			name:           "incident subdir",
			ticketType:     "incident",
			vaultSubdir:    "Incidents",
			expectedSubdir: "Incidents",
		},
		{
			name:           "hack subdir",
			ticketType:     "hack",
			vaultSubdir:    "Hacks",
			expectedSubdir: "Hacks",
		},
		{
			name:           "custom subdir",
			ticketType:     "proj",
			vaultSubdir:    "CustomTickets",
			expectedSubdir: "CustomTickets",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create a temporary vault directory per subtest
			tmpDir := t.TempDir()

			nm := NewNoteManager(tmpDir, "templates", "Areas", "Daily", false)
			nm.SetVaultSubdir(tt.vaultSubdir)

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
		})
	}
}

func TestCreateTicketNote_ExistingNote(t *testing.T) {
	t.Parallel()

	// Create a temporary vault directory
	tmpDir := t.TempDir()

	nm := NewNoteManager(tmpDir, "templates", "Areas", "Daily", false)

	// Create the first note
	notePath1, err := nm.CreateTicketNote("jira", "TEST-456", nil)
	if err != nil {
		t.Fatalf("CreateTicketNote() first call error: %v", err)
	}

	// Get original content
	originalContent, err := os.ReadFile(notePath1)
	if err != nil {
		t.Fatalf("Failed to read original note: %v", err)
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

	// Content should remain unchanged (not overwritten)
	currentContent, err := os.ReadFile(notePath1)
	if err != nil {
		t.Fatalf("Failed to read note after second call: %v", err)
	}

	if string(originalContent) != string(currentContent) {
		t.Error("CreateTicketNote() should not overwrite existing note")
	}
}

func TestCreateTicketNote_MissingVault(t *testing.T) {
	t.Parallel()

	nm := NewNoteManager("/nonexistent/vault/path", "templates", "Areas", "Daily", false)

	_, err := nm.CreateTicketNote("jira", "TEST-789", nil)
	if err == nil {
		t.Error("CreateTicketNote() expected error for missing vault, got nil")
	}

	if !strings.Contains(err.Error(), "vault path not found") {
		t.Errorf("Error should mention vault path not found, got: %v", err)
	}
}

func TestCreateTicketNote_WithJiraInfo(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	nm := NewNoteManager(tmpDir, "templates", "Areas", "Daily", false)
	nm.SetVaultSubdir("Tickets")

	jiraInfo := &JiraInfo{
		Type:        "Bug",
		Summary:     "Fix critical issue",
		Status:      "In Progress",
		Description: "Detailed description of the bug",
	}

	notePath, err := nm.CreateTicketNote("proj", "PROJ-100", jiraInfo)
	if err != nil {
		t.Fatalf("CreateTicketNote() error: %v", err)
	}

	content, err := os.ReadFile(notePath)
	if err != nil {
		t.Fatalf("Failed to read note: %v", err)
	}

	contentStr := string(content)

	// Should contain JIRA info
	if !strings.Contains(contentStr, "Fix critical issue") {
		t.Error("Note should contain JIRA summary")
	}
	if !strings.Contains(contentStr, "## JIRA Details") {
		t.Error("Note should contain JIRA Details section")
	}
	if !strings.Contains(contentStr, "**Type:** Bug") {
		t.Error("Note should contain JIRA type")
	}
	if !strings.Contains(contentStr, "**Status:** In Progress") {
		t.Error("Note should contain JIRA status")
	}
}

func TestCreateTicketNote_IncidentType(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	nm := NewNoteManager(tmpDir, "templates", "Areas", "Daily", false)
	nm.SetVaultSubdir("Incidents")

	// Incident type should use basic note even with jiraInfo provided
	jiraInfo := &JiraInfo{
		Type:    "Incident",
		Summary: "Production outage",
		Status:  "Open",
	}

	notePath, err := nm.CreateTicketNote("incident", "INC-001", jiraInfo)
	if err != nil {
		t.Fatalf("CreateTicketNote() error: %v", err)
	}

	content, err := os.ReadFile(notePath)
	if err != nil {
		t.Fatalf("Failed to read note: %v", err)
	}

	contentStr := string(content)

	// Should use basic note template (no JIRA details)
	if !strings.Contains(contentStr, "# INC-001") {
		t.Error("Incident note should have ticket as title")
	}
	if !strings.Contains(contentStr, "Incident ticket") {
		t.Error("Incident note should mention ticket type")
	}
}

func TestCreateBasicNote(t *testing.T) {
	t.Parallel()

	nm := NewNoteManager("/vault", "templates", "areas", "daily", false)

	tests := []struct {
		name       string
		ticket     string
		ticketType string
		wantTitle  string
		wantType   string
	}{
		{
			name:       "fraas ticket",
			ticket:     "FRAAS-123",
			ticketType: "fraas",
			wantTitle:  "# FRAAS-123",
			wantType:   "Fraas ticket",
		},
		{
			name:       "incident ticket",
			ticket:     "INC-456",
			ticketType: "incident",
			wantTitle:  "# INC-456",
			wantType:   "Incident ticket",
		},
		{
			name:       "hack ticket",
			ticket:     "winter-cleanup",
			ticketType: "hack",
			wantTitle:  "# winter-cleanup",
			wantType:   "Hack ticket",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			content, err := nm.createBasicNote(tt.ticket, tt.ticketType)
			if err != nil {
				t.Fatalf("createBasicNote() error: %v", err)
			}

			if !strings.Contains(content, tt.wantTitle) {
				t.Errorf("createBasicNote() missing ticket title %q", tt.wantTitle)
			}
			if !strings.Contains(content, tt.wantType) {
				t.Errorf("createBasicNote() missing ticket type %q", tt.wantType)
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
			if !strings.Contains(content, "Created:") {
				t.Error("createBasicNote() missing Created date")
			}
		})
	}
}

func TestTitleCase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "lowercase word",
			input: "incident",
			want:  "Incident",
		},
		{
			name:  "already capitalized",
			input: "Incident",
			want:  "Incident",
		},
		{
			name:  "all uppercase",
			input: "INCIDENT",
			want:  "INCIDENT",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "single character lowercase",
			input: "a",
			want:  "A",
		},
		{
			name:  "single character uppercase",
			input: "A",
			want:  "A",
		},
		{
			name:  "multi-word (only first capitalized)",
			input: "hello world",
			want:  "Hello world",
		},
		{
			name:  "unicode lowercase",
			input: "über",
			want:  "Über",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := titleCase(tt.input)
			if got != tt.want {
				t.Errorf("titleCase(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestBuildJiraSection(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	nm := NewNoteManager("/vault", "templates", "areas", "daily", false)

	tests := []struct {
		name      string
		content   string
		insertion string
		validate  func(t *testing.T, result string)
	}{
		{
			name: "insert between summary and notes",
			content: `# Ticket

## Summary

This is the summary.

## Notes

Some notes here.`,
			insertion: "## JIRA Details\n\nNew content here.",
			validate: func(t *testing.T, result string) {
				summaryIdx := strings.Index(result, "## Summary")
				jiraIdx := strings.Index(result, "## JIRA Details")
				notesIdx := strings.Index(result, "## Notes")

				if summaryIdx == -1 || jiraIdx == -1 || notesIdx == -1 {
					t.Fatalf("Missing sections: summary=%d, jira=%d, notes=%d",
						summaryIdx, jiraIdx, notesIdx)
				}

				if summaryIdx >= jiraIdx || jiraIdx >= notesIdx {
					t.Errorf("Wrong order: summary=%d, jira=%d, notes=%d",
						summaryIdx, jiraIdx, notesIdx)
				}
			},
		},
		{
			name: "summary at end of file",
			content: `# Ticket

## Summary

This is the summary at the end.`,
			insertion: "## JIRA Details\n\nAppended content.",
			validate: func(t *testing.T, result string) {
				if !strings.Contains(result, "## JIRA Details") {
					t.Error("Missing JIRA Details section")
				}
				if !strings.Contains(result, "Appended content") {
					t.Error("Missing appended content")
				}
			},
		},
		{
			name: "no summary section",
			content: `# Ticket

## Notes

Some notes without summary.`,
			insertion: "## JIRA Details\n\nShould not be inserted.",
			validate: func(t *testing.T, result string) {
				// When no summary section, insertion may not happen in expected location
				// Just verify the original content is preserved
				if !strings.Contains(result, "## Notes") {
					t.Error("Original Notes section should be preserved")
				}
			},
		},
		{
			name: "multiple sections after summary",
			content: `# Ticket

## Summary

Summary content.

## Notes

Notes content.

## Log

Log content.

## References

References content.`,
			insertion: "## JIRA Details\n\nInserted before Notes.",
			validate: func(t *testing.T, result string) {
				summaryIdx := strings.Index(result, "## Summary")
				jiraIdx := strings.Index(result, "## JIRA Details")
				notesIdx := strings.Index(result, "## Notes")
				logIdx := strings.Index(result, "## Log")

				if jiraIdx < summaryIdx || jiraIdx > notesIdx {
					t.Errorf("JIRA should be between Summary and Notes: summary=%d, jira=%d, notes=%d",
						summaryIdx, jiraIdx, notesIdx)
				}
				if logIdx < notesIdx {
					t.Error("Log should come after Notes")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := nm.insertAfterSummary(tt.content, tt.insertion)
			tt.validate(t, result)
		})
	}
}

func TestInsertLogEntry(t *testing.T) {
	t.Parallel()

	nm := NewNoteManager("/vault", "templates", "areas", "daily", false)

	tests := []struct {
		name     string
		content  string
		logEntry string
		validate func(t *testing.T, result string)
	}{
		{
			name: "insert into existing log section",
			content: `# Daily

## Tasks

- Do something

## Log

- [10:00] Previous entry`,
			logEntry: "- [14:30] [[NEW-TICKET]]",
			validate: func(t *testing.T, result string) {
				if !strings.Contains(result, "- [14:30] [[NEW-TICKET]]") {
					t.Error("Missing new log entry")
				}
				if !strings.Contains(result, "- [10:00] Previous entry") {
					t.Error("Previous entry should be preserved")
				}
			},
		},
		{
			name: "append when no log section",
			content: `# Daily

## Tasks

- Do something`,
			logEntry: "- [14:30] [[NEW-TICKET]]",
			validate: func(t *testing.T, result string) {
				if !strings.Contains(result, "## Log") {
					t.Error("Should add Log section")
				}
				if !strings.Contains(result, "- [14:30] [[NEW-TICKET]]") {
					t.Error("Missing log entry")
				}
			},
		},
		{
			name: "log section at end of file",
			content: `# Daily

## Tasks

- Do something

## Log
`,
			logEntry: "- [09:00] [[FIRST-TICKET]]",
			validate: func(t *testing.T, result string) {
				if !strings.Contains(result, "- [09:00] [[FIRST-TICKET]]") {
					t.Error("Missing log entry")
				}
			},
		},
		{
			name: "log section with section after it",
			content: `# Daily

## Log

- [08:00] Existing

## References

Some refs`,
			logEntry: "- [16:00] [[AFTER-TICKET]]",
			validate: func(t *testing.T, result string) {
				logIdx := strings.Index(result, "## Log")
				entryIdx := strings.Index(result, "- [16:00] [[AFTER-TICKET]]")
				refsIdx := strings.Index(result, "## References")

				if entryIdx < logIdx || entryIdx > refsIdx {
					t.Errorf("Entry should be between Log and References: log=%d, entry=%d, refs=%d",
						logIdx, entryIdx, refsIdx)
				}
			},
		},
		{
			name: "multiple existing entries",
			content: `# Daily

## Log

- [08:00] First
- [09:00] Second
- [10:00] Third`,
			logEntry: "- [11:00] Fourth",
			validate: func(t *testing.T, result string) {
				if !strings.Contains(result, "- [11:00] Fourth") {
					t.Error("Missing new entry")
				}
				// All previous entries should be preserved
				for _, entry := range []string{"First", "Second", "Third"} {
					if !strings.Contains(result, entry) {
						t.Errorf("Missing previous entry: %s", entry)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := nm.insertLogEntry(tt.content, tt.logEntry)
			tt.validate(t, result)
		})
	}
}

func TestVaultExists(t *testing.T) {
	t.Parallel()

	// Create a temporary directory
	tmpDir := t.TempDir()

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
		{
			name:      "empty path",
			vaultPath: "",
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			nm := NewNoteManager(tt.vaultPath, "templates", "areas", "daily", false)
			result := nm.vaultExists()
			if result != tt.expected {
				t.Errorf("vaultExists() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestUpdateDailyNote_ExistingNote(t *testing.T) {
	t.Parallel()

	// Create a temporary vault directory
	tmpDir := t.TempDir()

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

	if err := nm.UpdateDailyNote("FRAAS-123"); err != nil {
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

func TestUpdateDailyNote_CreatesNewNote(t *testing.T) {
	t.Parallel()

	// Create a temporary vault directory
	tmpDir := t.TempDir()

	nm := NewNoteManager(tmpDir, "templates", "Areas", "Daily", false)

	// Should not error when daily note and directory don't exist
	if err := nm.UpdateDailyNote("FRAAS-456"); err != nil {
		t.Fatalf("UpdateDailyNote() should create new note: %v", err)
	}

	// Verify note was created
	today := getTodayDate()
	dailyNotePath := filepath.Join(tmpDir, "Daily", today+".md")

	content, err := os.ReadFile(dailyNotePath)
	if err != nil {
		t.Fatalf("Failed to read created daily note: %v", err)
	}

	if !strings.Contains(string(content), "[[FRAAS-456]]") {
		t.Error("New daily note should contain ticket link")
	}
	if !strings.Contains(string(content), "## Log") {
		t.Error("New daily note should contain Log section")
	}
}

func TestUpdateDailyNote_NoLogSection(t *testing.T) {
	t.Parallel()

	// Create a temporary vault directory
	tmpDir := t.TempDir()

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

	if err := nm.UpdateDailyNote("FRAAS-789"); err != nil {
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

func TestUpdateDailyNote_MultipleUpdates(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	nm := NewNoteManager(tmpDir, "templates", "Areas", "Daily", false)

	// Add multiple tickets to daily note
	tickets := []string{"TICKET-001", "TICKET-002", "TICKET-003"}

	for _, ticket := range tickets {
		if err := nm.UpdateDailyNote(ticket); err != nil {
			t.Fatalf("UpdateDailyNote(%s) error: %v", ticket, err)
		}
	}

	// Verify all tickets are in the note
	today := getTodayDate()
	dailyNotePath := filepath.Join(tmpDir, "Daily", today+".md")

	content, err := os.ReadFile(dailyNotePath)
	if err != nil {
		t.Fatalf("Failed to read daily note: %v", err)
	}

	for _, ticket := range tickets {
		if !strings.Contains(string(content), "[["+ticket+"]]") {
			t.Errorf("Daily note should contain ticket link: %s", ticket)
		}
	}
}

func TestCreateJiraNote_WithTemplate(t *testing.T) {
	t.Parallel()

	// Create a temporary vault directory
	tmpDir := t.TempDir()

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

	// Should contain today's date
	today := time.Now().Format("2006-01-02")
	if !strings.Contains(content, today) {
		t.Error("createJiraNote() should contain today's date")
	}
}

func TestCreateJiraNote_WithoutTemplate(t *testing.T) {
	t.Parallel()

	// Create a temporary vault directory with no template
	tmpDir := t.TempDir()

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

func TestCreateJiraNote_TemplateWithoutJiraInfo(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create templates directory
	templatesDir := filepath.Join(tmpDir, "templates")
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("Failed to create templates dir: %v", err)
	}

	templateContent := `# <Insert ticket title or short summary here>

## Summary

## Notes
`
	templatePath := filepath.Join(templatesDir, "Jira.md")
	if err := os.WriteFile(templatePath, []byte(templateContent), 0644); err != nil {
		t.Fatalf("Failed to write template: %v", err)
	}

	nm := NewNoteManager(tmpDir, "templates", "Areas", "Daily", false)

	// Empty JiraInfo - should use template without JIRA details section
	jiraInfo := &JiraInfo{}

	content, err := nm.createJiraNote("TICKET-001", jiraInfo)
	if err != nil {
		t.Fatalf("createJiraNote() error: %v", err)
	}

	// Should use template but not add JIRA details (all fields empty)
	if !strings.Contains(content, "## Summary") {
		t.Error("Should preserve Summary section from template")
	}
}

func TestCreateDefaultJiraNote(t *testing.T) {
	t.Parallel()

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

	// Check date is present
	today := time.Now().Format("2006-01-02")
	if !strings.Contains(content, today) {
		t.Error("createDefaultJiraNote() missing today's date")
	}
}

func TestCreateDefaultJiraNote_NoSummary(t *testing.T) {
	t.Parallel()

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

func TestCreateDefaultDailyNote(t *testing.T) {
	t.Parallel()

	nm := NewNoteManager("/vault", "templates", "areas", "daily", false)

	tests := []struct {
		name string
		date string
	}{
		{
			name: "standard date",
			date: "2025-01-15",
		},
		{
			name: "end of year",
			date: "2025-12-31",
		},
		{
			name: "leap year date",
			date: "2024-02-29",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			content := nm.createDefaultDailyNote(tt.date)

			if !strings.Contains(content, "# "+tt.date) {
				t.Errorf("createDefaultDailyNote() missing date header: %s", tt.date)
			}
			if !strings.Contains(content, "## Notes") {
				t.Error("createDefaultDailyNote() missing Notes section")
			}
			if !strings.Contains(content, "## Log") {
				t.Error("createDefaultDailyNote() missing Log section")
			}
		})
	}
}

func TestBuildJiraSection_PartialInfo(t *testing.T) {
	t.Parallel()

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
		{
			name: "all fields populated",
			jiraInfo: &JiraInfo{
				Type:        "Epic",
				Status:      "Closed",
				Description: "Full description here",
			},
			contains: []string{"**Type:** Epic", "**Status:** Closed", "**Description:**", "Full description here"},
			missing:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

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

func TestJiraInfo_ZeroValue(t *testing.T) {
	t.Parallel()

	// Test that zero-value JiraInfo works correctly
	info := &JiraInfo{}

	if info.Type != "" {
		t.Error("Zero-value JiraInfo.Type should be empty")
	}
	if info.Summary != "" {
		t.Error("Zero-value JiraInfo.Summary should be empty")
	}
	if info.Status != "" {
		t.Error("Zero-value JiraInfo.Status should be empty")
	}
	if info.Description != "" {
		t.Error("Zero-value JiraInfo.Description should be empty")
	}
}

// Helper function to get today's date in YYYY-MM-DD format
func getTodayDate() string {
	return time.Now().Format("2006-01-02")
}
