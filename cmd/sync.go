package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/spf13/cobra"

	"thoreinstein.com/rig/pkg/config"
	"thoreinstein.com/rig/pkg/jira"
	"thoreinstein.com/rig/pkg/ticket"
)

var syncCmd = &cobra.Command{
	Use:   "sync [ticket]",
	Short: "Sync and update notes",
	Long: `Sync and update notes with latest information.

This command can:
- Update a specific ticket note with fresh JIRA information
- Refresh daily note entries
- Sync multiple tickets at once

Examples:
  rig sync                    # Interactive mode - prompts for ticket
  rig sync proj-123           # Sync specific ticket
  rig sync proj-123 --jira    # Force JIRA refresh
  rig sync --daily            # Update today's daily note`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ticketID := ""
		if len(args) > 0 {
			ticketID = args[0]
		}
		return runSyncCommand(cmd, ticketID)
	},
}
var (
	syncJira  bool
	syncDaily bool
	syncForce bool
)

func init() {
	rootCmd.AddCommand(syncCmd)

	syncCmd.Flags().BoolVar(&syncJira, "jira", false, "Force refresh of JIRA information")
	syncCmd.Flags().BoolVar(&syncDaily, "daily", false, "Update daily note")
	syncCmd.Flags().BoolVar(&syncForce, "force", false, "Force update even if note was recently modified")
}

func runSyncCommand(cmd *cobra.Command, ticketID string) error {
	// Load configuration
	cfg, err := loadConfig()
	if err != nil {
		return errors.Wrap(err, "failed to load configuration")
	}

	// Handle daily note sync
	if syncDaily {
		return syncDailyNote(cmd, cfg)
	}

	// Handle ticket sync
	if ticketID == "" {
		return errors.New("ticket required (or use --daily flag)")
	}

	return syncTicketNote(cmd, cfg, ticketID)
}

func syncTicketNote(cmd *cobra.Command, cfg *config.Config, ticketID string) error {
	// Parse ticket
	ticketInfo, err := ticket.ParseTicket(ticketID)
	if err != nil {
		return err
	}

	if verbose {
		fmt.Printf("Syncing note for ticket: %s\n", ticketInfo.Full)
	}

	// Initialize knowledge provider
	projectPath, err := os.Getwd()
	if err != nil {
		return errors.Wrap(err, "failed to get current directory")
	}
	noteProvider, knowledgeCleanup, err := getKnowledgeProvider(cfg, projectPath)
	if err != nil {
		return errors.Wrap(err, "failed to initialize knowledge provider")
	}
	defer knowledgeCleanup()

	// Get note path
	notePath, err := noteProvider.GetNotePath(cmd.Context(), ticketInfo.Type, ticketInfo.Full)
	if err != nil {
		return errors.Wrap(err, "failed to get note path")
	}

	// Check if note exists
	if _, err := os.Stat(notePath); os.IsNotExist(err) {
		fmt.Printf("Note not found: %s\n", notePath)
		fmt.Println("Use 'rig work' to create the note first.")
		return nil
	}

	var updated bool

	// Update JIRA information if requested or if it's a non-incident ticket
	if syncJira || ticketInfo.Type != "incident" {
		if cfg.Jira.Enabled {
			if verbose {
				fmt.Println("Refreshing JIRA information...")
			}

			jiraClient, err := jira.NewJiraClient(&cfg.Jira, verbose)
			if err != nil {
				if verbose {
					fmt.Printf("Warning: Could not initialize JIRA client: %v\n", err)
				}
			} else {
				jiraInfo, err := jiraClient.FetchTicketDetails(ticketInfo.Full)
				if err != nil {
					if verbose {
						fmt.Printf("Warning: Could not fetch JIRA details: %v\n", err)
					}
				} else {
					// Update note with fresh JIRA info
					err = updateNoteWithJiraInfo(notePath, jiraInfo)
					if err != nil {
						return errors.Wrap(err, "failed to update note with JIRA info")
					}
					fmt.Println("JIRA information updated")
					updated = true
				}
			}
		}
	}

	// Update daily note entry
	if verbose {
		fmt.Println("Updating daily note entry...")
	}

	err = noteProvider.UpdateDailyNote(cmd.Context(), ticketInfo.Full, ticketInfo.Type)
	if err != nil {
		if verbose {
			fmt.Printf("Warning: Could not update daily note: %v\n", err)
		}
	} else {
		fmt.Println("Daily note updated")
		updated = true
	}

	if !updated {
		fmt.Println("No updates were made.")
	} else {
		fmt.Printf("Sync completed for: %s\n", ticketInfo.Full)
	}

	return nil
}

