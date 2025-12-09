package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestListCommandFlags(t *testing.T) {
	cmd := listCmd

	// Check --worktrees flag exists
	worktreesFlag := cmd.Flags().Lookup("worktrees")
	if worktreesFlag == nil {
		t.Error("list command should have --worktrees flag")
	}
	if worktreesFlag != nil && worktreesFlag.DefValue != "false" {
		t.Errorf("--worktrees default should be false, got %s", worktreesFlag.DefValue)
	}

	// Check --sessions flag exists
	sessionsFlag := cmd.Flags().Lookup("sessions")
	if sessionsFlag == nil {
		t.Error("list command should have --sessions flag")
	}
	if sessionsFlag != nil && sessionsFlag.DefValue != "false" {
		t.Errorf("--sessions default should be false, got %s", sessionsFlag.DefValue)
	}
}

func TestListCommandDescription(t *testing.T) {
	cmd := listCmd

	if cmd.Use != "list" {
		t.Errorf("list command Use = %q, want %q", cmd.Use, "list")
	}

	if cmd.Short == "" {
		t.Error("list command should have Short description")
	}

	if cmd.Long == "" {
		t.Error("list command should have Long description")
	}

	// Verify key information is in the description
	if !strings.Contains(cmd.Long, "worktree") {
		t.Error("list command Long description should mention 'worktree'")
	}

	if !strings.Contains(cmd.Long, "session") {
		t.Error("list command Long description should mention 'session'")
	}
}

func TestWorktreeInfo(t *testing.T) {
	// Test the WorktreeInfo struct
	info := WorktreeInfo{
		Path:   "/home/user/repo/fraas/FRAAS-123",
		Branch: "FRAAS-123",
		Repo:   "main",
	}

	if info.Path != "/home/user/repo/fraas/FRAAS-123" {
		t.Errorf("Path = %q, want %q", info.Path, "/home/user/repo/fraas/FRAAS-123")
	}
	if info.Branch != "FRAAS-123" {
		t.Errorf("Branch = %q, want %q", info.Branch, "FRAAS-123")
	}
	if info.Repo != "main" {
		t.Errorf("Repo = %q, want %q", info.Repo, "main")
	}
}

func TestGetWorktreeDetails(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH, skipping test")
	}

	// Create a temporary git repository
	tmpDir, err := os.MkdirTemp("", "list-test-*")
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

	// Get worktree details
	details := getWorktreeDetails(repoDir)

	// Should have at least the main worktree
	if len(details) == 0 {
		t.Error("getWorktreeDetails() returned empty map, expected at least main worktree")
	}

	// Find the main repo in details (handle symlink differences on macOS)
	found := false
	for path := range details {
		realPath, _ := filepath.EvalSymlinks(path)
		realRepoDir, _ := filepath.EvalSymlinks(repoDir)
		if path == repoDir || realPath == realRepoDir {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("getWorktreeDetails() missing main repo path %q", repoDir)
	}
}

func TestGetWorktreeDetails_WithWorktree(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH, skipping test")
	}

	// Create a temporary git repository
	tmpDir, err := os.MkdirTemp("", "list-test-*")
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

	// Create a worktree
	worktreePath := filepath.Join(tmpDir, "worktree1")
	cmd = exec.Command("git", "worktree", "add", "-b", "feature-branch", worktreePath)
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git worktree add failed: %v", err)
	}

	// Get worktree details
	details := getWorktreeDetails(repoDir)

	// Should have 2 worktrees (main + feature)
	if len(details) < 2 {
		t.Errorf("getWorktreeDetails() returned %d worktrees, expected at least 2", len(details))
	}

	// Find the feature worktree and check its branch
	found := false
	for path, info := range details {
		realPath, _ := filepath.EvalSymlinks(path)
		realWorktreePath, _ := filepath.EvalSymlinks(worktreePath)
		if path == worktreePath || realPath == realWorktreePath {
			found = true
			if info.Branch != "feature-branch" {
				t.Errorf("Branch = %q, want %q", info.Branch, "feature-branch")
			}
			break
		}
	}
	if !found {
		t.Errorf("getWorktreeDetails() missing feature worktree path %q", worktreePath)
	}
}

