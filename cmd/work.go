package cmd

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/spf13/cobra"

	"thoreinstein.com/rig/pkg/beads"
	"thoreinstein.com/rig/pkg/git"
	"thoreinstein.com/rig/pkg/jira"
	"thoreinstein.com/rig/pkg/notes"
	"thoreinstein.com/rig/pkg/tmux"
	"thoreinstein.com/rig/pkg/workflow"
)

var workNoNotes bool

// workCmd represents the work command
var workCmd = &cobra.Command{
	Use:   "work <ticket>",
	Short: "Start workflow for a ticket",
	Long: `Start the complete workflow for a given ticket.

This command performs the following actions:
- Parses ticket type and number
- Creates git worktree and branch
- Creates/updates markdown note with JIRA integration (use --no-notes to skip)
- Updates daily note with log entry
- Creates tmux session with configured windows

Examples:
  rig work proj-123
  rig work ops-456
  rig work incident-789 --no-notes`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runWorkCommand(args[0])
	},
}

func init() {
	rootCmd.AddCommand(workCmd)

	workCmd.Flags().BoolVar(&workNoNotes, "no-notes", false, "Skip creating markdown note and note-related tmux window commands")
	workCmd.Flags().StringVarP(&projectFlag, "project", "p", "", "Override project directory")
}

// TicketInfo holds parsed ticket information
type TicketInfo struct {
	Full    string // Original user input (e.g., "project:TYPE-ID")
	Project string // Optional project prefix (e.g., "project")
	ID      string // Clean ticket identifier (e.g., "TYPE-ID")
	Type    string // Ticket type, normalized to lowercase (e.g., "type")
	Number  string // Ticket number or alphanumeric identifier (e.g., "ID")
}

// SessionID returns a sanitized ticket identifier suitable for tmux session names
func (t *TicketInfo) SessionID() string {
	if t.Project != "" {
		return t.Project + "-" + t.ID
	}
	return t.ID
}

// parseTicket parses a ticket string into type and number/identifier components.
// Supports both traditional Jira-style tickets (proj-123) and beads-style tickets (rig-abc123).
// Also supports optional project prefix (project:ticket).
func parseTicket(ticket string) (*TicketInfo, error) {
	var project string
	fullInput := ticket

	// Check for optional project prefix
	if p, t, ok := strings.Cut(ticket, ":"); ok {
		if p == "" {
			return nil, errors.New("invalid ticket format. Project name cannot be empty when using ':'")
		}
		project = p
		ticket = t
	}

	// Match pattern: TYPE-ID where ID can be digits or alphanumeric (e.g., proj-123, rig-abc, beads-42f)
	re := regexp.MustCompile(`^([a-zA-Z]+)-([a-zA-Z0-9]+)$`)
	matches := re.FindStringSubmatch(ticket)

	if len(matches) != 3 {
		return nil, errors.New("invalid ticket format. Expected format: [project:]TYPE-ID (e.g., proj-123, rig:proj-123 or rig-abc)")
	}

	return &TicketInfo{
		Full:    fullInput,
		Project: project,
		ID:      ticket,
		Type:    strings.ToLower(matches[1]),
		Number:  matches[2],
	}, nil
}

