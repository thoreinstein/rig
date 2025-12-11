package notes

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager("/notes", "daily", "/templates", true)

	if m.BasePath != "/notes" {
		t.Errorf("BasePath = %q, want %q", m.BasePath, "/notes")
	}
	if m.DailyDir != "daily" {
		t.Errorf("DailyDir = %q, want %q", m.DailyDir, "daily")
	}
	if m.TemplateDir != "/templates" {
		t.Errorf("TemplateDir = %q, want %q", m.TemplateDir, "/templates")
	}
	if !m.Verbose {
		t.Error("Verbose = false, want true")
	}
}

func TestGetNotePath(t *testing.T) {
	m := NewManager("/notes", "daily", "", false)

	tests := []struct {
		ticketType string
		ticket     string
		want       string
	}{
		{"proj", "proj-123", "/notes/proj/proj-123.md"},
		{"ops", "ops-456", "/notes/ops/ops-456.md"},
		{"hack", "winter-cleanup", "/notes/hack/winter-cleanup.md"},
	}

	for _, tt := range tests {
		t.Run(tt.ticket, func(t *testing.T) {
			got := m.GetNotePath(tt.ticketType, tt.ticket)
			if got != tt.want {
				t.Errorf("GetNotePath(%q, %q) = %q, want %q", tt.ticketType, tt.ticket, got, tt.want)
			}
		})
	}
}

func TestGetDailyNotePath(t *testing.T) {
	m := NewManager("/notes", "daily", "", false)

	got := m.GetDailyNotePath()
	if !strings.HasPrefix(got, "/notes/daily/") {
		t.Errorf("GetDailyNotePath() = %q, want prefix %q", got, "/notes/daily/")
	}
	if !strings.HasSuffix(got, ".md") {
		t.Errorf("GetDailyNotePath() = %q, want suffix %q", got, ".md")
	}
}

func TestCreateTicketNote_Success(t *testing.T) {
	tmpDir := t.TempDir()

	m := NewManager(tmpDir, "daily", "", false)

	data := TicketData{
		Ticket:       "proj-123",
		TicketType:   "proj",
		Date:         "2025-01-15",
		Time:         "14:30",
		Summary:      "Test ticket summary",
		WorktreePath: "/path/to/worktree",
	}

	notePath, err := m.CreateTicketNote(data)
	if err != nil {
		t.Fatalf("CreateTicketNote() error = %v, want nil", err)
	}

	expectedPath := filepath.Join(tmpDir, "proj", "proj-123.md")
	if notePath != expectedPath {
		t.Errorf("CreateTicketNote() path = %q, want %q", notePath, expectedPath)
	}

	// Verify file was created
	if _, err := os.Stat(notePath); os.IsNotExist(err) {
		t.Error("Note file was not created")
	}

	// Verify content
	content, err := os.ReadFile(notePath)
	if err != nil {
		t.Fatalf("Failed to read note: %v", err)
	}

	if !strings.Contains(string(content), "# proj-123") {
		t.Error("Note content missing ticket header")
	}
	if !strings.Contains(string(content), "Test ticket summary") {
		t.Error("Note content missing summary")
	}
	if !strings.Contains(string(content), "2025-01-15") {
		t.Error("Note content missing date")
	}
}

func TestCreateTicketNote_AlreadyExists(t *testing.T) {
	tmpDir := t.TempDir()

	m := NewManager(tmpDir, "daily", "", false)

	// Create note directory and file
	noteDir := filepath.Join(tmpDir, "proj")
	if err := os.MkdirAll(noteDir, 0755); err != nil {
		t.Fatalf("Failed to create note dir: %v", err)
	}

	existingContent := "# Existing note\n\nThis should not be overwritten."
	notePath := filepath.Join(noteDir, "proj-123.md")
	if err := os.WriteFile(notePath, []byte(existingContent), 0644); err != nil {
		t.Fatalf("Failed to create existing note: %v", err)
	}

	data := TicketData{
		Ticket:     "proj-123",
		TicketType: "proj",
	}

	returnedPath, err := m.CreateTicketNote(data)
	if err != nil {
		t.Fatalf("CreateTicketNote() error = %v, want nil", err)
	}

	if returnedPath != notePath {
		t.Errorf("CreateTicketNote() path = %q, want %q", returnedPath, notePath)
	}

	// Verify content was NOT overwritten
	content, err := os.ReadFile(notePath)
	if err != nil {
		t.Fatalf("Failed to read note: %v", err)
	}

	if string(content) != existingContent {
		t.Error("Existing note content was overwritten")
	}
}

func TestCreateTicketNote_HackTemplate(t *testing.T) {
	tmpDir := t.TempDir()

	m := NewManager(tmpDir, "daily", "", false)

	data := TicketData{
		Ticket:       "winter-cleanup",
		TicketType:   "hack",
		Date:         "2025-01-15",
		WorktreePath: "/path/to/worktree",
	}

	notePath, err := m.CreateTicketNote(data)
	if err != nil {
		t.Fatalf("CreateTicketNote() error = %v, want nil", err)
	}

	content, err := os.ReadFile(notePath)
	if err != nil {
		t.Fatalf("Failed to read note: %v", err)
	}

	// Hack template should have "Goal" section instead of JIRA fields
	if !strings.Contains(string(content), "## Goal") {
		t.Error("Hack note should contain Goal section")
	}
}

func TestCreateTicketNote_BasePathNotExists(t *testing.T) {
	m := NewManager("/nonexistent/path", "daily", "", false)

	data := TicketData{
		Ticket:     "proj-123",
		TicketType: "proj",
	}

	_, err := m.CreateTicketNote(data)
	if err == nil {
		t.Fatal("CreateTicketNote() expected error for nonexistent base path, got nil")
	}

	if !strings.Contains(err.Error(), "notes directory does not exist") {
		t.Errorf("Error = %q, want to contain 'notes directory does not exist'", err.Error())
	}
}

