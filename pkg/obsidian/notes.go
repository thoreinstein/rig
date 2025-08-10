package obsidian

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// NoteManager handles Obsidian note operations
type NoteManager struct {
	VaultPath    string
	TemplatesDir string
	AreasDir     string
	DailyDir     string
	Verbose      bool
}

// NewNoteManager creates a new NoteManager
func NewNoteManager(vaultPath, templatesDir, areasDir, dailyDir string, verbose bool) *NoteManager {
	return &NoteManager{
		VaultPath:    vaultPath,
		TemplatesDir: templatesDir,
		AreasDir:     areasDir,
		DailyDir:     dailyDir,
		Verbose:      verbose,
	}
}

// CreateTicketNote creates or updates a ticket note in Obsidian
func (nm *NoteManager) CreateTicketNote(ticketType, ticket string, jiraInfo *JiraInfo) (string, error) {
	// Determine vault subdirectory based on ticket type
	var vaultSubdir string
	switch ticketType {
	case "incident":
		vaultSubdir = "Incidents"
	default:
		vaultSubdir = "Jira"
	}
	
	// Create full note path
	notePath := filepath.Join(nm.VaultPath, nm.AreasDir, vaultSubdir, ticketType, ticket+".md")
	noteDir := filepath.Dir(notePath)
	
	if nm.Verbose {
		fmt.Printf("Creating note at: %s\n", notePath)
	}
	
	// Check if vault exists
	if !nm.vaultExists() {
		return "", fmt.Errorf("vault path not found at %s", nm.VaultPath)
	}
	
	// Create directory if it doesn't exist
	if err := os.MkdirAll(noteDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create note directory: %w", err)
	}
	
	// Check if note already exists
	if _, err := os.Stat(notePath); err == nil {
		if nm.Verbose {
			fmt.Printf("Note already exists at %s\n", notePath)
		}
		return notePath, nil
	}
	
	// Create the note content
	var content string
	var err error
	
	if ticketType != "incident" && jiraInfo != nil {
		content, err = nm.createJiraNote(ticket, jiraInfo)
	} else {
		content, err = nm.createBasicNote(ticket, ticketType)
	}
	
	if err != nil {
		return "", fmt.Errorf("failed to create note content: %w", err)
	}
	
	// Write the note
	if err := os.WriteFile(notePath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write note: %w", err)
	}
	
	if nm.Verbose {
		fmt.Printf("Note created successfully at %s\n", notePath)
	}
	
	return notePath, nil
}

// JiraInfo holds JIRA ticket information
type JiraInfo struct {
	Type        string
	Summary     string
	Status      string
	Description string
}

// createJiraNote creates a note with JIRA template and information
func (nm *NoteManager) createJiraNote(ticket string, jiraInfo *JiraInfo) (string, error) {
	templatePath := filepath.Join(nm.VaultPath, nm.TemplatesDir, "Jira.md")
	
	var content string
	
	// Try to use template if it exists
	if _, err := os.Stat(templatePath); err == nil {
		templateBytes, err := os.ReadFile(templatePath)
		if err != nil {
			return "", fmt.Errorf("failed to read template: %w", err)
		}
		content = string(templateBytes)
		
		// Replace template placeholders
		if jiraInfo.Summary != "" {
			content = strings.ReplaceAll(content, "<Insert ticket title or short summary here>", jiraInfo.Summary)
		}
		
		// Replace date placeholder
		today := time.Now().Format("2006-01-02")
		content = strings.ReplaceAll(content, "<% tp.date.now(\"YYYY-MM-DD\") %>", today)
		
		// Add JIRA details section after ## Summary
		if jiraInfo.Type != "" || jiraInfo.Status != "" || jiraInfo.Description != "" {
			jiraSection := nm.buildJiraSection(jiraInfo)
			content = nm.insertAfterSummary(content, jiraSection)
		}
	} else {
		// Create basic JIRA note if no template
		content = nm.createDefaultJiraNote(ticket, jiraInfo)
	}
	
	return content, nil
}

// createBasicNote creates a basic note without template
func (nm *NoteManager) createBasicNote(ticket, ticketType string) (string, error) {
	today := time.Now().Format("2006-01-02")
	
	content := fmt.Sprintf(`# %s

## Summary

Work on %s ticket: %s

## Notes

- Created: %s

## Log

`, ticket, strings.Title(ticketType), ticket, today)
	
	return content, nil
}

