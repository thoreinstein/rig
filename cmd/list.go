package cmd

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/spf13/cobra"

	"thoreinstein.com/sre/pkg/config"
	"thoreinstein.com/sre/pkg/git"
	"thoreinstein.com/sre/pkg/tmux"
)

var listWorktrees bool
var listSessions bool

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List active worktrees and tmux sessions",
	Long: `List all active git worktrees and tmux sessions.

By default, shows both worktrees and sessions. Use flags to filter.

Examples:
  sre list              # Show both worktrees and sessions
  sre list --worktrees  # Show only worktrees
  sre list --sessions   # Show only tmux sessions`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runListCommand()
	},
}

func init() {
	rootCmd.AddCommand(listCmd)

	listCmd.Flags().BoolVar(&listWorktrees, "worktrees", false, "Show only git worktrees")
	listCmd.Flags().BoolVar(&listSessions, "sessions", false, "Show only tmux sessions")
}

// WorktreeInfo holds information about a worktree
type WorktreeInfo struct {
	Path   string
	Branch string
	Repo   string
}

func runListCommand() error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return errors.Wrap(err, "failed to load configuration")
	}

	// If neither flag is set, show both
	showWorktrees := listWorktrees || (!listWorktrees && !listSessions)
	showSessions := listSessions || (!listWorktrees && !listSessions)

	if showWorktrees {
		if err := listCurrentRepoWorktrees(cfg); err != nil {
			// Don't fail completely if worktrees can't be listed
			if verbose {
				fmt.Printf("Warning: Could not list worktrees: %v\n", err)
			}
		}
	}

	if showSessions {
		if showWorktrees {
			fmt.Println() // Add spacing between sections
		}
		if err := listAllSessions(cfg); err != nil {
			// Don't fail completely if sessions can't be listed
			if verbose {
				fmt.Printf("Warning: Could not list sessions: %v\n", err)
			}
		}
	}

	return nil
}

func listCurrentRepoWorktrees(cfg *config.Config) error {
	fmt.Println("=== Git Worktrees ===")
	fmt.Println()

	gitManager := git.NewWorktreeManager(cfg.Git.BaseBranch, verbose)

	repoRoot, err := gitManager.GetRepoRoot()
	if err != nil {
		return err
	}
	repoName, err := gitManager.GetRepoName()
	if err != nil {
		return err
	}

	worktrees, err := gitManager.ListWorktrees()
	if err != nil {
		return errors.Wrap(err, "failed to list worktrees")
	}

	if len(worktrees) == 0 {
		fmt.Println("  No worktrees found")
		return nil
	}

	// Get branch info for each worktree
	worktreeInfos := getWorktreeDetails(repoRoot)

	fmt.Printf("[%s]\n", repoName)

	totalWorktrees := 0
	for _, wt := range worktrees {
		// Skip the main repo path itself
		if wt == repoRoot {
			continue
		}

		// Get relative path from repo
		relPath := strings.TrimPrefix(wt, repoRoot+"/")
		if relPath == wt {
			relPath = wt // Couldn't make relative, use full path
		}

		// Find branch info
		branch := ""
		if info, ok := worktreeInfos[wt]; ok {
			branch = info.Branch
		}

		if branch != "" {
			fmt.Printf("  %-40s [%s]\n", relPath, branch)
		} else {
			fmt.Printf("  %s\n", relPath)
		}
		totalWorktrees++
	}
	fmt.Println()

	if totalWorktrees == 0 {
		fmt.Println("  No worktrees found")
	} else {
		fmt.Printf("Total: %d worktree(s)\n", totalWorktrees)
	}

	return nil
}

func getWorktreeDetails(repoPath string) map[string]WorktreeInfo {
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

func listAllSessions(cfg *config.Config) error {
	fmt.Println("=== Tmux Sessions ===")
	fmt.Println()

	// Check if tmux is running
	if !isTmuxRunning() {
		fmt.Println("  No tmux server running")
		return nil
	}

	sessionManager := tmux.NewSessionManager(cfg.Tmux.SessionPrefix, nil, verbose)
	sessions, err := sessionManager.ListSessions()
	if err != nil {
		return errors.Wrap(err, "failed to list sessions")
	}

	if len(sessions) == 0 {
		fmt.Println("  No active sessions")
		return nil
	}

	// Get detailed session info
	sessionDetails := getSessionDetails()

	for _, session := range sessions {
		if detail, ok := sessionDetails[session]; ok {
			fmt.Printf("  %-30s %s\n", session, detail)
		} else {
			fmt.Printf("  %s\n", session)
		}
	}
	fmt.Println()
	fmt.Printf("Total: %d session(s)\n", len(sessions))

	return nil
}

func isTmuxRunning() bool {
	cmd := exec.Command("tmux", "list-sessions")
	return cmd.Run() == nil
}

func getSessionDetails() map[string]string {
	result := make(map[string]string)

	// Get session details: name, windows count, attached status
	cmd := exec.Command("tmux", "list-sessions", "-F", "#{session_name}|#{session_windows}|#{session_attached}")
	output, err := cmd.Output()
	if err != nil {
		return result
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		parts := strings.Split(line, "|")
		if len(parts) >= 3 {
			name := parts[0]
			windows := parts[1]
			attached := parts[2]

			status := fmt.Sprintf("(%s windows)", windows)
			if attached == "1" {
				status += " [attached]"
			}
			result[name] = status
		}
	}

	return result
}
