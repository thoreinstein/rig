package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CommandRunner executes shell commands and returns output
// This interface allows for mocking in tests
type CommandRunner interface {
	Run(dir string, name string, args ...string) error
	Output(dir string, name string, args ...string) ([]byte, error)
}

// RealCommandRunner executes actual shell commands
type RealCommandRunner struct {
	Verbose bool
}

// Run executes a command without capturing output
func (r *RealCommandRunner) Run(dir string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if r.Verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	return cmd.Run()
}

// Output executes a command and returns its output
func (r *RealCommandRunner) Output(dir string, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	return cmd.Output()
}

// WorktreeManager handles Git worktree operations
type WorktreeManager struct {
	RepoPath   string
	BaseBranch string
	Verbose    bool
	runner     CommandRunner
}

// NewWorktreeManager creates a new WorktreeManager
func NewWorktreeManager(repoPath, baseBranch string, verbose bool) *WorktreeManager {
	return &WorktreeManager{
		RepoPath:   repoPath,
		BaseBranch: baseBranch,
		Verbose:    verbose,
		runner:     &RealCommandRunner{Verbose: verbose},
	}
}

// NewWorktreeManagerWithRunner creates a WorktreeManager with a custom CommandRunner (for testing)
func NewWorktreeManagerWithRunner(repoPath, baseBranch string, verbose bool, runner CommandRunner) *WorktreeManager {
	return &WorktreeManager{
		RepoPath:   repoPath,
		BaseBranch: baseBranch,
		Verbose:    verbose,
		runner:     runner,
	}
}

// CreateWorktree creates a new git worktree for the given ticket
// The branch name defaults to the ticket name
func (wm *WorktreeManager) CreateWorktree(ticketType, ticket string) (string, error) {
	return wm.CreateWorktreeWithBranch(ticketType, ticket, ticket)
}

// CreateWorktreeWithBranch creates a new git worktree with a custom branch name
func (wm *WorktreeManager) CreateWorktreeWithBranch(ticketType, name, branchName string) (string, error) {
	worktreePath := filepath.Join(wm.RepoPath, ticketType, name)

	// Check if bare repo exists
	if !wm.repoExists() {
		return "", fmt.Errorf("bare repository not found at %s", wm.RepoPath)
	}

	// Create type directory if it doesn't exist
	typeDir := filepath.Join(wm.RepoPath, ticketType)
	if err := os.MkdirAll(typeDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create type directory: %w", err)
	}

	// Check if worktree already exists
	if _, err := os.Stat(worktreePath); err == nil {
		if wm.Verbose {
			fmt.Printf("Worktree already exists at %s\n", worktreePath)
		}
		return worktreePath, nil
	}

	// Determine base branch to use
	baseBranch, err := wm.getBaseBranch()
	if err != nil {
		return "", fmt.Errorf("failed to determine base branch: %w", err)
	}

	// Fetch and pull latest changes before creating worktree
	if err := wm.fetchAndPull(baseBranch); err != nil {
		// Log warning but don't fail - repo might be offline or have no remote
		if wm.Verbose {
			fmt.Printf("Warning: Could not fetch/pull latest changes: %v\n", err)
		}
	}

	if wm.Verbose {
		fmt.Printf("Creating git worktree for %s using base branch %s...\n", name, baseBranch)
	}

	// Create the worktree with custom branch name
	err = wm.createWorktreeFromBranchWithName(ticketType, name, branchName, baseBranch)
	if err != nil {
		return "", fmt.Errorf("failed to create worktree: %w", err)
	}

	return worktreePath, nil
}

// repoExists checks if the repository exists and is a git repository
func (wm *WorktreeManager) repoExists() bool {
	if _, err := os.Stat(wm.RepoPath); os.IsNotExist(err) {
		return false
	}
	
	// Check if it's a git repository
	gitDir := filepath.Join(wm.RepoPath, ".git")
	if _, err := os.Stat(gitDir); err == nil {
		return true
	}
	
	// Check if it's a bare repository
	if _, err := os.Stat(filepath.Join(wm.RepoPath, "refs")); err == nil {
		return true
	}
	
	return false
}

