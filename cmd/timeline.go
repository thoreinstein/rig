package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"thoreinstein.com/sre/pkg/config"
	"thoreinstein.com/sre/pkg/history"
)

// timelineCmd represents the timeline command
var timelineCmd = &cobra.Command{
	Use:   "timeline <ticket>",
	Short: "Generate command timeline for a ticket",
	Long: `Generate a timeline of commands executed for a specific ticket and export to Obsidian.

This command queries the history database (zsh-histdb or atuin) to find commands
related to the specified ticket and generates a formatted timeline that can be
inserted into the ticket's Obsidian note.

Examples:
  sre timeline fraas-25857
  sre timeline fraas-25857 --since "2025-08-10 09:00"
  sre timeline fraas-25857 --until "2025-08-10 18:00" 
  sre timeline fraas-25857 --failed-only
  sre timeline fraas-25857 --directory /path/to/worktree`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTimelineCommand(args[0])
	},
}

var (
	timelineSince     string
	timelineUntil     string
	timelineDirectory string
	timelineFailedOnly bool
	timelineLimit     int
	timelineOutput    string
	timelineNoUpdate  bool
)

func init() {
	rootCmd.AddCommand(timelineCmd)
	
	timelineCmd.Flags().StringVar(&timelineSince, "since", "", "Start time (YYYY-MM-DD HH:MM or YYYY-MM-DD)")
	timelineCmd.Flags().StringVar(&timelineUntil, "until", "", "End time (YYYY-MM-DD HH:MM or YYYY-MM-DD)")
	timelineCmd.Flags().StringVar(&timelineDirectory, "directory", "", "Filter by directory path")
	timelineCmd.Flags().BoolVar(&timelineFailedOnly, "failed-only", false, "Show only failed commands (exit code != 0)")
	timelineCmd.Flags().IntVar(&timelineLimit, "limit", 1000, "Maximum number of commands to retrieve")
	timelineCmd.Flags().StringVar(&timelineOutput, "output", "", "Output file path (default: update ticket note)")
	timelineCmd.Flags().BoolVar(&timelineNoUpdate, "no-update", false, "Don't update the ticket note, only output to console")
}

func runTimelineCommand(ticket string) error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}
	
	// Parse ticket
	ticketInfo, err := parseTicket(ticket)
	if err != nil {
		return err
	}
	
	if verbose {
		fmt.Printf("Generating timeline for ticket: %s\n", ticketInfo.Full)
	}
	
	// Initialize history database manager
	dbManager := history.NewDatabaseManager(cfg.History.DatabasePath, verbose)
	
	if !dbManager.IsAvailable() {
		return fmt.Errorf("history database not available at: %s", cfg.History.DatabasePath)
	}
	
	// Parse time options
	var since, until *time.Time
	
	if timelineSince != "" {
		parsedSince, err := parseTimeString(timelineSince)
		if err != nil {
			return fmt.Errorf("invalid --since time: %w", err)
		}
		since = &parsedSince
	}
	
	if timelineUntil != "" {
		parsedUntil, err := parseTimeString(timelineUntil)
		if err != nil {
			return fmt.Errorf("invalid --until time: %w", err)
		}
		until = &parsedUntil
	}
	
	// Build query options
	options := history.QueryOptions{
		Since:     since,
		Until:     until,
		Directory: timelineDirectory,
		Ticket:    ticketInfo.Full,
		Limit:     timelineLimit,
	}
	
	if timelineFailedOnly {
		failedExitCode := 1
		options.ExitCode = &failedExitCode
	}
	
	// Query commands
	if verbose {
		fmt.Println("Querying command history...")
	}
	
	commands, err := dbManager.QueryCommands(options)
	if err != nil {
		return fmt.Errorf("failed to query commands: %w", err)
	}
	
	if len(commands) == 0 {
		fmt.Printf("No commands found for ticket: %s\n", ticketInfo.Full)
		return nil
	}
	
	if verbose {
		fmt.Printf("Found %d commands\n", len(commands))
	}
	
	// Generate timeline markdown
	timeline := generateTimelineMarkdown(commands, ticketInfo.Full)
	
	// Output timeline
	if timelineNoUpdate {
		fmt.Println(timeline)
		return nil
	}
	
	if timelineOutput != "" {
		// Write to specified file
		err = writeTimelineToFile(timeline, timelineOutput)
		if err != nil {
			return fmt.Errorf("failed to write timeline to file: %w", err)
		}
		fmt.Printf("Timeline written to: %s\n", timelineOutput)
	} else {
		// Update ticket note
		err = updateTicketNoteWithTimeline(cfg, ticketInfo, timeline)
		if err != nil {
			return fmt.Errorf("failed to update ticket note: %w", err)
		}
		fmt.Printf("Timeline added to ticket note for: %s\n", ticketInfo.Full)
	}
	
	return nil
}

