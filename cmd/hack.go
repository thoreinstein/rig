package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"thoreinstein.com/sre/pkg/config"
	"thoreinstein.com/sre/pkg/git"
	"thoreinstein.com/sre/pkg/obsidian"
	"thoreinstein.com/sre/pkg/tmux"
)

var hackNotes bool
var hackRepo string

// hackCmd represents the hack command
var hackCmd = &cobra.Command{
	Use:   "hack <name>",
	Short: "Initialize a hack worktree for non-ticket work",
	Long: `Initialize a hack worktree for exploratory or non-ticket work.

This command creates a simplified workflow without JIRA integration:
- Creates git worktree at {repo}/hack/{name}
- Creates branch hack/{name}
- Optionally creates Obsidian note with --notes flag
- Creates tmux session

Examples:
  sre hack winter-2025
  sre hack experiment-auth --notes
  sre hack quick-fix --repo infra`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runHackCommand(args[0])
	},
}

func init() {
	rootCmd.AddCommand(hackCmd)

	hackCmd.Flags().BoolVar(&hackNotes, "notes", false, "Create an Obsidian note for this hack")
	hackCmd.Flags().StringVar(&hackRepo, "repo", "", "Repository to use (defaults to default_repo or first configured repo)")
}

func runHackCommand(name string) error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Get repository config
	var repoConfig *config.RepositoryConfig
	if hackRepo != "" {
		repoConfig, err = cfg.GetRepoByName(hackRepo)
		if err != nil {
			return fmt.Errorf("invalid repository: %w", err)
		}
	} else {
		repoConfig = cfg.GetDefaultRepo()
	}
	repoPath := cfg.GetRepositoryPathForRepo(repoConfig)

	if verbose {
		fmt.Printf("Starting hack workflow for: %s\n", name)
		fmt.Printf("  Repository: %s/%s\n", repoConfig.Owner, repoConfig.Name)
		fmt.Printf("  Notes: %v\n", hackNotes)
	}

	// Step 1: Create git worktree
	if verbose {
		fmt.Println("Creating git worktree...")
	}
	gitManager := git.NewWorktreeManager(repoPath, repoConfig.BaseBranch, verbose)

	// For hacks, use "hack" as the type directory and "hack/{name}" as the branch name
	worktreePath, err := gitManager.CreateWorktreeWithBranch("hack", name, "hack/"+name)
	if err != nil {
		return fmt.Errorf("failed to create git worktree: %w", err)
	}
	fmt.Printf("✓ Git worktree created at: %s\n", worktreePath)

	// Step 2: Create Obsidian note (only if --notes flag is set)
	var notePath string
	if hackNotes {
		if verbose {
			fmt.Println("Creating Obsidian note...")
		}
		noteManager := obsidian.NewNoteManager(
			cfg.Vault.Path,
			cfg.Vault.TemplatesDir,
			cfg.Vault.AreasDir,
			cfg.Vault.DailyDir,
			verbose,
		)
		noteManager.SetVaultSubdir("Hacks")
		notePath, err = noteManager.CreateTicketNote("hack", name, nil)
		if err != nil {
			// Don't fail if note creation fails
			if verbose {
				fmt.Printf("Warning: Could not create Obsidian note: %v\n", err)
			}
		} else {
			fmt.Printf("✓ Obsidian note created at: %s\n", notePath)
		}
	}

	// Step 3: Create tmux session
	if verbose {
		fmt.Println("Creating tmux session...")
	}

	// Convert config windows to tmux windows
	var tmuxWindows []tmux.WindowConfig
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
		fmt.Println("✓ Tmux session created successfully")
	}

	fmt.Printf("\n🎉 Hack workflow for %s completed successfully!\n", name)
	fmt.Printf("Worktree: %s\n", worktreePath)
	if notePath != "" {
		fmt.Printf("Note: %s\n", notePath)
	}

	return nil
}