// buildJiraSection creates the JIRA details section
func (nm *NoteManager) buildJiraSection(jiraInfo *JiraInfo) string {
	var section strings.Builder
	
	section.WriteString("## JIRA Details\n\n")
	
	if jiraInfo.Type != "" {
		section.WriteString(fmt.Sprintf("**Type:** %s\n", jiraInfo.Type))
	}
	
	if jiraInfo.Status != "" {
		section.WriteString(fmt.Sprintf("**Status:** %s\n", jiraInfo.Status))
	}
	
	if jiraInfo.Description != "" {
		section.WriteString(fmt.Sprintf("\n**Description:**\n%s\n", jiraInfo.Description))
	}
	
	section.WriteString("\n")
	
	return section.String()
}

// insertAfterSummary inserts content after the ## Summary section
func (nm *NoteManager) insertAfterSummary(content, insertion string) string {
	lines := strings.Split(content, "\n")
	var result []string
	summaryFound := false
	insertionDone := false
	
	for i, line := range lines {
		result = append(result, line)
		
		if strings.HasPrefix(line, "## Summary") {
			summaryFound = true
			continue
		}
		
		if summaryFound && !insertionDone {
			// Look for the next section or end of summary content
			if (strings.HasPrefix(line, "## ") && !strings.HasPrefix(line, "## Summary")) || i == len(lines)-1 {
				// Insert before this line
				result = result[:len(result)-1] // Remove the current line
				result = append(result, "")     // Add blank line
				result = append(result, strings.Split(insertion, "\n")...)
				result = append(result, line) // Add back the current line
				insertionDone = true
			}
		}
	}
	
	// If we never found a place to insert, append at the end
	if summaryFound && !insertionDone {
		result = append(result, "")
		result = append(result, strings.Split(insertion, "\n")...)
	}
	
	return strings.Join(result, "\n")
}

// createDefaultJiraNote creates a default JIRA note structure
func (nm *NoteManager) createDefaultJiraNote(ticket string, jiraInfo *JiraInfo) string {
	today := time.Now().Format("2006-01-02")
	
	var content strings.Builder
	
	title := ticket
	if jiraInfo.Summary != "" {
		title = jiraInfo.Summary
	}
	
	content.WriteString(fmt.Sprintf("# %s\n\n", title))
	content.WriteString("## Summary\n\n")
	
	if jiraInfo != nil {
		content.WriteString(nm.buildJiraSection(jiraInfo))
	}
	
	content.WriteString("## Notes\n\n")
	content.WriteString(fmt.Sprintf("- Created: %s\n\n", today))
	content.WriteString("## Log\n\n")
	
	return content.String()
}

// UpdateDailyNote adds an entry to the daily note
func (nm *NoteManager) UpdateDailyNote(ticket string) error {
	today := time.Now().Format("2006-01-02")
	currentTime := time.Now().Format("15:04")
	dailyNotePath := filepath.Join(nm.VaultPath, nm.DailyDir, today+".md")
	
	if nm.Verbose {
		fmt.Printf("Updating daily note at: %s\n", dailyNotePath)
	}
	
	// Check if daily note exists
	if _, err := os.Stat(dailyNotePath); os.IsNotExist(err) {
		if nm.Verbose {
			fmt.Printf("Daily note for %s not found, skipping update\n", today)
		}
		return nil // Don't fail if daily note doesn't exist
	}
	
	// Read existing content
	content, err := os.ReadFile(dailyNotePath)
	if err != nil {
		return fmt.Errorf("failed to read daily note: %w", err)
	}
	
	// Create log entry
	logEntry := fmt.Sprintf("- [%s] [[%s]]", currentTime, ticket)
	
	// Update the daily note
	updatedContent := nm.insertLogEntry(string(content), logEntry)
	
	// Write back to file
	if err := os.WriteFile(dailyNotePath, []byte(updatedContent), 0644); err != nil {
		return fmt.Errorf("failed to update daily note: %w", err)
	}
	
	if nm.Verbose {
		fmt.Printf("Added log entry to daily note: %s\n", logEntry)
	}
	
	return nil
}

// insertLogEntry inserts a log entry into the daily note
func (nm *NoteManager) insertLogEntry(content, logEntry string) string {
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

// vaultExists checks if the Obsidian vault exists
func (nm *NoteManager) vaultExists() bool {
	_, err := os.Stat(nm.VaultPath)
	return !os.IsNotExist(err)
}