package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewWorktreeManager(t *testing.T) {
	wm := NewWorktreeManager("/path/to/repo", "main", true)

	if wm.RepoPath != "/path/to/repo" {
		t.Errorf("RepoPath = %q, want %q", wm.RepoPath, "/path/to/repo")
	}
	if wm.BaseBranch != "main" {
		t.Errorf("BaseBranch = %q, want %q", wm.BaseBranch, "main")
	}
	if !wm.Verbose {
		t.Error("Verbose should be true")
	}
}

func TestRepoExists(t *testing.T) {
	// Create a temporary directory for tests
	tmpDir, err := os.MkdirTemp("", "git-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name     string
		setup    func(dir string) string
		expected bool
	}{
		{
			name: "nonexistent directory",
			setup: func(dir string) string {
				return filepath.Join(dir, "nonexistent")
			},
			expected: false,
		},
		{
			name: "empty directory (not a git repo)",
			setup: func(dir string) string {
				emptyDir := filepath.Join(dir, "empty")
				os.MkdirAll(emptyDir, 0755)
				return emptyDir
			},
			expected: false,
		},
		{
			name: "directory with .git folder",
			setup: func(dir string) string {
				repoDir := filepath.Join(dir, "repo-with-git")
				os.MkdirAll(filepath.Join(repoDir, ".git"), 0755)
				return repoDir
			},
			expected: true,
		},
		{
			name: "bare repository (has refs directory)",
			setup: func(dir string) string {
				bareDir := filepath.Join(dir, "bare-repo")
				os.MkdirAll(filepath.Join(bareDir, "refs"), 0755)
				return bareDir
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoPath := tt.setup(tmpDir)
			wm := NewWorktreeManager(repoPath, "main", false)
			result := wm.repoExists()
			if result != tt.expected {
				t.Errorf("repoExists() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestCreateWorktreeWithBranch_Integration tests actual git worktree creation
// This is an integration test that requires git to be installed
func TestCreateWorktreeWithBranch_Integration(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH, skipping integration test")
	}

	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "git-worktree-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize a git repository
	repoDir := filepath.Join(tmpDir, "repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("Failed to create repo dir: %v", err)
	}

	// git init
	cmd := exec.Command("git", "init")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}

	// Configure git user for the test repo
	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = repoDir
	cmd.Run()

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = repoDir
	cmd.Run()

	// Create an initial commit
	cmd = exec.Command("git", "commit", "--allow-empty", "-m", "Initial commit")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}

	// Test creating a worktree with custom branch name
	wm := NewWorktreeManager(repoDir, "main", false)

	// Test CreateWorktree (which delegates to CreateWorktreeWithBranch)
	worktreePath, err := wm.CreateWorktree("fraas", "FRAAS-123")
	if err != nil {
		t.Fatalf("CreateWorktree() error: %v", err)
	}

	expectedPath := filepath.Join(repoDir, "fraas", "FRAAS-123")
	if worktreePath != expectedPath {
		t.Errorf("CreateWorktree() = %q, want %q", worktreePath, expectedPath)
	}

	// Verify worktree was created
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		t.Errorf("Worktree directory not created at %q", worktreePath)
	}

	// Verify branch was created with the ticket name
	cmd = exec.Command("git", "branch", "--list", "FRAAS-123")
	cmd.Dir = repoDir
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("git branch --list failed: %v", err)
	}
	if len(output) == 0 {
		t.Error("Branch FRAAS-123 was not created")
	}
}

// TestCreateWorktreeWithBranch_CustomBranch tests custom branch naming
func TestCreateWorktreeWithBranch_CustomBranch(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH, skipping integration test")
	}

	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "git-worktree-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize a git repository
	repoDir := filepath.Join(tmpDir, "repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("Failed to create repo dir: %v", err)
	}

	// git init
	cmd := exec.Command("git", "init")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}

	// Configure git user for the test repo
	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = repoDir
	cmd.Run()

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = repoDir
	cmd.Run()

	// Create an initial commit
	cmd = exec.Command("git", "commit", "--allow-empty", "-m", "Initial commit")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}

	// Test creating a worktree with custom branch name (like hack command does)
	wm := NewWorktreeManager(repoDir, "main", false)

	// Create worktree with custom branch name: hack/winter-2025
	worktreePath, err := wm.CreateWorktreeWithBranch("hack", "winter-2025", "hack/winter-2025")
	if err != nil {
		t.Fatalf("CreateWorktreeWithBranch() error: %v", err)
	}

	expectedPath := filepath.Join(repoDir, "hack", "winter-2025")
	if worktreePath != expectedPath {
		t.Errorf("CreateWorktreeWithBranch() = %q, want %q", worktreePath, expectedPath)
	}

	// Verify worktree was created
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		t.Errorf("Worktree directory not created at %q", worktreePath)
	}

	// Verify branch was created with the custom name
	cmd = exec.Command("git", "branch", "--list", "hack/winter-2025")
	cmd.Dir = repoDir
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("git branch --list failed: %v", err)
	}
	if len(output) == 0 {
		t.Error("Branch hack/winter-2025 was not created")
	}
}

