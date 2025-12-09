package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"thoreinstein.com/sre/pkg/config"
	"thoreinstein.com/sre/pkg/git"
	"thoreinstein.com/sre/pkg/tmux"
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
  sre clean              # Interactive cleanup with confirmation
  sre clean --dry-run    # Show what would be removed without removing
  sre clean --force      # Remove without confirmation`,
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
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Find cleanup candidates
	candidates, err := findCleanupCandidates(cfg)
	if err != nil {
		return fmt.Errorf("failed to find cleanup candidates: %w", err)
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
			return fmt.Errorf("failed to read input: %w", err)
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
		err := removeWorktree(cfg, candidate)
		if err != nil {
			fmt.Printf("  ✗ Failed to remove %s: %v\n", candidate.Path, err)
		} else {
			fmt.Printf("  ✓ Removed %s\n", candidate.Path)
			removed++
		}
	}

	fmt.Printf("\nRemoved %d worktree(s)\n", removed)
	return nil
}

func findCleanupCandidates(cfg *config.Config) ([]CleanupCandidate, error) {
	var candidates []CleanupCandidate

	repos := cfg.GetAllRepos()
	sessionManager := tmux.NewSessionManager(cfg.Tmux.SessionPrefix, nil, verbose)
	sessions, _ := sessionManager.ListSessions()
	sessionSet := make(map[string]bool)
	for _, s := range sessions {
		sessionSet[s] = true
	}

	for repoName, repoConfig := range repos {
		repoPath := cfg.GetRepositoryPathForRepo(repoConfig)
		gitManager := git.NewWorktreeManager(repoPath, repoConfig.BaseBranch, verbose)

		worktrees, err := gitManager.ListWorktrees()
		if err != nil {
			if verbose {
				fmt.Printf("Warning: Could not list worktrees for %s: %v\n", repoName, err)
			}
			continue
		}

		worktreeDetails := getWorktreeDetailsForClean(repoPath)

		for _, wt := range worktrees {
			// Skip the main repo path
			if wt == repoPath {
				continue
			}

			// Get branch info
			branch := ""
			if info, ok := worktreeDetails[wt]; ok {
				branch = info.Branch
			}

			// Determine session name from worktree path
			sessionName := filepath.Base(wt)
			if cfg.Tmux.SessionPrefix != "" {
				sessionName = cfg.Tmux.SessionPrefix + sessionName
			}

			// Check if branch is merged
			isMerged := isBranchMerged(repoPath, branch, repoConfig.BaseBranch)

			candidate := CleanupCandidate{
				Path:       wt,
				Branch:     branch,
				RepoName:   repoName,
				RepoPath:   repoPath,
				IsMerged:   isMerged,
				HasSession: sessionSet[sessionName],
			}

			candidates = append(candidates, candidate)
		}
	}

	return candidates, nil
}

func getWorktreeDetailsForClean(repoPath string) map[string]WorktreeInfo {
	result := make(map[string]WorktreeInfo)

	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		return result
	}

	lines := strings.Split(string(output), "\n")
	var currentPath string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "worktree ") {
			currentPath = strings.TrimPrefix(line, "worktree ")
			result[currentPath] = WorktreeInfo{Path: currentPath}
		} else if strings.HasPrefix(line, "branch ") && currentPath != "" {
			branch := strings.TrimPrefix(line, "branch refs/heads/")
			info := result[currentPath]
			info.Branch = branch
			result[currentPath] = info
		}
	}

	return result
}

func isBranchMerged(repoPath, branch, baseBranch string) bool {
	if branch == "" || branch == baseBranch {
		return false
	}

	// Check if branch is merged into base branch
	cmd := exec.Command("git", "branch", "--merged", baseBranch)
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "* ") // Current branch marker
		if line == branch {
			return true
		}
	}

	return false
}

func removeWorktree(cfg *config.Config, candidate CleanupCandidate) error {
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
	gitManager := git.NewWorktreeManager(candidate.RepoPath, "", verbose)

	// Extract type and name from path
	relPath := strings.TrimPrefix(candidate.Path, candidate.RepoPath+"/")
	parts := strings.Split(relPath, string(filepath.Separator))
	if len(parts) < 2 {
		// Try force remove if we can't parse the path structure
		return forceRemoveWorktree(candidate.RepoPath, candidate.Path)
	}

	ticketType := parts[0]
	ticketName := parts[1]

	return gitManager.RemoveWorktree(ticketType, ticketName)
}

func forceRemoveWorktree(repoPath, worktreePath string) error {
	cmd := exec.Command("git", "worktree", "remove", "--force", worktreePath)
	cmd.Dir = repoPath

	if verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	return cmd.Run()
}
