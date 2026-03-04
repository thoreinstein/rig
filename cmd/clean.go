package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/spf13/cobra"

	"thoreinstein.com/rig/pkg/config"
	"thoreinstein.com/rig/pkg/tmux"
	"thoreinstein.com/rig/pkg/vcs"
)

var cleanDryRun bool
var cleanForce bool

// cleanCmd represents the clean command
var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Clean up old worktrees and associated sessions",
	Long: `Clean up git worktrees and their associated tmux sessions.

This command identifies worktrees that can be safely removed and offers
to clean them up. By default, it prompts for confirmation before removing.

Examples:
  rig clean              # Interactive cleanup with confirmation
  rig clean --dry-run    # Show what would be removed without removing
  rig clean --force      # Remove without confirmation`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runCleanCommand()
	},
}

func init() {
	rootCmd.AddCommand(cleanCmd)

	cleanCmd.Flags().BoolVar(&cleanDryRun, "dry-run", false, "Show what would be removed without removing")
	cleanCmd.Flags().BoolVar(&cleanForce, "force", false, "Remove without confirmation prompts")
}

// CleanupCandidate represents a worktree that can be cleaned up
type CleanupCandidate struct {
	Path       string
	Branch     string
	RepoName   string
	RepoPath   string
	IsMerged   bool
	HasSession bool
}

func runCleanCommand() error {
	// Load configuration
	cfg, err := loadConfig()
	if err != nil {
		return errors.Wrap(err, "failed to load configuration")
	}

	// Initialize VCS provider
	provider, cleanup, err := getVCSProvider(cfg)
	if err != nil {
		return errors.Wrap(err, "failed to initialize VCS provider")
	}
	defer cleanup()

	// Find cleanup candidates
	candidates, err := findCleanupCandidates(cfg, provider)
	if err != nil {
		return errors.Wrap(err, "failed to find cleanup candidates")
	}

	if len(candidates) == 0 {
		fmt.Println("No worktrees found to clean up.")
		return nil
	}

	// Display candidates
	fmt.Println("=== Cleanup Candidates ===")
	fmt.Println()

	for i, candidate := range candidates {
		status := ""
		if candidate.IsMerged {
			status = " [merged]"
		}
		if candidate.HasSession {
			status += " [has session]"
		}

		relPath := strings.TrimPrefix(candidate.Path, candidate.RepoPath+"/")
		fmt.Printf("  %d. [%s] %s%s\n", i+1, candidate.RepoName, relPath, status)
		if verbose {
			fmt.Printf("      Branch: %s\n", candidate.Branch)
			fmt.Printf("      Path: %s\n", candidate.Path)
		}
	}
	fmt.Println()

	if cleanDryRun {
		fmt.Printf("Would remove %d worktree(s) (dry-run mode)\n", len(candidates))
		return nil
	}

	// Confirm unless --force
	if !cleanForce {
		fmt.Printf("Remove %d worktree(s)? [y/N]: ", len(candidates))
		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			return errors.Wrap(err, "failed to read input")
		}

		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	// Remove worktrees
	removed := 0
	for _, candidate := range candidates {
		err := removeWorktree(cfg, provider, candidate)
		if err != nil {
			fmt.Printf("  Failed to remove %s: %v\n", candidate.Path, err)
		} else {
			fmt.Printf("  Removed %s\n", candidate.Path)
			removed++
		}
	}

	fmt.Printf("\nRemoved %d worktree(s)\n", removed)
	return nil
}