func TestUpdateDailyNote_CreateNew(t *testing.T) {
	tmpDir := t.TempDir()

	m := NewManager(tmpDir, "daily", "", false)

	err := m.UpdateDailyNote("proj-123", "proj")
	if err != nil {
		t.Fatalf("UpdateDailyNote() error = %v, want nil", err)
	}

	// Verify daily note was created
	dailyPath := m.GetDailyNotePath()
	content, err := os.ReadFile(dailyPath)
	if err != nil {
		t.Fatalf("Failed to read daily note: %v", err)
	}

	// Should contain relative link
	if !strings.Contains(string(content), "[proj-123](../proj/proj-123.md)") {
		t.Errorf("Daily note should contain relative link, got: %s", string(content))
	}
}

func TestUpdateDailyNote_AppendToExisting(t *testing.T) {
	tmpDir := t.TempDir()

	m := NewManager(tmpDir, "daily", "", false)

	// Create existing daily note
	dailyDir := filepath.Join(tmpDir, "daily")
	if err := os.MkdirAll(dailyDir, 0755); err != nil {
		t.Fatalf("Failed to create daily dir: %v", err)
	}

	dailyPath := m.GetDailyNotePath()
	existingContent := "# 2025-01-15\n\n## Notes\n\nSome notes here.\n\n## Log\n\n- [09:00] [ops-456](../ops/ops-456.md)\n"
	if err := os.WriteFile(dailyPath, []byte(existingContent), 0644); err != nil {
		t.Fatalf("Failed to create existing daily note: %v", err)
	}

	err := m.UpdateDailyNote("proj-123", "proj")
	if err != nil {
		t.Fatalf("UpdateDailyNote() error = %v, want nil", err)
	}

	content, err := os.ReadFile(dailyPath)
	if err != nil {
		t.Fatalf("Failed to read daily note: %v", err)
	}

	// Should contain both entries
	if !strings.Contains(string(content), "[ops-456]") {
		t.Error("Daily note should still contain existing entry")
	}
	if !strings.Contains(string(content), "[proj-123](../proj/proj-123.md)") {
		t.Error("Daily note should contain new entry")
	}
}

func TestInsertLogEntry_WithLogSection(t *testing.T) {
	m := NewManager("/notes", "daily", "", false)

	content := "# 2025-01-15\n\n## Notes\n\n## Log\n\n## Other\n"
	entry := "- [14:30] [proj-123](../proj/proj-123.md)"

	result := m.insertLogEntry(content, entry)

	// Entry should be after ## Log but before ## Other
	logIdx := strings.Index(result, "## Log")
	otherIdx := strings.Index(result, "## Other")
	entryIdx := strings.Index(result, entry)

	if entryIdx < logIdx || entryIdx > otherIdx {
		t.Errorf("Entry not inserted in correct position. Log: %d, Entry: %d, Other: %d", logIdx, entryIdx, otherIdx)
	}
}

func TestInsertLogEntry_NoLogSection(t *testing.T) {
	m := NewManager("/notes", "daily", "", false)

	content := "# 2025-01-15\n\n## Notes\n\nSome content.\n"
	entry := "- [14:30] [proj-123](../proj/proj-123.md)"

	result := m.insertLogEntry(content, entry)

	// Should have added ## Log section at end
	if !strings.Contains(result, "## Log\n"+entry) {
		t.Error("Log section should be added at end with entry")
	}
}

func TestRenderTemplate_EmbeddedTemplate(t *testing.T) {
	m := NewManager("/notes", "daily", "", false)

	data := TicketData{
		Ticket:       "proj-123",
		TicketType:   "proj",
		Date:         "2025-01-15",
		Summary:      "Test summary",
		WorktreePath: "/path/to/worktree",
	}

	content, err := m.renderTemplate("ticket.md.tmpl", data)
	if err != nil {
		t.Fatalf("renderTemplate() error = %v, want nil", err)
	}

	if !strings.Contains(content, "# proj-123") {
		t.Error("Template should contain ticket header")
	}
	if !strings.Contains(content, "Test summary") {
		t.Error("Template should contain summary")
	}
}

func TestRenderTemplate_UserOverride(t *testing.T) {
	tmpDir := t.TempDir()

	// Create user template
	userTemplate := "# Custom: {{.Ticket}}\n\nUser template content for {{.Summary}}"
	if err := os.WriteFile(filepath.Join(tmpDir, "ticket.md.tmpl"), []byte(userTemplate), 0644); err != nil {
		t.Fatalf("Failed to create user template: %v", err)
	}

	m := NewManager("/notes", "daily", tmpDir, false)

	data := TicketData{
		Ticket:  "proj-123",
		Summary: "Test summary",
	}

	content, err := m.renderTemplate("ticket.md.tmpl", data)
	if err != nil {
		t.Fatalf("renderTemplate() error = %v, want nil", err)
	}

	if !strings.Contains(content, "# Custom: proj-123") {
		t.Error("Should use user template")
	}
	if !strings.Contains(content, "User template content") {
		t.Error("Should use user template content")
	}
}

func TestRenderTemplate_InvalidTemplate(t *testing.T) {
	m := NewManager("/notes", "daily", "", false)

	_, err := m.renderTemplate("nonexistent.md.tmpl", TicketData{})
	if err == nil {
		t.Fatal("renderTemplate() expected error for nonexistent template, got nil")
	}
}
