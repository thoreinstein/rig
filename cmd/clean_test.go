package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestIsBranchMerged(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH, skipping test")
	}

	// Create a temporary git repository
	tmpDir, err := os.MkdirTemp("", "clean-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	repoDir := filepath.Join(tmpDir, "repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("Failed to create repo dir: %v", err)
	}

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}

	// Configure git user
	exec.Command("git", "config", "user.email", "test@example.com").Run()
	exec.Command("git", "config", "user.name", "Test User").Run()

	// Create initial commit on main
	cmd = exec.Command("git", "commit", "--allow-empty", "-m", "Initial commit")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}

	// Create and checkout a feature branch
	cmd = exec.Command("git", "checkout", "-b", "feature-branch")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git checkout -b failed: %v", err)
	}

	// Add a commit to feature branch
	cmd = exec.Command("git", "commit", "--allow-empty", "-m", "Feature commit")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git commit on feature failed: %v", err)
	}

	// Go back to main
	cmd = exec.Command("git", "checkout", "main")
	cmd.Dir = repoDir
	cmd.Run() // Might fail if default branch is master
	cmd = exec.Command("git", "checkout", "master")
	cmd.Dir = repoDir
	cmd.Run()

	// Get current branch name
	cmd = exec.Command("git", "branch", "--show-current")
	cmd.Dir = repoDir
	output, _ := cmd.Output()
	baseBranch := string(output)
	if baseBranch == "" {
		baseBranch = "main" // default
	} else {
		baseBranch = baseBranch[:len(baseBranch)-1] // trim newline
	}

	tests := []struct {
		name       string
		branch     string
		baseBranch string
		setup      func()
		expected   bool
	}{
		{
			name:       "unmerged branch",
			branch:     "feature-branch",
			baseBranch: baseBranch,
			expected:   false,
		},
		{
			name:       "empty branch name",
			branch:     "",
			baseBranch: baseBranch,
			expected:   false,
		},
		{
			name:       "same as base branch",
			branch:     baseBranch,
			baseBranch: baseBranch,
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup()
			}
			result := isBranchMerged(repoDir, tt.branch, tt.baseBranch)
			if result != tt.expected {
				t.Errorf("isBranchMerged(%q, %q) = %v, want %v", tt.branch, tt.baseBranch, result, tt.expected)
			}
		})
	}
}

func TestIsBranchMerged_MergedBranch(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH, skipping test")
	}

	// Create a temporary git repository
	tmpDir, err := os.MkdirTemp("", "clean-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	repoDir := filepath.Join(tmpDir, "repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("Failed to create repo dir: %v", err)
	}

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}

	// Configure git user
	cmd = exec.Command("git", "-C", repoDir, "config", "user.email", "test@example.com")
	cmd.Run()
	cmd = exec.Command("git", "-C", repoDir, "config", "user.name", "Test User")
	cmd.Run()

	// Create initial commit
	cmd = exec.Command("git", "commit", "--allow-empty", "-m", "Initial commit")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}

	// Get current branch
	cmd = exec.Command("git", "branch", "--show-current")
	cmd.Dir = repoDir
	output, _ := cmd.Output()
	baseBranch := "main"
	if len(output) > 0 {
		baseBranch = string(output[:len(output)-1])
	}

	// Create and checkout a feature branch
	cmd = exec.Command("git", "checkout", "-b", "merged-feature")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git checkout -b failed: %v", err)
	}

	// Add a commit to feature branch
	cmd = exec.Command("git", "commit", "--allow-empty", "-m", "Feature commit")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git commit on feature failed: %v", err)
	}

	// Go back to base and merge
	cmd = exec.Command("git", "checkout", baseBranch)
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git checkout base failed: %v", err)
	}

	cmd = exec.Command("git", "merge", "merged-feature", "--no-ff", "-m", "Merge feature")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git merge failed: %v", err)
	}

	// Now test if branch is detected as merged
	if !isBranchMerged(repoDir, "merged-feature", baseBranch) {
		t.Error("isBranchMerged() should return true for merged branch")
	}
}

func TestGetWorktreeDetailsForClean(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH, skipping test")
	}

	// Create a temporary git repository
	tmpDir, err := os.MkdirTemp("", "clean-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	repoDir := filepath.Join(tmpDir, "repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("Failed to create repo dir: %v", err)
	}

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}

	// Configure git user
	cmd = exec.Command("git", "-C", repoDir, "config", "user.email", "test@example.com")
	cmd.Run()
	cmd = exec.Command("git", "-C", repoDir, "config", "user.name", "Test User")
	cmd.Run()

	// Create initial commit
	cmd = exec.Command("git", "commit", "--allow-empty", "-m", "Initial commit")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}

	// Get worktree details (should have main repo)
	details := getWorktreeDetailsForClean(repoDir)

	// Should have at least the main worktree
	if len(details) == 0 {
		t.Error("getWorktreeDetailsForClean() returned empty map, expected at least main worktree")
	}

	// Check that the main repo path exists in details
	// Note: On macOS, /var is a symlink to /private/var, so paths might differ
	found := false
	for path := range details {
		// Compare using EvalSymlinks to handle symlink differences
		realPath, _ := filepath.EvalSymlinks(path)
		realRepoDir, _ := filepath.EvalSymlinks(repoDir)
		if path == repoDir || realPath == realRepoDir {
			found = true
			break
		}
	}
	if !found {
		t.Logf("Details keys: %v", details)
		t.Errorf("getWorktreeDetailsForClean() missing main repo path %q", repoDir)
	}
}

func TestCleanupCandidate(t *testing.T) {
	// Test the CleanupCandidate struct
	candidate := CleanupCandidate{
		Path:       "/home/user/repo/fraas/FRAAS-123",
		Branch:     "FRAAS-123",
		RepoName:   "main",
		RepoPath:   "/home/user/repo",
		IsMerged:   true,
		HasSession: true,
	}

	if candidate.Path != "/home/user/repo/fraas/FRAAS-123" {
		t.Errorf("Path = %q, want %q", candidate.Path, "/home/user/repo/fraas/FRAAS-123")
	}
	if candidate.Branch != "FRAAS-123" {
		t.Errorf("Branch = %q, want %q", candidate.Branch, "FRAAS-123")
	}
	if !candidate.IsMerged {
		t.Error("IsMerged should be true")
	}
	if !candidate.HasSession {
		t.Error("HasSession should be true")
	}
}
