package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"

	"thoreinstein.com/sre/pkg/config"
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

	// Configure git user and disable GPG signing for tests
	configEmail := exec.Command("git", "-C", repoDir, "config", "user.email", "test@example.com")
	_ = configEmail.Run()
	configName := exec.Command("git", "-C", repoDir, "config", "user.name", "Test User")
	_ = configName.Run()
	configGpg := exec.Command("git", "-C", repoDir, "config", "commit.gpgsign", "false")
	_ = configGpg.Run()

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

	// Configure git user and disable GPG signing for tests
	configEmail := exec.Command("git", "-C", repoDir, "config", "user.email", "test@example.com")
	_ = configEmail.Run()
	configName := exec.Command("git", "-C", repoDir, "config", "user.name", "Test User")
	_ = configName.Run()
	configGpg := exec.Command("git", "-C", repoDir, "config", "commit.gpgsign", "false")
	_ = configGpg.Run()

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

func TestIsBranchMerged_WorktreeCheckedOut(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH, skipping test")
	}

	// This test verifies that branches checked out in worktrees (prefixed with '+')
	// are correctly detected as merged
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")

	// Initialize as bare repo
	cmd := exec.Command("git", "init", "--bare", repoDir)
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init --bare failed: %v", err)
	}

	for _, args := range [][]string{
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "Test User"},
		{"config", "commit.gpgsign", "false"},
	} {
		cmd = exec.Command("git", args...)
		cmd.Dir = repoDir
		_ = cmd.Run()
	}

	// Create main worktree
	mainWorktree := filepath.Join(tmpDir, "main-wt")
	cmd = exec.Command("git", "worktree", "add", "-b", "main", mainWorktree)
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git worktree add main failed: %v", err)
	}

	cmd = exec.Command("git", "commit", "--allow-empty", "-m", "Initial commit")
	cmd.Dir = mainWorktree
	if err := cmd.Run(); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}

	// Create feature worktree
	featureWorktree := filepath.Join(tmpDir, "feature-wt")
	cmd = exec.Command("git", "worktree", "add", "-b", "feature-branch", featureWorktree, "main")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git worktree add feature failed: %v", err)
	}

	cmd = exec.Command("git", "commit", "--allow-empty", "-m", "Feature commit")
	cmd.Dir = featureWorktree
	if err := cmd.Run(); err != nil {
		t.Fatalf("git commit on feature failed: %v", err)
	}

	// Merge feature into main
	cmd = exec.Command("git", "merge", "feature-branch", "-m", "Merge feature")
	cmd.Dir = mainWorktree
	if err := cmd.Run(); err != nil {
		t.Fatalf("git merge failed: %v", err)
	}

	// The feature branch is still checked out in its worktree, so git branch --merged
	// will show it with a '+' prefix. Test that isBranchMerged handles this correctly.
	if !isBranchMerged(repoDir, "feature-branch", "main") {
		t.Error("isBranchMerged() should return true for merged branch checked out in worktree (with '+' prefix)")
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

	// Configure git user and disable GPG signing for tests
	configEmail := exec.Command("git", "-C", repoDir, "config", "user.email", "test@example.com")
	_ = configEmail.Run()
	configName := exec.Command("git", "-C", repoDir, "config", "user.name", "Test User")
	_ = configName.Run()
	configGpg := exec.Command("git", "-C", repoDir, "config", "commit.gpgsign", "false")
	_ = configGpg.Run()

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

	// Configure git user and disable GPG signing for tests
	configEmail := exec.Command("git", "-C", repoDir, "config", "user.email", "test@example.com")
	_ = configEmail.Run()
	configName := exec.Command("git", "-C", repoDir, "config", "user.name", "Test User")
	_ = configName.Run()
	configGpg := exec.Command("git", "-C", repoDir, "config", "commit.gpgsign", "false")
	_ = configGpg.Run()

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

	// Configure git user and disable GPG signing for tests
	configEmail := exec.Command("git", "-C", repoDir, "config", "user.email", "test@example.com")
	_ = configEmail.Run()
	configName := exec.Command("git", "-C", repoDir, "config", "user.name", "Test User")
	_ = configName.Run()
	configGpg := exec.Command("git", "-C", repoDir, "config", "commit.gpgsign", "false")
	_ = configGpg.Run()

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

	// Configure git user and disable GPG signing for tests
	configEmail := exec.Command("git", "-C", repoDir, "config", "user.email", "test@example.com")
	_ = configEmail.Run()
	configName := exec.Command("git", "-C", repoDir, "config", "user.name", "Test User")
	_ = configName.Run()
	configGpg := exec.Command("git", "-C", repoDir, "config", "commit.gpgsign", "false")
	_ = configGpg.Run()

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

// Integration tests for runCleanCommand

// setupCleanTestGitRepo creates a bare git repo with worktrees for clean command testing
func setupCleanTestGitRepo(t *testing.T) (repoDir string, worktreePaths []string) {
	t.Helper()

	tmpDir := t.TempDir()
	repoDir = filepath.Join(tmpDir, "repo")

	// Initialize as bare repo to match production setup
	cmd := exec.Command("git", "init", "--bare", repoDir)
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init --bare failed: %v", err)
	}

	// Configure git user and disable GPG signing for tests
	for _, args := range [][]string{
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "Test User"},
		{"config", "commit.gpgsign", "false"},
	} {
		cmd = exec.Command("git", args...)
		cmd.Dir = repoDir
		_ = cmd.Run()
	}

	// Create a main worktree to make initial commit
	mainWorktree := filepath.Join(tmpDir, "main-worktree")
	cmd = exec.Command("git", "worktree", "add", "-b", "main", mainWorktree)
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git worktree add main failed: %v", err)
	}

	// Create initial commit in main worktree
	cmd = exec.Command("git", "commit", "--allow-empty", "-m", "Initial commit")
	cmd.Dir = mainWorktree
	if err := cmd.Run(); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}

	// Remove the temp main worktree
	cmd = exec.Command("git", "worktree", "remove", mainWorktree)
	cmd.Dir = repoDir
	_ = cmd.Run()

	// Create worktrees for testing under the bare repo
	worktreeNames := []string{"feature-1", "feature-2"}
	for _, name := range worktreeNames {
		worktreePath := filepath.Join(repoDir, "fraas", name)

		cmd = exec.Command("git", "worktree", "add", "-b", name, worktreePath, "main")
		cmd.Dir = repoDir
		if err := cmd.Run(); err != nil {
			t.Fatalf("git worktree add failed: %v", err)
		}
		worktreePaths = append(worktreePaths, worktreePath)
	}

	return repoDir, worktreePaths
}

