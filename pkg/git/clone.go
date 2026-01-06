package git

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cockroachdb/errors"
)

// RepoURL represents a parsed GitHub repository URL
type RepoURL struct {
	Original  string // Original input
	Canonical string // Normalized URL for cloning
	Protocol  string // "ssh" or "https"
	Owner     string // GitHub org/user
	Repo      string // Repository name (without .git)
}

// URL parsing patterns for GitHub repository URLs
var (
	// SSH format: git@github.com:owner/repo.git or git@github.com:owner/repo
	sshURLRegex = regexp.MustCompile(`^git@github\.com:([a-zA-Z0-9_.-]+)/([a-zA-Z0-9_.-]+?)(?:\.git)?$`)

	// HTTPS format: https://github.com/owner/repo or https://github.com/owner/repo.git
	httpsURLRegex = regexp.MustCompile(`^https://github\.com/([a-zA-Z0-9_.-]+)/([a-zA-Z0-9_.-]+?)(?:\.git)?$`)

	// Shorthand format: github.com/owner/repo (no protocol)
	shorthandURLRegex = regexp.MustCompile(`^github\.com/([a-zA-Z0-9_.-]+)/([a-zA-Z0-9_.-]+?)(?:\.git)?$`)
)

// ParseGitHubURL parses various GitHub URL formats and returns a normalized RepoURL.
// Supported formats:
//   - SSH: git@github.com:owner/repo.git
//   - HTTPS: https://github.com/owner/repo
//   - Shorthand: github.com/owner/repo (interpreted as SSH by default)
func ParseGitHubURL(input string) (*RepoURL, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, errors.New("empty URL provided")
	}

	// Try SSH format first
	if matches := sshURLRegex.FindStringSubmatch(input); len(matches) == 3 {
		return &RepoURL{
			Original:  input,
			Canonical: fmt.Sprintf("git@github.com:%s/%s.git", matches[1], matches[2]),
			Protocol:  "ssh",
			Owner:     matches[1],
			Repo:      matches[2],
		}, nil
	}

	// Try HTTPS format
	if matches := httpsURLRegex.FindStringSubmatch(input); len(matches) == 3 {
		return &RepoURL{
			Original:  input,
			Canonical: fmt.Sprintf("https://github.com/%s/%s.git", matches[1], matches[2]),
			Protocol:  "https",
			Owner:     matches[1],
			Repo:      matches[2],
		}, nil
	}

	// Try shorthand format (default to SSH)
	if matches := shorthandURLRegex.FindStringSubmatch(input); len(matches) == 3 {
		return &RepoURL{
			Original:  input,
			Canonical: fmt.Sprintf("git@github.com:%s/%s.git", matches[1], matches[2]),
			Protocol:  "ssh",
			Owner:     matches[1],
			Repo:      matches[2],
		}, nil
	}

	return nil, errors.Newf("invalid GitHub URL format: %q\n\nSupported formats:\n  git@github.com:owner/repo.git (SSH)\n  https://github.com/owner/repo (HTTPS)\n  github.com/owner/repo (shorthand)", input)
}

// CloneManager handles repository cloning operations
type CloneManager struct {
	BasePath string // Base path for clones (default: ~/src)
	Verbose  bool
	runner   CommandRunner
	homedir  func() (string, error) // For testing; defaults to os.UserHomeDir
}

// NewCloneManager creates a new CloneManager with default settings
func NewCloneManager(basePath string, verbose bool) *CloneManager {
	return &CloneManager{
		BasePath: basePath,
		Verbose:  verbose,
		runner:   &RealCommandRunner{Verbose: verbose},
		homedir:  os.UserHomeDir,
	}
}

// NewCloneManagerWithRunner creates a CloneManager with a custom CommandRunner (for testing)
func NewCloneManagerWithRunner(basePath string, verbose bool, runner CommandRunner) *CloneManager {
	return &CloneManager{
		BasePath: basePath,
		Verbose:  verbose,
		runner:   runner,
		homedir:  os.UserHomeDir,
	}
}

// Clone clones a repository to ~/src/<owner>/<repo> (or custom BasePath)
// For SSH URLs: bare clone + worktree setup
// For HTTPS URLs: standard git clone
// Returns the path to the cloned repository
func (cm *CloneManager) Clone(url *RepoURL) (string, error) {
	if url == nil {
		return "", errors.New("nil URL provided")
	}

	// Determine base path
	basePath := cm.BasePath
	if basePath == "" {
		home, err := cm.homedir()
		if err != nil {
			return "", errors.Wrap(err, "failed to get home directory")
		}
		basePath = filepath.Join(home, "src")
	}

	// Create target directory structure: basePath/<owner>/<repo>
	repoPath := filepath.Join(basePath, url.Owner, url.Repo)

	// Check if repository already exists
	if _, err := os.Stat(repoPath); err == nil {
		if cm.Verbose {
			fmt.Printf("Repository already exists at %s\n", repoPath)
		}
		return repoPath, nil
	}

	// Create parent directory (owner directory)
	ownerDir := filepath.Join(basePath, url.Owner)
	if err := os.MkdirAll(ownerDir, 0755); err != nil {
		return "", errors.Wrapf(err, "failed to create directory %s", ownerDir)
	}

	if url.Protocol == "ssh" {
		return cm.cloneSSH(url, repoPath)
	}
	return cm.cloneHTTPS(url, repoPath)
}