func syncDailyNote(cmd *cobra.Command, cfg *config.Config) error {
	if verbose {
		fmt.Println("Syncing today's daily note...")
	}

	// Initialize knowledge provider
	projectPath, err := os.Getwd()
	if err != nil {
		return errors.Wrap(err, "failed to get current directory")
	}
	noteProvider, knowledgeCleanup, err := getKnowledgeProvider(cfg, projectPath)
	if err != nil {
		return errors.Wrap(err, "failed to initialize knowledge provider")
	}
	defer knowledgeCleanup()

	// For now, just verify the daily note exists
	dailyNotePath, err := noteProvider.GetDailyNotePath(cmd.Context())
	if err != nil {
		return errors.Wrap(err, "failed to get daily note path")
	}

	if _, err := os.Stat(dailyNotePath); os.IsNotExist(err) {
		fmt.Printf("Daily note not found: %s\n", dailyNotePath)
		fmt.Println("Create the daily note first or use 'rig work' to create a ticket.")
		return nil
	}

	fmt.Printf("Daily note exists: %s\n", dailyNotePath)
	fmt.Println("Daily note sync completed.")

	return nil
}

// updateNoteWithJiraInfo updates a note file with fresh JIRA information
func updateNoteWithJiraInfo(notePath string, jiraInfo *jira.TicketInfo) error {
	// Read existing content
	content, err := os.ReadFile(notePath)
	if err != nil {
		return errors.Wrap(err, "failed to read note")
	}

	noteContent := string(content)

	// Update the title if we have a summary
	if jiraInfo.Summary != "" {
		noteContent = updateNoteTitle(noteContent, jiraInfo.Summary)
	}

	// Update or add JIRA details section
	noteContent = updateJiraDetailsSection(noteContent, jiraInfo)

	// Write back to file with restricted permissions
	err = os.WriteFile(notePath, []byte(noteContent), 0600)
	if err != nil {
		return errors.Wrap(err, "failed to write updated note")
	}

	return nil
}

// updateNoteTitle updates the note title/heading with the JIRA summary
func updateNoteTitle(content, summary string) string {
	lines := strings.Split(content, "\n")

	for i, line := range lines {
		// Look for the main heading (starts with # )
		if strings.HasPrefix(line, "# ") {
			lines[i] = "# " + summary
			break
		}
	}

	return strings.Join(lines, "\n")
}

// updateJiraDetailsSection updates or creates the JIRA Details section
func updateJiraDetailsSection(content string, jiraInfo *jira.TicketInfo) string {
	lines := strings.Split(content, "\n")
	var result []string

	jiraSection := buildJiraDetailsSection(jiraInfo)
	jiraSectionFound := false
	inJiraSection := false

	// Use index-based loop so we can skip ahead when needed
	for i := 0; i < len(lines); i++ {
		line := lines[i]

		// Check if we're entering the JIRA Details section
		if strings.HasPrefix(line, "## JIRA Details") {
			jiraSectionFound = true
			inJiraSection = true
			// Add the section header and new content
			result = append(result, line)
			result = append(result, "")
			result = append(result, strings.Split(jiraSection, "\n")...)
			continue
		}

		// Check if we're leaving the JIRA Details section
		if inJiraSection && strings.HasPrefix(line, "## ") && !strings.HasPrefix(line, "## JIRA Details") {
			inJiraSection = false
		}

		// Skip lines that are part of the old JIRA section (except the header)
		if inJiraSection {
			continue
		}

		result = append(result, line)

		// If we haven't found a JIRA section and we're at the end of Summary section, insert it
		if !jiraSectionFound && strings.HasPrefix(line, "## Summary") {
			// Look ahead to find the end of the summary section
			j := i + 1
			for j < len(lines) && !strings.HasPrefix(lines[j], "## ") {
				result = append(result, lines[j])
				j++
			}

			// Insert JIRA section
			result = append(result, "")
			result = append(result, "## JIRA Details")
			result = append(result, "")
			result = append(result, strings.Split(jiraSection, "\n")...)
			result = append(result, "")

			// Skip the lines we already processed (now this actually works!)
			i = j - 1
			jiraSectionFound = true
		}
	}

	// If no JIRA section was found, append it at the end
	if !jiraSectionFound {
		result = append(result, "")
		result = append(result, "## JIRA Details")
		result = append(result, "")
		result = append(result, strings.Split(jiraSection, "\n")...)
	}

	return strings.Join(result, "\n")
}

// buildJiraDetailsSection builds the JIRA details section content
func buildJiraDetailsSection(jiraInfo *jira.TicketInfo) string {
	var section strings.Builder

	if jiraInfo.Type != "" {
		fmt.Fprintf(&section, "**Type:** %s\n", jiraInfo.Type)
	}

	if jiraInfo.Status != "" {
		fmt.Fprintf(&section, "**Status:** %s\n", jiraInfo.Status)
	}

	if jiraInfo.Priority != "" {
		fmt.Fprintf(&section, "**Priority:** %s\n", jiraInfo.Priority)
	}

	// Display custom fields if present
	if len(jiraInfo.CustomFields) > 0 {
		for fieldName, fieldValue := range jiraInfo.CustomFields {
			if fieldValue != "" {
				fmt.Fprintf(&section, "**%s:** %s\n", fieldName, fieldValue)
			}
		}
	}

	if jiraInfo.Description != "" {
		section.WriteString("\n**Description:**\n" + jiraInfo.Description)
	}

	return section.String()
}