// parseTimeString parses various time string formats
func parseTimeString(timeStr string) (time.Time, error) {
	formats := []string{
		"2006-01-02 15:04",
		"2006-01-02 15:04:05",
		"2006-01-02",
		time.RFC3339,
	}
	
	for _, format := range formats {
		if t, err := time.Parse(format, timeStr); err == nil {
			return t, nil
		}
	}
	
	return time.Time{}, fmt.Errorf("unable to parse time: %s", timeStr)
}

// generateTimelineMarkdown generates a markdown timeline from commands
func generateTimelineMarkdown(commands []history.Command, ticket string) string {
	var timeline strings.Builder
	
	timeline.WriteString(fmt.Sprintf("## Command Timeline - %s\n\n", ticket))
	timeline.WriteString(fmt.Sprintf("Generated: %s\n", time.Now().Format("2006-01-02 15:04:05")))
	timeline.WriteString(fmt.Sprintf("Commands: %d\n\n", len(commands)))
	
	// Group commands by day
	dayGroups := make(map[string][]history.Command)
	
	for _, cmd := range commands {
		day := cmd.Timestamp.Format("2006-01-02")
		dayGroups[day] = append(dayGroups[day], cmd)
	}
	
	// Sort days and output
	var days []string
	for day := range dayGroups {
		days = append(days, day)
	}
	sort.Strings(days)
	
	for _, day := range days {
		dayCommands := dayGroups[day]
		timeline.WriteString(fmt.Sprintf("### %s\n\n", day))
		
		for _, cmd := range dayCommands {
			// Format timestamp
			timeStr := cmd.Timestamp.Format("15:04:05")
			
			// Format duration if available
			var durationStr string
			if cmd.Duration > 0 {
				durationStr = fmt.Sprintf(" (%dms)", cmd.Duration)
			}
			
			// Format exit code
			var exitStr string
			if cmd.ExitCode != 0 {
				exitStr = fmt.Sprintf(" [Exit: %d]", cmd.ExitCode)
			}
			
			// Format directory (show only basename if it's long)
			var dirStr string
			if cmd.Directory != "" {
				if len(cmd.Directory) > 50 {
					dirStr = fmt.Sprintf(" `.../%s`", cmd.Directory[len(cmd.Directory)-30:])
				} else {
					dirStr = fmt.Sprintf(" `%s`", cmd.Directory)
				}
			}
			
			timeline.WriteString(fmt.Sprintf("- **%s**%s%s%s: `%s`\n", 
				timeStr, durationStr, exitStr, dirStr, cmd.Command))
		}
		
		timeline.WriteString("\n")
	}
	
	return timeline.String()
}

// writeTimelineToFile writes the timeline to a specified file
func writeTimelineToFile(timeline, filename string) error {
	return os.WriteFile(filename, []byte(timeline), 0644)
}

// updateTicketNoteWithTimeline updates the ticket's Obsidian note with the timeline
func updateTicketNoteWithTimeline(cfg *config.Config, ticketInfo *TicketInfo, timeline string) error {
	// Get note path
	notePath := cfg.GetNotePath(ticketInfo.Type, ticketInfo.Full)
	
	// Check if note exists
	if _, err := os.Stat(notePath); os.IsNotExist(err) {
		return fmt.Errorf("ticket note not found: %s", notePath)
	}
	
	// Read existing note content
	content, err := os.ReadFile(notePath)
	if err != nil {
		return fmt.Errorf("failed to read note: %w", err)
	}
	
	noteContent := string(content)
	
	// Remove existing timeline section if present
	noteContent = removeExistingTimeline(noteContent)
	
	// Add new timeline at the end
	updatedContent := noteContent + "\n" + timeline
	
	// Write back to file
	err = os.WriteFile(notePath, []byte(updatedContent), 0644)
	if err != nil {
		return fmt.Errorf("failed to write updated note: %w", err)
	}
	
	return nil
}

// removeExistingTimeline removes any existing timeline section from the note
func removeExistingTimeline(content string) string {
	lines := strings.Split(content, "\n")
	var result []string
	inTimelineSection := false

	for _, line := range lines {
		// Check if this is a timeline section header
		if strings.HasPrefix(line, "## Command Timeline") {
			inTimelineSection = true
			continue
		}

		// Check if we've reached another section (ends the timeline section)
		if inTimelineSection && strings.HasPrefix(line, "## ") {
			inTimelineSection = false
			// Fall through to include this line (the new section header)
		}

		// Include line if we're not in the timeline section
		if !inTimelineSection {
			result = append(result, line)
		}
	}

	// Note: If timeline was the last section, we simply don't include it.
	// Content after it (if any) that isn't a section header is also skipped,
	// but that's the expected behavior - it was part of the timeline section.

	return strings.Join(result, "\n")
}