func TestCreateWorktree_ExistingWorktree(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH, skipping integration test")
	}

	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "git-worktree-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize a git repository
	repoDir := filepath.Join(tmpDir, "repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("Failed to create repo dir: %v", err)
	}

	// git init
	cmd := exec.Command("git", "init")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}

	// Configure git user for the test repo
	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = repoDir
	cmd.Run()

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = repoDir
	cmd.Run()

	// Create an initial commit
	cmd = exec.Command("git", "commit", "--allow-empty", "-m", "Initial commit")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}

	wm := NewWorktreeManager(repoDir, "main", false)

	// Create worktree first time
	path1, err := wm.CreateWorktree("fraas", "TEST-001")
	if err != nil {
		t.Fatalf("First CreateWorktree() error: %v", err)
	}

	// Create same worktree second time (should return existing path without error)
	path2, err := wm.CreateWorktree("fraas", "TEST-001")
	if err != nil {
		t.Fatalf("Second CreateWorktree() error: %v", err)
	}

	if path1 != path2 {
		t.Errorf("CreateWorktree() returned different paths: %q vs %q", path1, path2)
	}
}

func TestListWorktrees(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH, skipping integration test")
	}

	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "git-worktree-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize a git repository
	repoDir := filepath.Join(tmpDir, "repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("Failed to create repo dir: %v", err)
	}

	// git init
	cmd := exec.Command("git", "init")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}

	// Configure git user for the test repo
	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = repoDir
	cmd.Run()

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = repoDir
	cmd.Run()

	// Create an initial commit
	cmd = exec.Command("git", "commit", "--allow-empty", "-m", "Initial commit")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}

	wm := NewWorktreeManager(repoDir, "main", false)

	// List worktrees before creating any (should include main repo)
	worktrees, err := wm.ListWorktrees()
	if err != nil {
		t.Fatalf("ListWorktrees() error: %v", err)
	}

	// Should have at least the main repo
	if len(worktrees) < 1 {
		t.Error("ListWorktrees() should return at least the main repo")
	}

	// Create a worktree
	_, err = wm.CreateWorktree("fraas", "TEST-LIST")
	if err != nil {
		t.Fatalf("CreateWorktree() error: %v", err)
	}

	// List worktrees again
	worktrees, err = wm.ListWorktrees()
	if err != nil {
		t.Fatalf("ListWorktrees() error: %v", err)
	}

	// Should have main repo + new worktree
	if len(worktrees) < 2 {
		t.Errorf("ListWorktrees() returned %d worktrees, want at least 2", len(worktrees))
	}

	// Verify the new worktree is in the list
	// Note: On macOS, /var is a symlink to /private/var, so we need to compare
	// using the base name or resolve real paths
	expectedSuffix := filepath.Join("fraas", "TEST-LIST")
	found := false
	for _, wt := range worktrees {
		if strings.HasSuffix(wt, expectedSuffix) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ListWorktrees() missing expected worktree ending with %q in %v", expectedSuffix, worktrees)
	}
}

func TestRemoveWorktree(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH, skipping integration test")
	}

	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "git-worktree-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize a git repository
	repoDir := filepath.Join(tmpDir, "repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("Failed to create repo dir: %v", err)
	}

	// git init
	cmd := exec.Command("git", "init")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}

	// Configure git user for the test repo
	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = repoDir
	cmd.Run()

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = repoDir
	cmd.Run()

	// Create an initial commit
	cmd = exec.Command("git", "commit", "--allow-empty", "-m", "Initial commit")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}

	wm := NewWorktreeManager(repoDir, "main", false)

	// Create a worktree
	worktreePath, err := wm.CreateWorktree("fraas", "TEST-REMOVE")
	if err != nil {
		t.Fatalf("CreateWorktree() error: %v", err)
	}

	// Verify it exists
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		t.Fatalf("Worktree was not created at %q", worktreePath)
	}

	// Remove the worktree
	err = wm.RemoveWorktree("fraas", "TEST-REMOVE")
	if err != nil {
		t.Fatalf("RemoveWorktree() error: %v", err)
	}

	// Verify it's gone
	if _, err := os.Stat(worktreePath); !os.IsNotExist(err) {
		t.Errorf("Worktree still exists at %q after removal", worktreePath)
	}
}

func TestRemoveWorktree_NonExistent(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "git-worktree-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	wm := NewWorktreeManager(tmpDir, "main", false)

	// Try to remove non-existent worktree
	err = wm.RemoveWorktree("fraas", "NONEXISTENT")
	if err == nil {
		t.Error("RemoveWorktree() expected error for non-existent worktree, got nil")
	}
}
