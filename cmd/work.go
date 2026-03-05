package cmd

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/spf13/cobra"

	"thoreinstein.com/rig/pkg/notes"
	"thoreinstein.com/rig/pkg/ticket"
	"thoreinstein.com/rig/pkg/tmux"
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
		return runWorkCommand(cmd.Context(), args[0])
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

func runWorkCommand(ctx context.Context, ticketID string) error {
	// Load configuration
	cfg, err := loadConfig()
	if err != nil {
		return errors.Wrap(err, "failed to load configuration")
	}

	// Parse ticket
	ticketInfo, err := parseTicket(ticketID)
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

	// Initialize VCS provider
	provider, cleanup, err := getVCSProvider(cfg)
	if err != nil {
		return errors.Wrap(err, "failed to initialize VCS provider")
	}
	defer cleanup()

	// Step 1: Create git worktree
	if verbose {
		fmt.Printf("Creating git worktree in %s...\n", repoPath)
	}

	// Get repo info for notes
	repoRoot, err := provider.GetRepoRoot(repoPath)
	if err != nil {
		return err
	}
	repoName, err := provider.GetRepoName(repoPath)
	if err != nil {
		return err
	}

	worktreePath, err := provider.CreateWorktree(repoPath, ticketInfo.Type, ticketInfo.ID, ticketInfo.ID, cfg.Git.BaseBranch)
	if err != nil {
		return errors.Wrap(err, "failed to create git worktree")
	}
	fmt.Printf("Git worktree created at: %s\n", worktreePath)

	// Step 2: Fetch Ticket details and update status
	var ticketDetails *ticket.TicketInfo
	ticketProvider, ticketCleanup, err := getTicketProvider(cfg, repoPath)
	if err != nil {
		if verbose {
			fmt.Printf("Warning: Could not initialize ticket provider: %v\n", err)
		}
	} else {
		defer ticketCleanup()
		if ticketProvider.IsAvailable(ctx) {
			if verbose {
				fmt.Println("Fetching ticket details...")
			}
			ticketDetails, err = ticketProvider.GetTicketInfo(ctx, ticketInfo.ID)
			if err != nil {
				if verbose {
					fmt.Printf("Warning: Could not fetch ticket details: %v\n", err)
				}
			} else {
				if verbose {
					fmt.Printf("Ticket details fetched: %s\n", ticketDetails.Title)
				}
				// Update status to in_progress
				if err := ticketProvider.UpdateStatus(ctx, ticketInfo.ID, "in_progress"); err != nil {
					if verbose {
						fmt.Printf("Warning: Could not update ticket status: %v\n", err)
					}
				} else {
					fmt.Println("Ticket status updated to in_progress")
				}
			}
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

		// Add issue info if available
		if ticketDetails != nil {
			noteData.Summary = ticketDetails.Title
			noteData.Status = ticketDetails.Status
			noteData.Description = ticketDetails.Description
		}

		result, err := noteManager.CreateTicketNote(noteData)
		if err != nil {
			// Don't fail if note creation fails — worktree is already created
			if verbose {
				fmt.Printf("Warning: Could not create note: %v\n", err)
			}
		} else {
			if result.Created {
				fmt.Printf("Note created at: %s\n", result.Path)
			} else {
				fmt.Printf("Opened existing note: %s\n", result.Path)
			}
			notePath = result.Path
		}
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
