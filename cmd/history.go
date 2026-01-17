package cmd

import (
	"fmt"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/spf13/cobra"

	"thoreinstein.com/rig/pkg/config"
	"thoreinstein.com/rig/pkg/history"
)

// historyCmd represents the history command
var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "Query and manage command history",
	Long: `Query and manage command history from the SQLite database.

This command provides subcommands to query the history database (zsh-histdb or atuin)
and get information about stored commands.`,
}

// historyQueryCmd queries the history database
var historyQueryCmd = &cobra.Command{
	Use:   "query [pattern]",
	Short: "Query command history",
	Long: `Query the command history database with optional filters.

Examples:
  rig history query                     # List recent commands
  rig history query "git"               # Search for commands containing "git"
  rig history query --since "2025-08-10"
  rig history query --directory /path/to/dir
  rig history query --failed-only
  rig history query --exit-code 1
  rig history query --min-duration 5s`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		pattern := ""
		if len(args) > 0 {
			pattern = args[0]
		}
		return runHistoryQueryCommand(pattern)
	},
}

// historyInfoCmd shows database information
var historyInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show history database information",
	Long:  `Display information about the history database including schema, size, and statistics.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runHistoryInfoCommand()
	},
}

var (
	historySince       string
	historyUntil       string
	historyDirectory   string
	historySession     string
	historySessionID   string
	historyFailedOnly  bool
	historyExitCode    int
	historyMinDuration time.Duration
	historyLimit       int
)

func init() {
	rootCmd.AddCommand(historyCmd)
	historyCmd.AddCommand(historyQueryCmd)
	historyCmd.AddCommand(historyInfoCmd)

	historyQueryCmd.Flags().StringVar(&historySince, "since", "", "Start time (YYYY-MM-DD HH:MM or YYYY-MM-DD)")
	historyQueryCmd.Flags().StringVar(&historyUntil, "until", "", "End time (YYYY-MM-DD HH:MM or YYYY-MM-DD)")
	historyQueryCmd.Flags().StringVar(&historyDirectory, "directory", "", "Filter by directory path")
	historyQueryCmd.Flags().StringVar(&historySession, "session", "", "Filter by session")
	historyQueryCmd.Flags().StringVar(&historySessionID, "session-id", "", "Filter by exact session ID")
	historyQueryCmd.Flags().BoolVar(&historyFailedOnly, "failed-only", false, "Show only failed commands")
	historyQueryCmd.Flags().IntVar(&historyExitCode, "exit-code", -1, "Filter by exact exit code")
	historyQueryCmd.Flags().DurationVar(&historyMinDuration, "min-duration", 0, "Filter by minimum duration (e.g. 5s, 1m)")
	historyQueryCmd.Flags().IntVar(&historyLimit, "limit", 50, "Maximum number of commands to show")
}

func runHistoryQueryCommand(pattern string) error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return errors.Wrap(err, "failed to load configuration")
	}

	// Initialize database manager
	dbManager := history.NewDatabaseManager(cfg.History.DatabasePath, verbose)

	if !dbManager.IsAvailable() {
		return errors.Newf("history database not available at: %s", cfg.History.DatabasePath)
	}

	// Parse time options
	var since, until *time.Time

	if historySince != "" {
		parsedSince, err := parseTimeString(historySince)
		if err != nil {
			return errors.Wrap(err, "invalid --since time")
		}
		since = &parsedSince
	}

	if historyUntil != "" {
		parsedUntil, err := parseTimeString(historyUntil)
		if err != nil {
			return errors.Wrap(err, "invalid --until time")
		}
		until = &parsedUntil
	}

	// Build query options
	options := history.QueryOptions{
		Since:       since,
		Until:       until,
		Directory:   historyDirectory,
		Session:     historySession,
		SessionID:   historySessionID,
		MinDuration: historyMinDuration,
		Pattern:     pattern,
		Limit:       historyLimit,
	}

	if historyExitCode != -1 {
		options.ExitCode = &historyExitCode
	} else if historyFailedOnly {
		failedExitCode := 1
		options.ExitCode = &failedExitCode
	}

	// Query commands
	commands, err := dbManager.QueryCommands(options)
	if err != nil {
		return errors.Wrap(err, "failed to query commands")
	}

	if len(commands) == 0 {
		fmt.Println("No commands found matching the criteria.")
		return nil
	}

	// Display results
	fmt.Printf("Found %d commands:\n\n", len(commands))

	for i, cmd := range commands {
		timestamp := cmd.Timestamp.Format("2006-01-02 15:04:05")

		var statusIcon string
		if cmd.ExitCode == 0 {
			statusIcon = "✓"
		} else {
			statusIcon = "✗"
		}

		var durationStr string
		if cmd.Duration > 0 {
			if cmd.Duration < 1000 {
				durationStr = fmt.Sprintf("%dms", cmd.Duration)
			} else {
				durationStr = fmt.Sprintf("%.1fs", float64(cmd.Duration)/1000.0)
			}
		}

		// Truncate command if too long
		command := cmd.Command
		if len(command) > 80 {
			command = command[:77] + "..."
		}

		// Truncate directory if too long
		directory := cmd.Directory
		if len(directory) > 30 {
			directory = "..." + directory[len(directory)-27:]
		}

		fmt.Printf("%3d. %s %s [%s] %s", i+1, statusIcon, timestamp, durationStr, command)

		if directory != "" {
			fmt.Printf("\n     Directory: %s", directory)
		}

		if cmd.Session != "" {
			fmt.Printf("\n     Session: %s", cmd.Session)
		}

		if cmd.ExitCode != 0 {
			fmt.Printf("\n     Exit Code: %d", cmd.ExitCode)
		}

		fmt.Println()

		// Add separator between commands
		if i < len(commands)-1 {
			fmt.Println()
		}
	}

	return nil
}

func runHistoryInfoCommand() error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return errors.Wrap(err, "failed to load configuration")
	}

	// Initialize database manager
	dbManager := history.NewDatabaseManager(cfg.History.DatabasePath, verbose)

	// Get database info
	info, err := dbManager.GetDatabaseInfo()
	if err != nil {
		return errors.Wrap(err, "failed to get database info")
	}

	fmt.Println("History Database Information")
	fmt.Println("============================")

	fmt.Printf("Path: %s\n", info["path"])
	fmt.Printf("Exists: %v\n", info["exists"])

	if !info["exists"].(bool) {
		fmt.Println("Database file does not exist.")
		fmt.Println("Make sure zsh-histdb or atuin is configured and running.")
		return nil
	}

	if size, ok := info["size"]; ok {
		fmt.Printf("Size: %d bytes\n", size)
	}

	if modified, ok := info["modified"]; ok {
		fmt.Printf("Modified: %s\n", modified.(time.Time).Format("2006-01-02 15:04:05"))
	}

	if schema, ok := info["schema"]; ok {
		fmt.Printf("Schema: %s\n", schema)
	}

	if count, ok := info["command_count"]; ok {
		fmt.Printf("Commands: %d\n", count)
	}

	if errMsg, ok := info["error"]; ok {
		fmt.Printf("Error: %s\n", errMsg)
	}

	// Test availability
	if dbManager.IsAvailable() {
		fmt.Println("Status: Available ✓")
	} else {
		fmt.Println("Status: Not available ✗")
	}

	return nil
}