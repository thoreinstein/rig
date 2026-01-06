package cmd

import (
	"fmt"

	"github.com/cockroachdb/errors"
	"github.com/spf13/cobra"

	"thoreinstein.com/sre/pkg/config"
	"thoreinstein.com/sre/pkg/git"
)

// cloneCmd represents the clone command
var cloneCmd = &cobra.Command{
	Use:   "clone <url>",
	Short: "Clone a GitHub repository to ~/src/<owner>/<repo>",
	Long: `Clone a GitHub repository using a structured directory layout.

This command clones repositories to ~/src/<owner>/<repo> with different
strategies based on the URL protocol:

SSH URLs (git@github.com:...):
  - Creates a bare clone for worktree workflow
  - Configures fetch refspec for remote tracking
  - Creates initial worktree for the default branch

HTTPS URLs (https://github.com/...):
  - Performs a standard git clone

Shorthand URLs (github.com/owner/repo):
  - Interpreted as SSH by default

Examples:
  sre clone git@github.com:thoreinstein/sre.git
  sre clone https://github.com/thoreinstein/sre
  sre clone github.com/owner/repo`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runCloneCommand(args[0])
	},
}

func init() {
	rootCmd.AddCommand(cloneCmd)
}

func runCloneCommand(urlInput string) error {
	// Parse the URL first
	repoURL, err := git.ParseGitHubURL(urlInput)
	if err != nil {
		return err
	}

	if verbose {
		fmt.Printf("Parsed URL:\n")
		fmt.Printf("  Original: %s\n", repoURL.Original)
		fmt.Printf("  Canonical: %s\n", repoURL.Canonical)
		fmt.Printf("  Protocol: %s\n", repoURL.Protocol)
		fmt.Printf("  Owner: %s\n", repoURL.Owner)
		fmt.Printf("  Repo: %s\n", repoURL.Repo)
	}

	// Load configuration to get base path (if configured)
	cfg, err := config.Load()
	if err != nil {
		return errors.Wrap(err, "failed to load configuration")
	}

	// Get base path from config or use default
	basePath := cfg.Clone.BasePath

	// Create clone manager and perform clone
	cloneManager := git.NewCloneManager(basePath, verbose)

	repoPath, err := cloneManager.Clone(repoURL)
	if err != nil {
		return errors.Wrap(err, "clone failed")
	}

	fmt.Printf("Repository cloned to: %s\n", repoPath)

	if repoURL.Protocol == "ssh" {
		fmt.Printf("\nWorktree workflow enabled. Use 'sre hack <name>' from within the repo to create feature worktrees.\n")
	}

	return nil
}