func runWorkCommand(ticket string) error {
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
		fmt.Printf("Starting workflow for ticket: %s\n", ticketInfo.Full)
		if ticketInfo.Project != "" {
			fmt.Printf("  Project: %s\n", ticketInfo.Project)
		}
		fmt.Printf("  Type: %s\n", ticketInfo.Type)
		fmt.Printf("  Number: %s\n", ticketInfo.Number)
	}

	// Determine project context and switch to it
	repoPath, err := resolveProjectContext(cfg, projectFlag, ticketInfo.Project)
	if err != nil {
		return err
	}

	if verbose {
		fmt.Printf("Switching to project root: %s\n", repoPath)
	}
	if err := os.Chdir(repoPath); err != nil {
		return errors.Wrapf(err, "failed to chdir to %s", repoPath)
	}

	// Step 1: Create git worktree
	if verbose {
		fmt.Printf("Creating git worktree in %s...\n", repoPath)
	}
	gitManager := git.NewWorktreeManagerAtPath(repoPath, cfg.Git.BaseBranch, verbose)

	// Get repo info for notes
	repoRoot, err := gitManager.GetRepoRoot()
	if err != nil {
		return err
	}
	repoName, err := gitManager.GetRepoName()
	if err != nil {
		return err
	}

	worktreePath, err := gitManager.CreateWorktree(ticketInfo.Type, ticketInfo.ID)
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
		jiraClient, err := jira.NewJiraClient(&cfg.Jira, verbose)
		if err != nil {
			if verbose {
				fmt.Printf("Warning: Could not initialize JIRA client: %v\n", err)
			}
		} else {
			jiraInfo, err = jiraClient.FetchTicketDetails(ticketInfo.ID)
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

	// Step 2b: Update beads status (if beads project detected)
	var beadsInfo *beads.IssueInfo
	router := workflow.NewTicketRouter(cfg, worktreePath, verbose)
	ticketSource := router.RouteTicket(ticketInfo.ID)

	if ticketSource == workflow.TicketSourceBeads && cfg.Beads.Enabled {
		if verbose {
			fmt.Println("Detected beads project, updating issue status...")
		}
		beadsClient, err := beads.NewCLIClient(cfg.Beads.CliCommand, verbose)
		if err != nil {
			if verbose {
				fmt.Printf("Warning: Invalid beads CLI command: %v\n", err)
			}
		} else if beadsClient.IsAvailable() {
			// First, fetch issue details for note integration
			beadsInfo, err = beadsClient.Show(ticketInfo.ID)
			if err != nil {
				if verbose {
					fmt.Printf("Warning: Could not fetch beads issue details: %v\n", err)
				}
			}

			// Update status to in_progress
			if err := beadsClient.UpdateStatus(ticketInfo.ID, "in_progress"); err != nil {
				if verbose {
					fmt.Printf("Warning: Could not update beads status: %v\n", err)
				}
			} else {
				fmt.Println("Beads issue status updated to in_progress")
			}
		} else if verbose {
			fmt.Printf("Warning: beads CLI '%s' not found in PATH\n", cfg.Beads.CliCommand)
		}
	}

	// Step 3: Create/update note (unless --no-notes flag is set)
	noteManager := notes.NewManager(
		cfg.Notes.Path,
		cfg.Notes.DailyDir,
		cfg.Notes.TemplateDir,
		verbose,
	)

	var notePath string
	if !workNoNotes {
		if verbose {
			fmt.Println("Creating note...")
		}

		// Build ticket data for template
		noteData := notes.TicketData{
			Ticket:       ticketInfo.ID,
			TicketType:   ticketInfo.Type,
			RepoName:     repoName,
			RepoPath:     repoRoot,
			WorktreePath: worktreePath,
		}

		// Add issue info if available (beads takes precedence over JIRA)
		if beadsInfo != nil {
			noteData.Summary = beadsInfo.Title
			noteData.Status = beadsInfo.Status
			noteData.Description = beadsInfo.Description
		} else if jiraInfo != nil {
			noteData.Summary = jiraInfo.Summary
			noteData.Status = jiraInfo.Status
			noteData.Description = jiraInfo.Description
		}

		result, err := noteManager.CreateTicketNote(noteData)
		if err != nil {
			return errors.Wrap(err, "failed to create note")
		}
		if result.Created {
			fmt.Printf("Note created at: %s\n", result.Path)
		} else {
			fmt.Printf("Opened existing note: %s\n", result.Path)
		}
		notePath = result.Path
	}

	// Step 4: Update daily note
	if verbose {
		fmt.Println("Updating daily note...")
	}
	err = noteManager.UpdateDailyNote(ticketInfo.ID, ticketInfo.Type)
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

	// Use sanitized ticket for session name (no colons)
	sessionID := ticketInfo.SessionID()

	sessionManager := tmux.NewSessionManager(cfg.Tmux.SessionPrefix, tmuxWindows, verbose)
	err = sessionManager.CreateSession(sessionID, worktreePath, notePath)
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
	if notePath != "" {
		fmt.Printf("Note: %s\n", notePath)
	}

	return nil
}