// cloneSSH performs a bare clone + worktree setup for SSH URLs
func (cm *CloneManager) cloneSSH(url *RepoURL, repoPath string) (string, error) {
	if cm.Verbose {
		fmt.Printf("Cloning (bare) %s to %s...\n", url.Canonical, repoPath)
	}

	// Clone as bare repository
	if err := cm.runner.Run("", "git", "clone", "--bare", url.Canonical, repoPath); err != nil {
		return "", errors.Wrapf(err, "git clone --bare failed for %s", url.Canonical)
	}

	// Configure fetch refspec for bare repos
	if err := cm.ensureFetchRefspec(repoPath); err != nil {
		// Log warning but continue - repo is still usable
		if cm.Verbose {
			fmt.Printf("Warning: could not configure fetch refspec: %v\n", err)
		}
	}

	// Fetch to populate remote-tracking branches
	if cm.Verbose {
		fmt.Println("Fetching remote branches...")
	}
	if err := cm.runner.Run(repoPath, "git", "fetch", "origin"); err != nil {
		if cm.Verbose {
			fmt.Printf("Warning: git fetch failed: %v\n", err)
		}
	}

	// Detect default branch
	defaultBranch, err := cm.detectDefaultBranch(repoPath)
	if err != nil {
		return "", errors.Wrap(err, "failed to detect default branch")
	}

	if cm.Verbose {
		fmt.Printf("Detected default branch: %s\n", defaultBranch)
	}

	// Create main worktree for the default branch
	worktreePath := filepath.Join(repoPath, defaultBranch)
	if cm.Verbose {
		fmt.Printf("Creating worktree for %s at %s...\n", defaultBranch, worktreePath)
	}

	if err := cm.runner.Run(repoPath, "git", "worktree", "add", defaultBranch, defaultBranch); err != nil {
		return "", errors.Wrapf(err, "failed to create worktree for %s", defaultBranch)
	}

	return repoPath, nil
}

// cloneHTTPS performs a standard git clone for HTTPS URLs
func (cm *CloneManager) cloneHTTPS(url *RepoURL, repoPath string) (string, error) {
	if cm.Verbose {
		fmt.Printf("Cloning %s to %s...\n", url.Canonical, repoPath)
	}

	if err := cm.runner.Run("", "git", "clone", url.Canonical, repoPath); err != nil {
		return "", errors.Wrapf(err, "git clone failed for %s", url.Canonical)
	}

	return repoPath, nil
}

// ensureFetchRefspec ensures the fetch refspec is configured for the origin remote.
// Bare repos created with `git clone --bare` don't have this configured by default.
func (cm *CloneManager) ensureFetchRefspec(repoPath string) error {
	// Check if fetch refspec already exists
	output, err := cm.runner.Output(repoPath, "git", "config", "--get", "remote.origin.fetch")
	if err == nil && len(strings.TrimSpace(string(output))) > 0 {
		// Already configured
		return nil
	}

	// Add the standard fetch refspec
	if cm.Verbose {
		fmt.Println("Adding fetch refspec for bare repository...")
	}

	if err := cm.runner.Run(repoPath, "git", "config", "remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*"); err != nil {
		return errors.Wrap(err, "failed to configure fetch refspec")
	}

	return nil
}

// detectDefaultBranch determines the default branch of the cloned repository.
// Priority: symbolic-ref HEAD > main > master > first remote branch
func (cm *CloneManager) detectDefaultBranch(repoPath string) (string, error) {
	// Try to get default branch from remote HEAD (symbolic-ref)
	output, err := cm.runner.Output(repoPath, "git", "symbolic-ref", "refs/remotes/origin/HEAD")
	if err == nil {
		ref := strings.TrimSpace(string(output))
		// Format: refs/remotes/origin/main -> main
		if strings.HasPrefix(ref, "refs/remotes/origin/") {
			branch := strings.TrimPrefix(ref, "refs/remotes/origin/")
			if cm.remoteBranchExists(repoPath, branch) {
				return branch, nil
			}
		}
	}

	// Fallback: check common default branch names
	for _, branch := range []string{"main", "master"} {
		if cm.remoteBranchExists(repoPath, branch) {
			return branch, nil
		}
	}

	// Last resort: get first remote branch
	return cm.getFirstRemoteBranch(repoPath)
}

// remoteBranchExists checks if a remote branch exists
func (cm *CloneManager) remoteBranchExists(repoPath, branch string) bool {
	err := cm.runner.Run(repoPath, "git", "show-ref", "--verify", "--quiet", "refs/remotes/origin/"+branch)
	return err == nil
}

// getFirstRemoteBranch returns the first available remote branch
func (cm *CloneManager) getFirstRemoteBranch(repoPath string) (string, error) {
	output, err := cm.runner.Output(repoPath, "git", "branch", "-r")
	if err != nil {
		return "", errors.Wrap(err, "failed to list remote branches")
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

	return "", errors.New("no remote branches found")
}
