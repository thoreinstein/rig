package cmd

import (
	"fmt"
	"os"

	"github.com/cockroachdb/errors"
	"github.com/spf13/cobra"

	"thoreinstein.com/rig/pkg/knowledge"
	"thoreinstein.com/rig/pkg/ticket"
	"thoreinstein.com/rig/pkg/tmux"
)

var workNoNotes bool

// workCmd represents the work command
var workCmd = &cobra.Command{
	Use:   "work <ticket>",
	Short: "Start workflow for a ticket",
	Long: `Start the complete workflow for a given ticket.

This command automates the entire setup process for a new ticket:
- Creates git worktree at {repo}/{ticket_type}/{ticket_id}
- Creates branch for the ticket
- Updates JIRA status to In Progress (if configured)
- Creates markdown note (use --no-notes to skip)
- Updates daily note with log entry
- Creates tmux session with predefined windows

Examples:
  rig work PROJ-123
  rig work PROJ-123 --no-notes
  rig work PROJ-123 -p /path/to/project`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runWorkCommand(cmd, args[0])
	},
}

func init() {
	rootCmd.AddCommand(workCmd)

	workCmd.Flags().BoolVar(&workNoNotes, "no-notes", false, "Skip creating markdown note and note-related tmux window commands")
	workCmd.Flags().StringVarP(&projectFlag, "project", "p", "", "Override project directory")
}

func runWorkCommand(cmd *cobra.Command, ticketID string) error {
	ctx := cmd.Context()

	// Load configuration
	cfg, err := loadConfig()
	if err != nil {
		return errors.Wrap(err, "failed to load configuration")
	}

	// Parse ticket
	ticketInfo, err := ticket.ParseTicket(ticketID)
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
	var ticketDetails *knowledge.NoteData
	ticketProvider, ticketCleanup, err := getTicketProvider(cfg, repoPath)
	if err != nil {
		if verbose {
			fmt.Printf("Warning: Could not initialize ticket provider: %v\n", err)
		}
	} else {
		defer ticketCleanup()

		// Fetch ticket details (best-effort, used for note enrichment)
		if verbose {
			fmt.Println("Fetching ticket details...")
		}
		info, err := ticketProvider.GetTicketInfo(ctx, ticketInfo.ID)
		if err != nil {
			if verbose {
				fmt.Printf("Warning: Could not fetch ticket details: %v\n", err)
			}
		} else {
			if verbose {
				fmt.Printf("Ticket details fetched: %s\n", info.Title)
			}
			ticketDetails = &knowledge.NoteData{
				Summary:     info.Title,
				Status:      info.Status,
				Description: info.Description,
			}
		}

		// Update status independently — don't gate on GetTicketInfo success
		if err := ticketProvider.UpdateStatus(ctx, ticketInfo.ID, "in_progress"); err != nil {
			if verbose {
				fmt.Printf("Warning: Could not update ticket status: %v\n", err)
			}
		} else {
			fmt.Println("Ticket status updated to in_progress")
		}
	}

	// Step 3: Create/update note (unless --no-notes flag is set)
	var notePath string
	noteProvider, knowledgeCleanup, err := getKnowledgeProvider(cfg, repoPath)
	if err != nil {
		fmt.Printf("Warning: Could not initialize knowledge provider: %v\n", err)
	} else {
		defer knowledgeCleanup()

		if !workNoNotes {
			if verbose {
				fmt.Println("Creating note...")
			}

			// Build ticket data for template
			noteData := knowledge.NoteData{
				Ticket:       ticketInfo.ID,
				TicketType:   ticketInfo.Type,
				RepoName:     repoName,
				RepoPath:     repoRoot,
				WorktreePath: worktreePath,
			}

			// Add issue info if available
			if ticketDetails != nil {
				noteData.Summary = ticketDetails.Summary
				noteData.Status = ticketDetails.Status
				noteData.Description = ticketDetails.Description
			}

			result, err := noteProvider.CreateTicketNote(ctx, &noteData)
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
		err = noteProvider.UpdateDailyNote(ctx, ticketInfo.ID, ticketInfo.Type)
		if err != nil {
			// Don't fail if daily note update fails
			if verbose {
				fmt.Printf("Warning: Could not update daily note: %v\n", err)
			}
		} else {
			fmt.Println("Daily note updated")
		}
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
