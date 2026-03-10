package vcs

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestLocalProvider_GetRepoRoot(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH, skipping test")
	}

	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("Failed to create repo dir: %v", err)
	}

	// Initialize git repo
	runGit(t, repoDir, "init")

	// Configure git user for tests
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	runGit(t, repoDir, "config", "user.name", "Test User")
	runGit(t, repoDir, "config", "commit.gpgsign", "false")

	runGit(t, repoDir, "commit", "--allow-empty", "-m", "Initial commit")

	provider := NewLocalProvider(false)
	root, err := provider.GetRepoRoot(repoDir)
	if err != nil {
		t.Fatalf("GetRepoRoot failed: %v", err)
	}

	realRoot, _ := filepath.EvalSymlinks(root)
	realRepoDir, _ := filepath.EvalSymlinks(repoDir)
	expectedRoot := realRepoDir
	if realRoot != expectedRoot {
		t.Errorf("GetRepoRoot = %q, want %q", realRoot, expectedRoot)
	}
}

func TestLocalProvider_ListWorktrees(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH, skipping test")
	}

	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	_ = os.MkdirAll(repoDir, 0755)
	runGit(t, repoDir, "init")

	runGit(t, repoDir, "config", "user.email", "test@example.com")
	runGit(t, repoDir, "config", "user.name", "Test User")
	runGit(t, repoDir, "config", "commit.gpgsign", "false")

	runGit(t, repoDir, "commit", "--allow-empty", "-m", "Initial commit")

	provider := NewLocalProvider(false)
	worktrees, err := provider.ListWorktrees(repoDir)
	if err != nil {
		t.Fatalf("ListWorktrees failed: %v", err)
	}

	if len(worktrees) == 0 {
		t.Error("Expected at least 1 worktree (main)")
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git %v failed: %v", args, err)
	}
}
