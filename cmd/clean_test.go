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
	tmpDir := t.TempDir()

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
	configEmail := exec.Command("git", "-C", repoDir, "config", "user.email", "test@example.com")
	_ = configEmail.Run()
	configName := exec.Command("git", "-C", repoDir, "config", "user.name", "Test User")
	_ = configName.Run()

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
	_ = cmd.Run() // Might fail if default branch is master
	cmd = exec.Command("git", "checkout", "master")
	cmd.Dir = repoDir
	_ = cmd.Run()

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
	tmpDir := t.TempDir()

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
	configEmail := exec.Command("git", "-C", repoDir, "config", "user.email", "test@example.com")
	_ = configEmail.Run()
	configName := exec.Command("git", "-C", repoDir, "config", "user.name", "Test User")
	_ = configName.Run()

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
	tmpDir := t.TempDir()

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
	configEmail := exec.Command("git", "-C", repoDir, "config", "user.email", "test@example.com")
	_ = configEmail.Run()
	configName := exec.Command("git", "-C", repoDir, "config", "user.name", "Test User")
	_ = configName.Run()

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

func TestCleanCommandFlags(t *testing.T) {
	cmd := cleanCmd

	// Check --dry-run flag exists
	dryRunFlag := cmd.Flags().Lookup("dry-run")
	if dryRunFlag == nil {
		t.Error("clean command should have --dry-run flag")
	}
	if dryRunFlag != nil && dryRunFlag.DefValue != "false" {
		t.Errorf("--dry-run default should be false, got %s", dryRunFlag.DefValue)
	}

	// Check --force flag exists
	forceFlag := cmd.Flags().Lookup("force")
	if forceFlag == nil {
		t.Error("clean command should have --force flag")
	}
	if forceFlag != nil && forceFlag.DefValue != "false" {
		t.Errorf("--force default should be false, got %s", forceFlag.DefValue)
	}
}

func TestCleanCommandDescription(t *testing.T) {
	cmd := cleanCmd

	if cmd.Use != "clean" {
		t.Errorf("clean command Use = %q, want %q", cmd.Use, "clean")
	}

	if cmd.Short == "" {
		t.Error("clean command should have Short description")
	}

	if cmd.Long == "" {
		t.Error("clean command should have Long description")
	}
}

func TestForceRemoveWorktree(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH, skipping test")
	}

	// Create a temporary git repository
	tmpDir := t.TempDir()

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
	configEmail := exec.Command("git", "-C", repoDir, "config", "user.email", "test@example.com")
	_ = configEmail.Run()
	configName := exec.Command("git", "-C", repoDir, "config", "user.name", "Test User")
	_ = configName.Run()

	// Create initial commit
	cmd = exec.Command("git", "commit", "--allow-empty", "-m", "Initial commit")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}

	// Create a worktree
	worktreePath := filepath.Join(tmpDir, "worktree")
	cmd = exec.Command("git", "worktree", "add", "-b", "test-branch", worktreePath)
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git worktree add failed: %v", err)
	}

	// Verify worktree exists
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		t.Fatalf("Worktree not created at %s", worktreePath)
	}

	// Force remove the worktree
	if err := forceRemoveWorktree(repoDir, worktreePath); err != nil {
		t.Fatalf("forceRemoveWorktree() error: %v", err)
	}

	// Verify worktree is removed
	if _, err := os.Stat(worktreePath); !os.IsNotExist(err) {
		t.Error("Worktree should be removed after forceRemoveWorktree()")
	}
}

func TestForceRemoveWorktree_NonExistent(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH, skipping test")
	}

	// Create a temporary git repository
	tmpDir := t.TempDir()

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
	configEmail := exec.Command("git", "-C", repoDir, "config", "user.email", "test@example.com")
	_ = configEmail.Run()
	configName := exec.Command("git", "-C", repoDir, "config", "user.name", "Test User")
	_ = configName.Run()

	// Create initial commit
	cmd = exec.Command("git", "commit", "--allow-empty", "-m", "Initial commit")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}

	// Try to remove non-existent worktree
	// Should return an error for non-existent worktree
	if err := forceRemoveWorktree(repoDir, "/nonexistent/worktree/path"); err == nil {
		t.Error("forceRemoveWorktree() should error for non-existent worktree")
	}
}

func TestGetWorktreeDetailsForClean_WithWorktree(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH, skipping test")
	}

	// Create a temporary git repository
	tmpDir := t.TempDir()

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
	configEmail := exec.Command("git", "-C", repoDir, "config", "user.email", "test@example.com")
	_ = configEmail.Run()
	configName := exec.Command("git", "-C", repoDir, "config", "user.name", "Test User")
	_ = configName.Run()

	// Create initial commit
	cmd = exec.Command("git", "commit", "--allow-empty", "-m", "Initial commit")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}

	// Create a worktree
	worktreePath := filepath.Join(tmpDir, "fraas", "FRAAS-123")
	if err := os.MkdirAll(filepath.Dir(worktreePath), 0755); err != nil {
		t.Fatalf("Failed to create parent dir: %v", err)
	}

	cmd = exec.Command("git", "worktree", "add", "-b", "FRAAS-123", worktreePath)
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git worktree add failed: %v", err)
	}

	// Get worktree details
	details := getWorktreeDetailsForClean(repoDir)

	// Should have 2 worktrees (main + feature)
	if len(details) < 2 {
		t.Errorf("getWorktreeDetailsForClean() returned %d worktrees, expected at least 2", len(details))
	}

	// Find the feature worktree and check its branch
	found := false
	for path, info := range details {
		realPath, _ := filepath.EvalSymlinks(path)
		realWorktreePath, _ := filepath.EvalSymlinks(worktreePath)
		if path == worktreePath || realPath == realWorktreePath {
			found = true
			if info.Branch != "FRAAS-123" {
				t.Errorf("Branch = %q, want %q", info.Branch, "FRAAS-123")
			}
			break
		}
	}
	if !found {
		t.Errorf("getWorktreeDetailsForClean() missing feature worktree path %q", worktreePath)
	}
}

func TestCleanupCandidateStatusString(t *testing.T) {
	// Test the status string building logic from runCleanCommand
	tests := []struct {
		name       string
		isMerged   bool
		hasSession bool
		expected   string
	}{
		{
			name:       "merged with session",
			isMerged:   true,
			hasSession: true,
			expected:   " [merged] [has session]",
		},
		{
			name:       "merged without session",
			isMerged:   true,
			hasSession: false,
			expected:   " [merged]",
		},
		{
			name:       "not merged with session",
			isMerged:   false,
			hasSession: true,
			expected:   " [has session]",
		},
		{
			name:       "not merged without session",
			isMerged:   false,
			hasSession: false,
			expected:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the status string building from runCleanCommand
			status := ""
			if tt.isMerged {
				status = " [merged]"
			}
			if tt.hasSession {
				status += " [has session]"
			}

			if status != tt.expected {
				t.Errorf("Status string = %q, want %q", status, tt.expected)
			}
		})
	}
}