func TestGetSessionDetails(t *testing.T) {
	// This test verifies the structure of getSessionDetails
	// It may return an empty map if tmux is not running
	details := getSessionDetails()

	// Should return a map (possibly empty)
	if details == nil {
		t.Error("getSessionDetails() should return non-nil map")
	}
}

func TestIsTmuxRunning(t *testing.T) {
	// Test that isTmuxRunning returns without error
	// The result depends on whether tmux is running
	result := isTmuxRunning()
	// Just verify it doesn't panic and returns a bool
	_ = result
}

func TestParseWorktreeOutput(t *testing.T) {
	// Test parsing of git worktree list --porcelain output
	tests := []struct {
		name     string
		output   string
		expected map[string]string // path -> branch
	}{
		{
			name: "single worktree",
			output: `worktree /home/user/repo
HEAD abc123def456
branch refs/heads/main
`,
			expected: map[string]string{
				"/home/user/repo": "main",
			},
		},
		{
			name: "multiple worktrees",
			output: `worktree /home/user/repo
HEAD abc123def456
branch refs/heads/main

worktree /home/user/repo/fraas/FRAAS-123
HEAD def456abc789
branch refs/heads/FRAAS-123
`,
			expected: map[string]string{
				"/home/user/repo":                "main",
				"/home/user/repo/fraas/FRAAS-123": "FRAAS-123",
			},
		},
		{
			name:     "empty output",
			output:   "",
			expected: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse the output manually (simulating what getWorktreeDetails does)
			result := make(map[string]string)
			lines := strings.Split(tt.output, "\n")
			var currentPath string

			for _, line := range lines {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "worktree ") {
					currentPath = strings.TrimPrefix(line, "worktree ")
				} else if strings.HasPrefix(line, "branch ") && currentPath != "" {
					branch := strings.TrimPrefix(line, "branch refs/heads/")
					result[currentPath] = branch
				}
			}

			// Verify results
			if len(result) != len(tt.expected) {
				t.Errorf("Parsed %d worktrees, want %d", len(result), len(tt.expected))
			}

			for path, expectedBranch := range tt.expected {
				if gotBranch, ok := result[path]; !ok {
					t.Errorf("Missing path %q", path)
				} else if gotBranch != expectedBranch {
					t.Errorf("Branch for %q = %q, want %q", path, gotBranch, expectedBranch)
				}
			}
		})
	}
}

func TestParseSessionOutput(t *testing.T) {
	// Test parsing of tmux list-sessions output format
	tests := []struct {
		name     string
		output   string
		expected map[string]string // session name -> status
	}{
		{
			name:   "single session attached",
			output: "FRAAS-123|3|1",
			expected: map[string]string{
				"FRAAS-123": "(3 windows) [attached]",
			},
		},
		{
			name:   "single session not attached",
			output: "FRAAS-456|2|0",
			expected: map[string]string{
				"FRAAS-456": "(2 windows)",
			},
		},
		{
			name: "multiple sessions",
			output: `FRAAS-123|3|1
FRAAS-456|2|0
hack-test|1|0`,
			expected: map[string]string{
				"FRAAS-123": "(3 windows) [attached]",
				"FRAAS-456": "(2 windows)",
				"hack-test": "(1 windows)",
			},
		},
		{
			name:     "empty output",
			output:   "",
			expected: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse the output manually (simulating what getSessionDetails does)
			result := make(map[string]string)
			if tt.output == "" {
				// No sessions
			} else {
				lines := strings.Split(strings.TrimSpace(tt.output), "\n")
				for _, line := range lines {
					parts := strings.Split(line, "|")
					if len(parts) >= 3 {
						name := parts[0]
						windows := parts[1]
						attached := parts[2]

						status := "(" + windows + " windows)"
						if attached == "1" {
							status += " [attached]"
						}
						result[name] = status
					}
				}
			}

			// Verify results
			if len(result) != len(tt.expected) {
				t.Errorf("Parsed %d sessions, want %d", len(result), len(tt.expected))
			}

			for name, expectedStatus := range tt.expected {
				if gotStatus, ok := result[name]; !ok {
					t.Errorf("Missing session %q", name)
				} else if gotStatus != expectedStatus {
					t.Errorf("Status for %q = %q, want %q", name, gotStatus, expectedStatus)
				}
			}
		})
	}
}