func findCleanupCandidates(cfg *config.Config, provider vcs.Provider) ([]CleanupCandidate, error) {
	// Use current directory to find repo
	cwd, _ := config.UserHomeDir() // Fallback
	if c, err := os.Getwd(); err == nil {
		cwd = c
	}

	repoRoot, err := provider.GetRepoRoot(cwd)
	if err != nil {
		return nil, err
	}
	repoName, err := provider.GetRepoName(cwd)
	if err != nil {
		return nil, err
	}

	sessionManager := tmux.NewSessionManager(cfg.Tmux.SessionPrefix, nil, verbose)
	sessions, err := sessionManager.ListSessions()
	if err != nil && verbose {
		fmt.Printf("Warning: Could not list tmux sessions: %v\n", err)
	}
	sessionSet := make(map[string]bool)
	for _, s := range sessions {
		sessionSet[s] = true
	}

	worktrees, err := provider.ListWorktrees(cwd)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list worktrees")
	}

	// Determine base branch for merge checking
	baseBranch, err := provider.GetDefaultBranch(cwd, cfg.Git.BaseBranch)
	if err != nil {
		baseBranch = "main" // fallback
	}

	candidates := make([]CleanupCandidate, 0, len(worktrees))
	for _, wt := range worktrees {
		// Skip the main repo path (handle symlink resolution for comparison)
		realWt, _ := filepath.EvalSymlinks(wt.Path)
		realRepoRoot, _ := filepath.EvalSymlinks(repoRoot)
		if wt.Path == repoRoot || realWt == realRepoRoot {
			continue
		}

		// Determine session name from worktree path
		sessionName := filepath.Base(wt.Path)
		if cfg.Tmux.SessionPrefix != "" {
			sessionName = cfg.Tmux.SessionPrefix + sessionName
		}

		// Check if branch is merged (default to not-merged on error to avoid accidental deletion)
		isMerged, err := provider.IsBranchMerged(cwd, wt.Branch, baseBranch)
		if err != nil && verbose {
			fmt.Printf("Warning: could not check merge status for %s: %v\n", wt.Branch, err)
		}

		candidate := CleanupCandidate{
			Path:       wt.Path,
			Branch:     wt.Branch,
			RepoName:   repoName,
			RepoPath:   repoRoot,
			IsMerged:   isMerged,
			HasSession: sessionSet[sessionName],
		}

		candidates = append(candidates, candidate)
	}

	return candidates, nil
}

func removeWorktree(cfg *config.Config, provider vcs.Provider, candidate CleanupCandidate) error {
	// Kill associated tmux session first
	if candidate.HasSession {
		sessionName := filepath.Base(candidate.Path)
		if cfg.Tmux.SessionPrefix != "" {
			sessionName = cfg.Tmux.SessionPrefix + sessionName
		}

		sessionManager := tmux.NewSessionManager(cfg.Tmux.SessionPrefix, nil, verbose)
		if err := sessionManager.KillSession(filepath.Base(candidate.Path)); err != nil {
			if verbose {
				fmt.Printf("    Warning: Could not kill session %s: %v\n", sessionName, err)
			}
		} else if verbose {
			fmt.Printf("    Killed tmux session: %s\n", sessionName)
		}
	}

	// Remove the worktree
	// Extract type and name from path
	// Path structure: repoPath/type/ticket or repoPath/type/.../ticket
	// Normalize paths to handle symlink differences (e.g., /var vs /private/var on macOS)
	candidatePath, _ := filepath.EvalSymlinks(candidate.Path)
	repoPath, _ := filepath.EvalSymlinks(candidate.RepoPath)
	if candidatePath == "" {
		candidatePath = candidate.Path
	}
	if repoPath == "" {
		repoPath = candidate.RepoPath
	}

	relPath := strings.TrimPrefix(candidatePath, repoPath+"/")
	parts := strings.Split(relPath, string(filepath.Separator))
	if len(parts) < 2 {
		// Single-level path or unusual structure - use force remove
		return provider.ForceRemoveWorktree(candidate.RepoPath, candidate.Path)
	}

	// First part is always the ticket type, last part is the ticket name
	ticketType := parts[0]
	ticketName := parts[len(parts)-1]

	return provider.RemoveWorktree(candidate.RepoPath, ticketType, ticketName)
}
