package cmd

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/spf13/cobra"
	"thoreinstein.com/rig/pkg/config"
	"thoreinstein.com/rig/pkg/git"
	"thoreinstein.com/rig/pkg/jira"
	"thoreinstein.com/rig/pkg/notes"
	"thoreinstein.com/rig/pkg/tmux"
)

// workCmd represents the work command
var workCmd = &cobra.Command{
	Use:   "work <ticket>",
	Short: "Start workflow for a ticket",
	Long: `Start the complete workflow for a given ticket.

This command performs the following actions:
- Parses ticket type and number
- Creates git worktree and branch
- Creates/updates markdown note with JIRA integration
- Updates daily note with log entry
- Creates tmux session with configured windows

Examples:
  rig work proj-123
  rig work ops-456
  rig work incident-789`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runWorkCommand(args[0])
	},
}

func init() {
	rootCmd.AddCommand(workCmd)
}

// TicketInfo holds parsed ticket information
type TicketInfo struct {
	Full   string
	Type   string
	Number string
}

// parseTicket parses a ticket string into type and number components
func parseTicket(ticket string) (*TicketInfo, error) {
	// Match pattern: TYPE-NUMBER (e.g., proj-123, ops-456)
	re := regexp.MustCompile(`^([a-zA-Z]+)-([0-9]+)$`)
	matches := re.FindStringSubmatch(ticket)

	if len(matches) != 3 {
		return nil, errors.New("invalid ticket format. Expected format: TYPE-NUMBER (e.g., proj-123)")
	}

	return &TicketInfo{
		Full:   ticket,
		Type:   strings.ToLower(matches[1]),
		Number: matches[2],
	}, nil
}

func runWorkCommand(ticket string) error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return errors.Wrap(err, "failed to load configuration")
	}

	// Parse ticket
	ticketInfo, err := parseTicket(ticket)
	if err != nil {
		return err
	}

	if verbose {
		fmt.Printf("Starting workflow for ticket: %s\n", ticketInfo.Full)
		fmt.Printf("  Type: %s\n", ticketInfo.Type)
		fmt.Printf("  Number: %s\n", ticketInfo.Number)
	}

	// Step 1: Create git worktree (uses CWD to find repo)
	if verbose {
		fmt.Println("Creating git worktree...")
	}
	gitManager := git.NewWorktreeManager(cfg.Git.BaseBranch, verbose)

	// Get repo info for notes
	repoRoot, err := gitManager.GetRepoRoot()
	if err != nil {
		return err
	}
	repoName, err := gitManager.GetRepoName()
	if err != nil {
		return err
	}

	worktreePath, err := gitManager.CreateWorktree(ticketInfo.Type, ticketInfo.Full)
	if err != nil {
		return errors.Wrap(err, "failed to create git worktree")
	}
	fmt.Printf("Git worktree created at: %s\n", worktreePath)

	// Step 2: Fetch JIRA details (if enabled)
	var jiraInfo *jira.TicketInfo
	if cfg.Jira.Enabled {
		if verbose {
			fmt.Println("Fetching JIRA details...")
		}
		jiraClient, err := jira.NewClient(cfg.Jira.CliCommand, verbose)
		if err != nil {
			if verbose {
				fmt.Printf("Warning: Invalid JIRA CLI command: %v\n", err)
			}
		} else {
			jiraInfo, err = jiraClient.FetchTicketDetails(ticketInfo.Full)
			if err != nil {
				if verbose {
					fmt.Printf("Warning: Could not fetch JIRA details: %v\n", err)
				}
				// Don't fail the entire process if JIRA fetch fails
				jiraInfo = nil
			} else {
				fmt.Println("JIRA details fetched successfully")
			}
		}
	}

	// Step 3: Create/update note
	if verbose {
		fmt.Println("Creating note...")
	}
	noteManager := notes.NewManager(
		cfg.Notes.Path,
		cfg.Notes.DailyDir,
		cfg.Notes.TemplateDir,
		verbose,
	)

	// Build ticket data for template
	noteData := notes.TicketData{
		Ticket:       ticketInfo.Full,
		TicketType:   ticketInfo.Type,
		RepoName:     repoName,
		RepoPath:     repoRoot,
		WorktreePath: worktreePath,
	}

	// Add JIRA info if available
	if jiraInfo != nil {
		noteData.Summary = jiraInfo.Summary
		noteData.Status = jiraInfo.Status
		noteData.Description = jiraInfo.Description
	}

	notePath, err := noteManager.CreateTicketNote(noteData)
	if err != nil {
		return errors.Wrap(err, "failed to create note")
	}
	fmt.Printf("Note created at: %s\n", notePath)

	// Step 4: Update daily note
	if verbose {
		fmt.Println("Updating daily note...")
	}
	err = noteManager.UpdateDailyNote(ticketInfo.Full, ticketInfo.Type)
	if err != nil {
		// Don't fail if daily note update fails
		if verbose {
			fmt.Printf("Warning: Could not update daily note: %v\n", err)
		}
	} else {
		fmt.Println("Daily note updated")
	}

	// Step 5: Create tmux session
	if verbose {
		fmt.Println("Creating tmux session...")
	}

	// Convert config windows to tmux windows
	tmuxWindows := make([]tmux.WindowConfig, 0, len(cfg.Tmux.Windows))
	for _, window := range cfg.Tmux.Windows {
		tmuxWindows = append(tmuxWindows, tmux.WindowConfig{
			Name:       window.Name,
			Command:    window.Command,
			WorkingDir: window.WorkingDir,
		})
	}

	sessionManager := tmux.NewSessionManager(cfg.Tmux.SessionPrefix, tmuxWindows, verbose)
	err = sessionManager.CreateSession(ticketInfo.Full, worktreePath, notePath)
	if err != nil {
		// Don't fail the entire process if tmux session creation fails
		if verbose {
			fmt.Printf("Warning: Could not create tmux session: %v\n", err)
		}
		fmt.Println("Warning: Tmux session creation failed, but other steps completed successfully")
	} else {
		fmt.Println("Tmux session created successfully")
	}

	fmt.Printf("\nWorkflow initialization for %s completed successfully!\n", ticketInfo.Full)
	fmt.Printf("Worktree: %s\n", worktreePath)
	fmt.Printf("Note: %s\n", notePath)

	return nil
}
