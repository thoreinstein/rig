package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cockroachdb/errors"
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
// Repository information is derived from git itself, not configuration
type WorktreeManager struct {
	Verbose          bool
	BaseBranchConfig string // Optional config override for base branch
	runner           CommandRunner
	getwd            func() (string, error) // For testing; defaults to os.Getwd
}

// NewWorktreeManager creates a new WorktreeManager
func NewWorktreeManager(baseBranchConfig string, verbose bool) *WorktreeManager {
	return &WorktreeManager{
		Verbose:          verbose,
		BaseBranchConfig: baseBranchConfig,
		runner:           &RealCommandRunner{Verbose: verbose},
		getwd:            os.Getwd,
	}
}

// NewWorktreeManagerWithRunner creates a WorktreeManager with a custom CommandRunner (for testing)
func NewWorktreeManagerWithRunner(baseBranchConfig string, verbose bool, runner CommandRunner) *WorktreeManager {
	return &WorktreeManager{
		Verbose:          verbose,
		BaseBranchConfig: baseBranchConfig,
		runner:           runner,
		getwd:            os.Getwd,
	}
}

// GetRepoRoot returns the bare repository root from the current working directory.
// This works correctly from both bare repositories and worktrees by using
// --git-common-dir which returns the shared git directory across all worktrees.
func (wm *WorktreeManager) GetRepoRoot() (string, error) {
	// Use --git-common-dir to get the shared git directory
	// This works from both bare repos and worktrees
	output, err := wm.runner.Output(".", "git", "rev-parse", "--git-common-dir")
	if err != nil {
		return "", errors.New("not inside a git repository. Run this command from within your repo")
	}

	commonDir := strings.TrimSpace(string(output))

	// If it's a relative path (like "." in bare repos), resolve to absolute
	if !filepath.IsAbs(commonDir) {
		cwd, err := wm.getwd()
		if err != nil {
			return "", errors.Wrap(err, "failed to get working directory")
		}
		commonDir = filepath.Join(cwd, commonDir)
	}

	// Clean the path to resolve any .. or . components
	return filepath.Clean(commonDir), nil
}

// GetRepoName returns the repository name (basename of repo root)
func (wm *WorktreeManager) GetRepoName() (string, error) {
	root, err := wm.GetRepoRoot()
	if err != nil {
		return "", err
	}
	return filepath.Base(root), nil
}

// GetDefaultBranch determines the default branch to use for new worktrees
// Priority: config override > remote HEAD > main > master > first remote branch
func (wm *WorktreeManager) GetDefaultBranch() (string, error) {
	repoRoot, err := wm.GetRepoRoot()
	if err != nil {
		return "", err
	}

	// 1. Use config override if provided
	if wm.BaseBranchConfig != "" {
		if wm.branchExists(repoRoot, wm.BaseBranchConfig) {
			return wm.BaseBranchConfig, nil
		}
		// Config specified but branch doesn't exist - warn but continue
		if wm.Verbose {
			fmt.Printf("Warning: configured base branch %q not found, auto-detecting...\n", wm.BaseBranchConfig)
		}
	}

	// 2. Try to get default branch from remote HEAD
	output, err := wm.runner.Output(repoRoot, "git", "symbolic-ref", "refs/remotes/origin/HEAD")
	if err == nil {
		ref := strings.TrimSpace(string(output))
		// Format: refs/remotes/origin/main -> main
		if strings.HasPrefix(ref, "refs/remotes/origin/") {
			branch := strings.TrimPrefix(ref, "refs/remotes/origin/")
			if wm.branchExists(repoRoot, branch) {
				return branch, nil
			}
		}
	}

	// 3. Check for common default branch names
	for _, branch := range []string{"main", "master"} {
		if wm.branchExists(repoRoot, branch) {
			return branch, nil
		}
	}

	// 4. Try to get first available remote branch
	branch, err := wm.getFirstRemoteBranch(repoRoot)
	if err == nil {
		return branch, nil
	}

	// 5. No branches exist - create initial branch
	return wm.createInitialBranch(repoRoot)
}

// CreateWorktree creates a new git worktree for the given ticket
// The branch name defaults to the ticket name
func (wm *WorktreeManager) CreateWorktree(ticketType, ticket string) (string, error) {
	return wm.CreateWorktreeWithBranch(ticketType, ticket, ticket)
}