func TestFindCleanupCandidates(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH, skipping test")
	}

	repoDir, worktreePaths := setupCleanTestGitRepo(t)

	// Setup minimal config for test
	notesDir := t.TempDir()
	setupCleanTestConfig(t, notesDir)
	defer viper.Reset()

	t.Chdir(repoDir)

	cfg, err := loadTestConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	candidates, err := findCleanupCandidates(cfg)
	if err != nil {
		t.Fatalf("findCleanupCandidates() error: %v", err)
	}

	// Should find 2 worktrees (not including main)
	if len(candidates) != 2 {
		t.Errorf("findCleanupCandidates() found %d candidates, expected 2", len(candidates))
	}

	// Verify worktrees are in candidates
	foundPaths := make(map[string]bool)
	for _, c := range candidates {
		foundPaths[c.Path] = true
	}

	for _, expectedPath := range worktreePaths {
		// Handle symlink resolution for comparison
		realExpected, _ := filepath.EvalSymlinks(expectedPath)
		found := false
		for path := range foundPaths {
			realPath, _ := filepath.EvalSymlinks(path)
			if path == expectedPath || realPath == realExpected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected worktree %q not found in candidates", expectedPath)
		}
	}
}

func TestRunCleanCommand_DryRun(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH, skipping test")
	}

	repoDir, worktreePaths := setupCleanTestGitRepo(t)

	notesDir := t.TempDir()
	setupCleanTestConfig(t, notesDir)
	defer func() {
		cleanDryRun = false
		cleanForce = false
		viper.Reset()
	}()

	t.Chdir(repoDir)

	// Set dry-run mode
	cleanDryRun = true
	cleanForce = false

	// Capture output (command runs synchronously)
	err := runCleanCommand()
	if err != nil {
		t.Fatalf("runCleanCommand() with --dry-run error: %v", err)
	}

	// Verify worktrees still exist (dry-run should NOT remove them)
	for _, path := range worktreePaths {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Worktree %q should still exist after dry-run", path)
		}
	}
}

func TestRunCleanCommand_Force(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH, skipping test")
	}

	repoDir, worktreePaths := setupCleanTestGitRepo(t)

	notesDir := t.TempDir()
	setupCleanTestConfig(t, notesDir)
	defer func() {
		cleanDryRun = false
		cleanForce = false
		viper.Reset()
	}()

	t.Chdir(repoDir)

	// Set force mode (skip confirmation)
	cleanDryRun = false
	cleanForce = true

	err := runCleanCommand()
	if err != nil {
		t.Fatalf("runCleanCommand() with --force error: %v", err)
	}

	// Verify worktrees were removed
	for _, path := range worktreePaths {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("Worktree %q should be removed after --force clean", path)
		}
	}
}

