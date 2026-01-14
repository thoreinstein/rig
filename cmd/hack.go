package cmd

import (
	"fmt"
	"regexp"

	"github.com/cockroachdb/errors"
	"github.com/spf13/cobra"

	"thoreinstein.com/rig/pkg/config"
	"thoreinstein.com/rig/pkg/git"
	"thoreinstein.com/rig/pkg/notes"
	"thoreinstein.com/rig/pkg/tmux"
)

var hackNoNotes bool

// hackCmd represents the hack command
var hackCmd = &cobra.Command{
	Use:   "hack <name>",
	Short: "Initialize a hack worktree for non-ticket work",
	Long: `Initialize a hack worktree for exploratory or non-ticket work.

This command creates a simplified workflow without JIRA integration:
- Creates git worktree at {repo}/hack/{name}
- Creates branch {name}
- Creates markdown note (use --no-notes to skip)
- Updates daily note with log entry
- Creates tmux session

Examples:
  rig hack winter-2025
  rig hack experiment-auth --no-notes`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runHackCommand(args[0])
	},
}

func init() {
	rootCmd.AddCommand(hackCmd)

	hackCmd.Flags().BoolVar(&hackNoNotes, "no-notes", false, "Skip creating markdown note and note-related tmux window commands")
}

// hackNameRegex validates hack names: must start with letter, contain only alphanumeric/hyphen/underscore, max 64 chars
var hackNameRegex = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]{0,63}$`)

// validateHackName validates the hack name parameter to prevent path traversal and injection attacks
func validateHackName(name string) error {
	if name == "" {
		return errors.New("hack name cannot be empty")
	}
	if !hackNameRegex.MatchString(name) {
		return errors.New("invalid hack name: must start with a letter and contain only alphanumeric characters, hyphens, and underscores (max 64 characters)")
	}
	return nil
}

func runHackCommand(name string) error {
	// Validate hack name first
	if err := validateHackName(name); err != nil {
		return err
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return errors.Wrap(err, "failed to load configuration")
	}

	if verbose {
		fmt.Printf("Starting hack workflow for: %s\n", name)
		fmt.Printf("  No-notes: %v\n", hackNoNotes)
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

	// For hacks, use "hack" as the type directory
	worktreePath, err := gitManager.CreateWorktreeWithBranch("hack", name, name)
	if err != nil {
		return errors.Wrap(err, "failed to create git worktree")
	}
	fmt.Printf("Git worktree created at: %s\n", worktreePath)

	// Step 2: Create note (unless --no-notes flag is set)
	noteManager := notes.NewManager(
		cfg.Notes.Path,
		cfg.Notes.DailyDir,
		cfg.Notes.TemplateDir,
		verbose,
	)

	var notePath string
	if !hackNoNotes {
		if verbose {
			fmt.Println("Creating note...")
		}

		noteData := notes.TicketData{
			Ticket:       name,
			TicketType:   "hack",
			RepoName:     repoName,
			RepoPath:     repoRoot,
			WorktreePath: worktreePath,
		}

		notePath, err = noteManager.CreateTicketNote(noteData)
		if err != nil {
			// Don't fail if note creation fails
			if verbose {
				fmt.Printf("Warning: Could not create note: %v\n", err)
			}
		} else {
			fmt.Printf("Note created at: %s\n", notePath)
		}
	}

	// Step 3: Update daily note (always, regardless of --no-notes)
	if verbose {
		fmt.Println("Updating daily note...")
	}
	err = noteManager.UpdateDailyNote(name, "hack")
	if err != nil {
		// Don't fail if daily note update fails
		if verbose {
			fmt.Printf("Warning: Could not update daily note: %v\n", err)
		}
	} else {
		fmt.Println("Daily note updated")
	}

	// Step 4: Create tmux session
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
	err = sessionManager.CreateSession(name, worktreePath, notePath)
	if err != nil {
		// Don't fail the entire process if tmux session creation fails
		if verbose {
			fmt.Printf("Warning: Could not create tmux session: %v\n", err)
		}
		fmt.Println("Warning: Tmux session creation failed, but other steps completed successfully")
	} else {
		fmt.Println("Tmux session created successfully")
	}

	fmt.Printf("\nHack workflow for %s completed successfully!\n", name)
	fmt.Printf("Worktree: %s\n", worktreePath)
	if notePath != "" {
		fmt.Printf("Note: %s\n", notePath)
	}

	return nil
}
