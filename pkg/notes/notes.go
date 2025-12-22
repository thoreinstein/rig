package notes

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/cockroachdb/errors"
)

//go:embed templates/*.tmpl
var defaultTemplates embed.FS

// Manager handles markdown note operations
type Manager struct {
	BasePath    string // Root path for notes
	DailyDir    string // Relative path for daily notes
	TemplateDir string // Optional user template directory
	Verbose     bool
}

// TicketData holds data for template rendering
type TicketData struct {
	Ticket       string // e.g., "proj-123"
	TicketType   string // e.g., "proj"
	Date         string // e.g., "2025-01-15"
	Time         string // e.g., "14:30"
	Summary      string // From JIRA (if available)
	Status       string // From JIRA (if available)
	Description  string // From JIRA (if available)
	RepoName     string // e.g., "myrepo"
	RepoPath     string // e.g., "/Users/jim/src/myorg/myrepo"
	WorktreePath string // e.g., "/Users/jim/src/myorg/myrepo/proj/proj-123"
}

// NewManager creates a new note Manager
func NewManager(basePath, dailyDir, templateDir string, verbose bool) *Manager {
	return &Manager{
		BasePath:    basePath,
		DailyDir:    dailyDir,
		TemplateDir: templateDir,
		Verbose:     verbose,
	}
}

// GetNotePath returns the path for a ticket note
func (m *Manager) GetNotePath(ticketType, ticket string) string {
	return filepath.Join(m.BasePath, ticketType, ticket+".md")
}

// GetDailyNotePath returns the path for today's daily note
func (m *Manager) GetDailyNotePath() string {
	today := time.Now().Format("2006-01-02")
	return filepath.Join(m.BasePath, m.DailyDir, today+".md")
}

// CreateTicketNote creates or returns existing ticket note
func (m *Manager) CreateTicketNote(data TicketData) (string, error) {
	notePath := m.GetNotePath(data.TicketType, data.Ticket)
	noteDir := filepath.Dir(notePath)

	if m.Verbose {
		fmt.Printf("Creating note at: %s\n", notePath)
	}

	// Check if base path exists
	if _, err := os.Stat(m.BasePath); os.IsNotExist(err) {
		return "", errors.Newf("notes directory does not exist: %s. Create it or update 'notes.path' in config", m.BasePath)
	}

	// Create directory if it doesn't exist (0700 for user-only access)
	if err := os.MkdirAll(noteDir, 0700); err != nil {
		return "", errors.Wrap(err, "failed to create note directory")
	}

	// Check if note already exists
	if _, err := os.Stat(notePath); err == nil {
		if m.Verbose {
			fmt.Printf("Note already exists at %s\n", notePath)
		}
		return notePath, nil
	}

	// Determine which template to use
	templateName := "ticket.md.tmpl"
	if data.TicketType == "hack" {
		templateName = "hack.md.tmpl"
	}

	// Fill in date/time if not set
	if data.Date == "" {
		data.Date = time.Now().Format("2006-01-02")
	}
	if data.Time == "" {
		data.Time = time.Now().Format("15:04")
	}

	// Render template
	content, err := m.renderTemplate(templateName, data)
	if err != nil {
		return "", errors.Wrap(err, "failed to render template")
	}

	// Write the note with restricted permissions (may contain command history)
	if err := os.WriteFile(notePath, []byte(content), 0600); err != nil {
		return "", errors.Wrap(err, "failed to write note")
	}

	if m.Verbose {
		fmt.Printf("Note created successfully at %s\n", notePath)
	}

	return notePath, nil
}