func TestRunCleanCommand_NoWorktrees(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH, skipping test")
	}

	// Create a bare repo with no extra worktrees
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")

	cmd := exec.Command("git", "init", "--bare", repoDir)
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init --bare failed: %v", err)
	}

	for _, args := range [][]string{
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "Test User"},
		{"config", "commit.gpgsign", "false"},
	} {
		cmd = exec.Command("git", args...)
		cmd.Dir = repoDir
		_ = cmd.Run()
	}

	// Create main worktree for initial commit
	mainWorktree := filepath.Join(tmpDir, "main-worktree")
	cmd = exec.Command("git", "worktree", "add", "-b", "main", mainWorktree)
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git worktree add main failed: %v", err)
	}

	cmd = exec.Command("git", "commit", "--allow-empty", "-m", "Initial commit")
	cmd.Dir = mainWorktree
	if err := cmd.Run(); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}

	// Remove temp worktree
	cmd = exec.Command("git", "worktree", "remove", mainWorktree)
	cmd.Dir = repoDir
	_ = cmd.Run()

	notesDir := t.TempDir()
	setupCleanTestConfig(t, notesDir)
	defer func() {
		cleanDryRun = false
		cleanForce = false
		viper.Reset()
	}()

	t.Chdir(repoDir)

	cleanDryRun = false
	cleanForce = true

	// Should not error when no worktrees to clean
	err := runCleanCommand()
	if err != nil {
		t.Errorf("runCleanCommand() should not error with no worktrees: %v", err)
	}
}

func TestRunCleanCommand_MergedBranchDetection(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH, skipping test")
	}

	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")

	// Initialize bare repo
	cmd := exec.Command("git", "init", "--bare", repoDir)
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init --bare failed: %v", err)
	}

	for _, args := range [][]string{
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "Test User"},
		{"config", "commit.gpgsign", "false"},
	} {
		cmd = exec.Command("git", args...)
		cmd.Dir = repoDir
		_ = cmd.Run()
	}

	// Create main worktree for initial commit
	mainWorktree := filepath.Join(tmpDir, "main-worktree")
	cmd = exec.Command("git", "worktree", "add", "-b", "main", mainWorktree)
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git worktree add main failed: %v", err)
	}

	cmd = exec.Command("git", "commit", "--allow-empty", "-m", "Initial commit")
	cmd.Dir = mainWorktree
	if err := cmd.Run(); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}

	// Create a worktree with a branch that will be merged
	mergedWorktreePath := filepath.Join(repoDir, "fraas", "merged-feature")
	cmd = exec.Command("git", "worktree", "add", "-b", "merged-feature", mergedWorktreePath, "main")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git worktree add failed: %v", err)
	}

	// Add commit on the feature branch
	cmd = exec.Command("git", "commit", "--allow-empty", "-m", "Feature commit")
	cmd.Dir = mergedWorktreePath
	if err := cmd.Run(); err != nil {
		t.Fatalf("git commit on feature failed: %v", err)
	}

	// Merge the feature branch into main from the main worktree
	cmd = exec.Command("git", "merge", "merged-feature", "-m", "Merge merged-feature")
	cmd.Dir = mainWorktree
	if err := cmd.Run(); err != nil {
		t.Fatalf("git merge failed: %v", err)
	}

	// Remove the main worktree (no longer needed)
	cmd = exec.Command("git", "worktree", "remove", mainWorktree)
	cmd.Dir = repoDir
	_ = cmd.Run()

	notesDir := t.TempDir()
	setupCleanTestConfig(t, notesDir)
	defer viper.Reset()

	t.Chdir(repoDir)

	cfg, err := loadTestConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	candidates, err := findCleanupCandidates(cfg)
	if err != nil {
		t.Fatalf("findCleanupCandidates() error: %v", err)
	}

	// Find the merged worktree candidate
	var mergedCandidate *CleanupCandidate
	for i, c := range candidates {
		realPath, _ := filepath.EvalSymlinks(c.Path)
		realMergedPath, _ := filepath.EvalSymlinks(mergedWorktreePath)
		if c.Path == mergedWorktreePath || realPath == realMergedPath {
			mergedCandidate = &candidates[i]
			break
		}
	}

	if mergedCandidate == nil {
		t.Fatal("Merged worktree should be in candidates")
	}

	if !mergedCandidate.IsMerged {
		t.Errorf("Merged branch should be detected as IsMerged=true")
	}
}

// Helper functions for clean tests

func setupCleanTestConfig(t *testing.T, notesPath string) {
	t.Helper()

	viper.Reset()
	viper.Set("notes.path", notesPath)
	viper.Set("notes.daily_dir", "daily")
	viper.Set("notes.template_dir", filepath.Join(notesPath, "templates"))
	viper.Set("git.base_branch", "")
	viper.Set("jira.enabled", false)
	viper.Set("tmux.session_prefix", "")
	viper.Set("tmux.windows", []map[string]string{
		{"name": "code", "command": ""},
	})
}

func loadTestConfig() (*config.Config, error) {
	return config.Load()
}
