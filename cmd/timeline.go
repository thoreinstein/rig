package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/spf13/cobra"

	"thoreinstein.com/rig/pkg/config"
	"thoreinstein.com/rig/pkg/events"
	"thoreinstein.com/rig/pkg/git"
	"thoreinstein.com/rig/pkg/history"
	"thoreinstein.com/rig/pkg/notes"
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
  rig timeline proj-123
  rig timeline proj-123 --since "2025-08-10 09:00"
  rig timeline proj-123 --until "2025-08-10 18:00"
  rig timeline proj-123 --failed-only
  rig timeline proj-123 --directory /path/to/worktree
  rig timeline proj-123 --min-duration 5s`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTimelineCommand(cmd.Context(), args[0])
	},
}

var (
	timelineSince       string
	timelineUntil       string
	timelineDirectory   string
	timelineSessionID   string
	timelineFailedOnly  bool
	timelineExitCode    int
	timelineMinDuration time.Duration
	timelineLimit       int
	timelineOutput      string
	timelineNoUpdate    bool
	timelineShowDiffs   bool
)

func init() {
	rootCmd.AddCommand(timelineCmd)

	timelineCmd.Flags().StringVar(&timelineSince, "since", "", "Start time (YYYY-MM-DD HH:MM or YYYY-MM-DD)")
	timelineCmd.Flags().StringVar(&timelineUntil, "until", "", "End time (YYYY-MM-DD HH:MM or YYYY-MM-DD)")
	timelineCmd.Flags().StringVar(&timelineDirectory, "directory", "", "Filter by directory path")
	timelineCmd.Flags().StringVar(&timelineSessionID, "session-id", "", "Filter by exact session ID")
	timelineCmd.Flags().BoolVar(&timelineFailedOnly, "failed-only", false, "Show only failed commands (exit code != 0)")
	timelineCmd.Flags().IntVar(&timelineExitCode, "exit-code", -1, "Filter by exact exit code")
	timelineCmd.Flags().DurationVar(&timelineMinDuration, "min-duration", 0, "Filter by minimum duration (e.g. 5s, 1m)")
	timelineCmd.Flags().IntVar(&timelineLimit, "limit", 1000, "Maximum number of commands to retrieve")
	timelineCmd.Flags().StringVar(&timelineOutput, "output", "", "Output file path (default: update ticket note)")
	timelineCmd.Flags().BoolVar(&timelineNoUpdate, "no-update", false, "Don't update the ticket note, only output to console")
	timelineCmd.Flags().BoolVar(&timelineShowDiffs, "show-diffs", false, "Show Dolt diffs between workflow checkpoints")
}

func runTimelineCommand(ctx context.Context, ticket string) error {
	// Load configuration
	cfg, err := loadConfig()
	if err != nil {
		return errors.Wrap(err, "failed to load configuration")
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
		return errors.Newf("history database not available at: %s", cfg.History.DatabasePath)
	}

	// Parse time options
	var since, until *time.Time

	if timelineSince != "" {
		parsedSince, err := parseTimeString(timelineSince)
		if err != nil {
			return errors.Wrap(err, "invalid --since time")
		}
		since = &parsedSince
	}

	if timelineUntil != "" {
		parsedUntil, err := parseTimeString(timelineUntil)
		if err != nil {
			return errors.Wrap(err, "invalid --until time")
		}
		until = &parsedUntil
	}

	// Get worktree path to include directory-based commands
	var projectPaths []string

	// If explicit directory provided, use it as strict filter (Directory field)
	// If NOT provided, use worktree path as an OR condition (ProjectPaths)
	if timelineDirectory == "" {
		// Attempt to resolve worktree path
		gitManager := git.NewWorktreeManager(cfg.Git.BaseBranch, verbose)
		worktreePath, err := gitManager.GetWorktreePath(ticketInfo.Type, ticketInfo.Full)
		if err == nil {
			projectPaths = append(projectPaths, worktreePath)
			if verbose {
				fmt.Printf("Including commands from worktree: %s\n", worktreePath)
			}
		} else if verbose {
			fmt.Printf("Warning: Could not resolve worktree path: %v\n", err)
		}
	}

	// Build query options
	options := history.QueryOptions{
		Since:        since,
		Until:        until,
		Directory:    timelineDirectory, // AND filter
		Ticket:       ticketInfo.Full,
		SessionID:    timelineSessionID,
		MinDuration:  timelineMinDuration,
		Limit:        timelineLimit,
		ProjectPaths: projectPaths, // OR logic with Ticket
	}

	if timelineExitCode != -1 {
		options.ExitCode = &timelineExitCode
	} else if timelineFailedOnly {
		failedExitCode := 1
		options.ExitCode = &failedExitCode
	}

	// Query commands
	if verbose {
		fmt.Println("Querying command history...")
	}

	commands, err := dbManager.QueryCommands(options)
	if err != nil {
		return errors.Wrap(err, "failed to query commands")
	}

	// Prepare unified entries
	var unifiedEntries []history.UnifiedEntry
	for _, cmd := range commands {
		unifiedEntries = append(unifiedEntries, history.UnifiedEntryFromCommand(cmd))
	}

	// Fetch workflow events if enabled
	var workflowEvents []events.WorkflowEvent
	var edm *events.DatabaseManager
	if cfg.Events.Enabled {
		if verbose {
			fmt.Println("Querying workflow events from Dolt...")
		}
		var edmErr error
		edm, edmErr = events.NewDatabaseManager(cfg.Events.DataPath, cfg.Events.CommitName, cfg.Events.CommitEmail, verbose)
		if edmErr != nil {
			fmt.Fprintf(os.Stderr, "Note: workflow events unavailable (use --verbose for details)\n")
			if verbose {
				fmt.Fprintf(os.Stderr, "Warning: failed to initialize events database: %v\n", edmErr)
			}
		} else {
			defer edm.Close()
			if initErr := edm.InitDatabase(); initErr != nil {
				fmt.Fprintf(os.Stderr, "Note: workflow events unavailable (use --verbose for details)\n")
				if verbose {
					fmt.Fprintf(os.Stderr, "Warning: failed to initialize events database: %v\n", initErr)
				}
				edm = nil // Prevent use of uninitialized database
			} else {
				evs, queryErr := edm.QueryEventsByTicket(ctx, ticketInfo.Full)
				if queryErr != nil {
					fmt.Fprintf(os.Stderr, "Note: workflow events unavailable (use --verbose for details)\n")
					if verbose {
						fmt.Fprintf(os.Stderr, "Warning: failed to query workflow events: %v\n", queryErr)
					}
				} else {
					workflowEvents = evs
					for _, e := range evs {
						unifiedEntries = append(unifiedEntries, history.UnifiedEntryFromEvent(e.CreatedAt, e.Step, e.Status, e.Message, e.CorrelationID))
					}
				}
			}
		}
	}

	if len(unifiedEntries) == 0 {
		fmt.Printf("No history found for ticket: %s\n", ticketInfo.Full)
		return nil
	}

	if verbose {
		fmt.Printf("Found %d total entries (%d commands, %d events)\n", len(unifiedEntries), len(commands), len(workflowEvents))
	}

	// Generate timeline markdown
	var timeline string
	if len(workflowEvents) > 0 {
		timeline = history.FormatUnifiedTimeline(unifiedEntries, ticketInfo.Full)
	} else {
		timeline = history.FormatTimeline(commands, ticketInfo.Full)
	}

	// Append diffs if requested
	if timelineShowDiffs && len(workflowEvents) > 0 && edm != nil {
		diffSummary, err := generateDiffSummary(ctx, edm, workflowEvents, verbose)
		if err != nil {
			if verbose {
				fmt.Fprintf(os.Stderr, "Warning: failed to generate diff summary: %v\n", err)
			}
		} else if diffSummary != "" {
			timeline += "\n" + diffSummary
		}
	}

	// Output timeline
	if timelineNoUpdate {
		fmt.Println(timeline)
		return nil
	}

	if timelineOutput != "" {
		// Validate output path before writing
		if err := validateOutputPath(timelineOutput); err != nil {
			return errors.Wrap(err, "invalid output path")
		}

		// Write to specified file
		err = writeTimelineToFile(timeline, timelineOutput)
		if err != nil {
			return errors.Wrap(err, "failed to write timeline to file")
		}
		fmt.Printf("Timeline written to: %s\n", timelineOutput)
	} else {
		// Update ticket note
		err = updateTicketNoteWithTimeline(cfg, ticketInfo, timeline)
		if err != nil {
			return errors.Wrap(err, "failed to update ticket note")
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

	return time.Time{}, errors.Newf("unable to parse time: %s", timeStr)
}

// validateOutputPath validates that the output path is safe to write to.
// It ensures the path:
// - Does not contain path traversal sequences
// - Is not an absolute path outside the user's home directory or temp directory
// - Does not point to sensitive system files
// - Has a safe file extension
func validateOutputPath(path string) error {
	if path == "" {
		return errors.New("output path cannot be empty")
	}

	// Clean the path and check for traversal
	cleanPath := filepath.Clean(path)

	// Reject paths containing .. after cleaning (prevents traversal)
	if strings.Contains(cleanPath, "..") {
		return errors.New("output path cannot contain path traversal sequences")
	}

	// If absolute path, validate it's in a safe location
	if filepath.IsAbs(cleanPath) {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return errors.Wrap(err, "cannot determine home directory")
		}

		// Allow paths within home directory or temp directory
		tempDir := os.TempDir()
		if !strings.HasPrefix(cleanPath, homeDir) && !strings.HasPrefix(cleanPath, tempDir) {
			return errors.Newf("absolute output path must be within home directory (%s) or temp directory (%s)", homeDir, tempDir)
		}
	}

	// Check for sensitive file patterns
	base := filepath.Base(cleanPath)
	sensitivePatterns := []string{
		".ssh",
		".gnupg",
		".bashrc",
		".zshrc",
		".profile",
		"authorized_keys",
		"known_hosts",
		"id_rsa",
		"id_ed25519",
		".env",
		"credentials",
	}

	lowerBase := strings.ToLower(base)
	for _, pattern := range sensitivePatterns {
		if strings.Contains(lowerBase, pattern) {
			return errors.Newf("output path cannot target sensitive file: %s", base)
		}
	}

	// Enforce safe file extensions
	ext := strings.ToLower(filepath.Ext(cleanPath))
	allowedExtensions := map[string]bool{
		".md":   true,
		".txt":  true,
		".json": true,
		"":      true, // Allow no extension
	}

	if !allowedExtensions[ext] {
		return errors.Newf("output file must have a safe extension (.md, .txt, .json), got: %s", ext)
	}

	return nil
}

// writeTimelineToFile writes the timeline to a specified file
func writeTimelineToFile(timeline, filename string) error {
	// Use restricted permissions as timeline may contain sensitive command history
	return os.WriteFile(filename, []byte(timeline), 0600)
}

// updateTicketNoteWithTimeline updates the ticket's note with the timeline
func updateTicketNoteWithTimeline(cfg *config.Config, ticketInfo *TicketInfo, timeline string) error {
	// Get note path using notes manager
	notesMgr := notes.NewManager(cfg.Notes.Path, cfg.Notes.DailyDir, cfg.Notes.TemplateDir, verbose)
	notePath := notesMgr.GetNotePath(ticketInfo.Type, ticketInfo.Full)

	// Check if note exists
	if _, err := os.Stat(notePath); os.IsNotExist(err) {
		return errors.Newf("ticket note not found: %s", notePath)
	}

	// Read existing note content
	content, err := os.ReadFile(notePath)
	if err != nil {
		return errors.Wrap(err, "failed to read note")
	}

	noteContent := string(content)

	// Remove existing timeline section if present
	noteContent = removeExistingTimeline(noteContent)

	// Add new timeline at the end
	updatedContent := noteContent + "\n" + timeline

	// Write back to file with restricted permissions (may contain command history)
	err = os.WriteFile(notePath, []byte(updatedContent), 0600)
	if err != nil {
		return errors.Wrap(err, "failed to write updated note")
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
		if strings.HasPrefix(line, "## Command Timeline") || strings.HasPrefix(line, "## Workflow Timeline") {
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

// generateDiffSummary queries Dolt diffs for all unique correlation IDs.
// It reuses the provided DatabaseManager rather than opening a second connection.
func generateDiffSummary(ctx context.Context, edm *events.DatabaseManager, workflowEvents []events.WorkflowEvent, verbose bool) (string, error) {
	// Find unique correlation IDs
	cids := make(map[string]bool)
	for _, e := range workflowEvents {
		if e.CorrelationID != "" {
			cids[e.CorrelationID] = true
		}
	}

	var sb strings.Builder
	sb.WriteString("## Workflow Checkpoint Diffs\n\n")

	foundAny := false
	for cid := range cids {
		diffs, err := edm.QueryDiffForCorrelation(ctx, cid)
		if err != nil {
			if verbose {
				fmt.Fprintf(os.Stderr, "Warning: failed to query diff for %s: %v\n", cid, err)
			}
			continue
		}

		if len(diffs) > 0 {
			foundAny = true
			sb.WriteString(fmt.Sprintf("### Correlation: %s\n\n", cid))
			for _, d := range diffs {
				sb.WriteString(fmt.Sprintf("- `%s` **[%s]** %s: %s\n",
					d.DiffType, d.Step, d.Status, d.Message))
			}
			sb.WriteString("\n")
		}
	}

	if !foundAny {
		return "", nil
	}

	return sb.String(), nil
}