// CreateWorktreeWithBranch creates a new git worktree with a custom branch name
func (wm *WorktreeManager) CreateWorktreeWithBranch(ticketType, name, branchName string) (string, error) {
	repoRoot, err := wm.GetRepoRoot()
	if err != nil {
		return "", err
	}

	worktreePath := filepath.Join(repoRoot, ticketType, name)

	// Validate path stays within repo root (prevent path traversal)
	if !strings.HasPrefix(worktreePath, repoRoot+string(filepath.Separator)) {
		return "", errors.New("invalid path: worktree path escapes repository root")
	}

	// Create type directory if it doesn't exist
	typeDir := filepath.Join(repoRoot, ticketType)
	if err := os.MkdirAll(typeDir, 0755); err != nil {
		return "", errors.Wrap(err, "failed to create type directory")
	}

	// Check if worktree already exists
	if _, err := os.Stat(worktreePath); err == nil {
		if wm.Verbose {
			fmt.Printf("Worktree already exists at %s\n", worktreePath)
		}
		return worktreePath, nil
	}

	// Determine base branch to use
	baseBranch, err := wm.GetDefaultBranch()
	if err != nil {
		return "", errors.Wrap(err, "failed to determine base branch")
	}

	// Fetch and pull latest changes before creating worktree
	if err := wm.fetchAndPull(repoRoot, baseBranch); err != nil {
		// Log warning but don't fail - repo might be offline or have no remote
		if wm.Verbose {
			fmt.Printf("Warning: Could not fetch/pull latest changes: %v\n", err)
		}
	}

	if wm.Verbose {
		fmt.Printf("Creating git worktree for %s using base branch %s...\n", name, baseBranch)
	}

	// Create the worktree with custom branch name
	relativePath := filepath.Join(ticketType, name)
	err = wm.runner.Run(repoRoot, "git", "worktree", "add", relativePath, "-b", branchName, baseBranch)
	if err != nil {
		return "", errors.Wrap(err, "failed to create worktree")
	}

	return worktreePath, nil
}

// ensureFetchRefspec ensures the fetch refspec is configured for the origin remote.
// Bare repos created with `git clone --bare` don't have this configured by default,
// which causes `git fetch` to not download remote-tracking branches.
func (wm *WorktreeManager) ensureFetchRefspec(repoRoot string) error {
	// Check if fetch refspec already exists
	output, err := wm.runner.Output(repoRoot, "git", "config", "--get", "remote.origin.fetch")
	if err == nil && len(strings.TrimSpace(string(output))) > 0 {
		// Already configured
		return nil
	}

	// Add the standard fetch refspec
	if wm.Verbose {
		fmt.Println("Adding missing fetch refspec for bare repository...")
	}

	if err := wm.runner.Run(repoRoot, "git", "config", "remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*"); err != nil {
		return errors.Wrap(err, "failed to configure fetch refspec")
	}

	return nil
}

// fetchAndPull fetches from origin and pulls the latest changes for the base branch
func (wm *WorktreeManager) fetchAndPull(repoRoot, baseBranch string) error {
	// Ensure fetch refspec is configured (needed for bare repos)
	if err := wm.ensureFetchRefspec(repoRoot); err != nil {
		// Log warning but don't fail - we'll try fetch anyway
		if wm.Verbose {
			fmt.Printf("Warning: Could not ensure fetch refspec: %v\n", err)
		}
	}

	if wm.Verbose {
		fmt.Println("Fetching latest changes from origin...")
	}

	// git fetch origin
	if err := wm.runner.Run(repoRoot, "git", "fetch", "origin"); err != nil {
		return errors.Wrap(err, "git fetch failed")
	}

	if wm.Verbose {
		fmt.Printf("Pulling latest changes for %s...\n", baseBranch)
	}

	// git pull origin <baseBranch>
	if err := wm.runner.Run(repoRoot, "git", "pull", "origin", baseBranch); err != nil {
		return errors.Wrap(err, "git pull failed")
	}

	return nil
}

// branchExists checks if a branch exists in the repository
func (wm *WorktreeManager) branchExists(repoRoot, branch string) bool {
	err := wm.runner.Run(repoRoot, "git", "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	return err == nil
}

// getFirstRemoteBranch gets the first available remote branch
func (wm *WorktreeManager) getFirstRemoteBranch(repoRoot string) (string, error) {
	output, err := wm.runner.Output(repoRoot, "git", "branch", "-r")
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

	return "", errors.New("no branches found")
}

// createInitialBranch creates an initial branch with empty commit
func (wm *WorktreeManager) createInitialBranch(repoRoot string) (string, error) {
	if wm.Verbose {
		fmt.Println("Creating initial commit on main branch...")
	}

	// Switch to main branch
	if err := wm.runner.Run(repoRoot, "git", "switch", "-c", "main"); err != nil {
		return "", errors.Wrap(err, "failed to create main branch")
	}

	// Create empty commit
	if err := wm.runner.Run(repoRoot, "git", "commit", "--allow-empty", "-m", "Initial commit"); err != nil {
		return "", errors.Wrap(err, "failed to create initial commit")
	}

	return "main", nil
}

// ListWorktrees returns a list of all existing worktrees
func (wm *WorktreeManager) ListWorktrees() ([]string, error) {
	repoRoot, err := wm.GetRepoRoot()
	if err != nil {
		return nil, err
	}

	output, err := wm.runner.Output(repoRoot, "git", "worktree", "list", "--porcelain")
	if err != nil {
		return nil, errors.Wrap(err, "failed to list worktrees")
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
	repoRoot, err := wm.GetRepoRoot()
	if err != nil {
		return err
	}

	worktreePath := filepath.Join(repoRoot, ticketType, ticket)

	// Validate path stays within repo root (prevent path traversal)
	if !strings.HasPrefix(worktreePath, repoRoot+string(filepath.Separator)) {
		return errors.New("invalid path: worktree path escapes repository root")
	}

	// Check if worktree exists
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		return errors.Newf("worktree does not exist: %s", worktreePath)
	}

	relativePath := filepath.Join(ticketType, ticket)
	return wm.runner.Run(repoRoot, "git", "worktree", "remove", relativePath)
}