// getBaseBranch determines which base branch to use
func (wm *WorktreeManager) getBaseBranch() (string, error) {
	branches := []string{wm.BaseBranch, "master", "main"}
	
	for _, branch := range branches {
		if wm.branchExists(branch) {
			return branch, nil
		}
	}
	
	// If no standard branches exist, get the first available branch
	branch, err := wm.getFirstBranch()
	if err != nil {
		// If no branches exist, create initial commit
		return wm.createInitialBranch()
	}
	
	return branch, nil
}

// fetchAndPull fetches from origin and pulls the latest changes for the base branch
func (wm *WorktreeManager) fetchAndPull(baseBranch string) error {
	if wm.Verbose {
		fmt.Println("Fetching latest changes from origin...")
	}

	// git fetch origin
	if err := wm.runner.Run(wm.RepoPath, "git", "fetch", "origin"); err != nil {
		return fmt.Errorf("git fetch failed: %w", err)
	}

	if wm.Verbose {
		fmt.Printf("Pulling latest changes for %s...\n", baseBranch)
	}

	// git pull origin <baseBranch>
	if err := wm.runner.Run(wm.RepoPath, "git", "pull", "origin", baseBranch); err != nil {
		return fmt.Errorf("git pull failed: %w", err)
	}

	return nil
}

// branchExists checks if a branch exists in the repository
func (wm *WorktreeManager) branchExists(branch string) bool {
	err := wm.runner.Run(wm.RepoPath, "git", "show-ref", "--verify", "--quiet", fmt.Sprintf("refs/heads/%s", branch))
	return err == nil
}

// getFirstBranch gets the first available branch
func (wm *WorktreeManager) getFirstBranch() (string, error) {
	output, err := wm.runner.Output(wm.RepoPath, "git", "branch", "-r")
	if err != nil {
		return "", err
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "HEAD ->") {
			continue
		}
		if strings.HasPrefix(line, "origin/") {
			return strings.TrimPrefix(line, "origin/"), nil
		}
	}

	return "", fmt.Errorf("no branches found")
}

// createInitialBranch creates an initial branch with empty commit
func (wm *WorktreeManager) createInitialBranch() (string, error) {
	if wm.Verbose {
		fmt.Println("Creating initial commit on main branch...")
	}

	// Switch to main branch
	if err := wm.runner.Run(wm.RepoPath, "git", "switch", "-c", "main"); err != nil {
		return "", fmt.Errorf("failed to create main branch: %w", err)
	}

	// Create empty commit
	if err := wm.runner.Run(wm.RepoPath, "git", "commit", "--allow-empty", "-m", "Initial commit"); err != nil {
		return "", fmt.Errorf("failed to create initial commit: %w", err)
	}

	return "main", nil
}

// createWorktreeFromBranch creates a worktree from the specified base branch
// Uses the ticket name as both the directory name and branch name
func (wm *WorktreeManager) createWorktreeFromBranch(ticketType, ticket, baseBranch string) error {
	return wm.createWorktreeFromBranchWithName(ticketType, ticket, ticket, baseBranch)
}

// createWorktreeFromBranchWithName creates a worktree with a custom branch name
func (wm *WorktreeManager) createWorktreeFromBranchWithName(ticketType, name, branchName, baseBranch string) error {
	relativePath := filepath.Join(ticketType, name)
	return wm.runner.Run(wm.RepoPath, "git", "worktree", "add", relativePath, "-b", branchName, baseBranch)
}

// ListWorktrees returns a list of all existing worktrees
func (wm *WorktreeManager) ListWorktrees() ([]string, error) {
	output, err := wm.runner.Output(wm.RepoPath, "git", "worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w", err)
	}

	var worktrees []string
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")

	for _, line := range lines {
		if strings.HasPrefix(line, "worktree ") {
			path := strings.TrimPrefix(line, "worktree ")
			worktrees = append(worktrees, path)
		}
	}

	return worktrees, nil
}

// RemoveWorktree removes a worktree
func (wm *WorktreeManager) RemoveWorktree(ticketType, ticket string) error {
	worktreePath := filepath.Join(wm.RepoPath, ticketType, ticket)

	// Check if worktree exists
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		return fmt.Errorf("worktree does not exist: %s", worktreePath)
	}

	relativePath := filepath.Join(ticketType, ticket)
	return wm.runner.Run(wm.RepoPath, "git", "worktree", "remove", relativePath)
}