// UpdateDailyNote adds an entry to the daily note, creating it if necessary
func (m *Manager) UpdateDailyNote(ticket, ticketType string) error {
	today := time.Now().Format("2006-01-02")
	currentTime := time.Now().Format("15:04")
	dailyNotePath := m.GetDailyNotePath()

	if m.Verbose {
		fmt.Printf("Updating daily note at: %s\n", dailyNotePath)
	}

	var content string

	// Check if daily note exists, create if not
	if _, statErr := os.Stat(dailyNotePath); os.IsNotExist(statErr) {
		// Create the daily directory if needed (0700 for user-only access)
		dailyDir := filepath.Dir(dailyNotePath)
		if err := os.MkdirAll(dailyDir, 0700); err != nil {
			return errors.Wrap(err, "failed to create daily notes directory")
		}

		// Render daily template
		data := TicketData{Date: today}
		rendered, err := m.renderTemplate("daily.md.tmpl", data)
		if err != nil {
			return errors.Wrap(err, "failed to render daily template")
		}
		content = rendered

		if m.Verbose {
			fmt.Printf("Creating new daily note for %s\n", today)
		}
	} else {
		// Read existing content
		contentBytes, err := os.ReadFile(dailyNotePath)
		if err != nil {
			return errors.Wrap(err, "failed to read daily note")
		}
		content = string(contentBytes)
	}

	// Calculate relative path from daily note to ticket note
	// Daily note: {base}/daily/2025-01-15.md
	// Ticket note: {base}/proj/proj-123.md
	// Relative path: ../proj/proj-123.md
	relativePath := filepath.Join("..", ticketType, ticket+".md")

	// Create log entry with relative markdown link
	logEntry := fmt.Sprintf("- [%s] [%s](%s)", currentTime, ticket, relativePath)

	// Update the daily note
	updatedContent := m.insertLogEntry(content, logEntry)

	// Write back to file with restricted permissions
	if err := os.WriteFile(dailyNotePath, []byte(updatedContent), 0600); err != nil {
		return errors.Wrap(err, "failed to update daily note")
	}

	if m.Verbose {
		fmt.Printf("Added log entry to daily note: %s\n", logEntry)
	}

	return nil
}

// renderTemplate renders a template with the given data
// It checks user template directory first, then falls back to embedded templates
func (m *Manager) renderTemplate(name string, data TicketData) (string, error) {
	var tmplContent []byte
	var err error

	// Try user template directory first
	if m.TemplateDir != "" {
		userTemplatePath := filepath.Join(m.TemplateDir, name)
		if _, statErr := os.Stat(userTemplatePath); statErr == nil {
			tmplContent, err = os.ReadFile(userTemplatePath)
			if err != nil {
				return "", errors.Wrapf(err, "failed to read user template %s", userTemplatePath)
			}
			if m.Verbose {
				fmt.Printf("Using user template: %s\n", userTemplatePath)
			}
		}
	}

	// Fall back to embedded template
	if tmplContent == nil {
		tmplContent, err = defaultTemplates.ReadFile("templates/" + name)
		if err != nil {
			return "", errors.Wrapf(err, "failed to read embedded template %s", name)
		}
	}

	// Parse and execute template
	tmpl, err := template.New(name).Parse(string(tmplContent))
	if err != nil {
		return "", errors.Wrapf(err, "failed to parse template %s", name)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", errors.Wrapf(err, "failed to execute template %s", name)
	}

	return buf.String(), nil
}

// insertLogEntry inserts a log entry into the note content
func (m *Manager) insertLogEntry(content, logEntry string) string {
	lines := strings.Split(content, "\n")

	// Look for ## Log section
	for i, line := range lines {
		if strings.HasPrefix(line, "## Log") {
			// Find the end of the log section
			insertIndex := len(lines) // Default to end of file

			for j := i + 1; j < len(lines); j++ {
				if strings.HasPrefix(lines[j], "## ") {
					insertIndex = j
					break
				}
			}

			// Insert the log entry
			newLines := make([]string, 0, len(lines)+1)
			newLines = append(newLines, lines[:insertIndex]...)
			newLines = append(newLines, logEntry)
			newLines = append(newLines, lines[insertIndex:]...)

			return strings.Join(newLines, "\n")
		}
	}

	// If no ## Log section found, add it at the end
	return content + "\n\n## Log\n" + logEntry
}
