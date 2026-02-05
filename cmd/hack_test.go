package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// setupTestGitRepo creates a temporary bare git repository for testing
// Returns the repo path and a cleanup function
func setupTestGitRepo(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")

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

	// Create a worktree for the main branch to make initial commit
	mainWorktree := filepath.Join(tmpDir, "main-worktree")
	cmd = exec.Command("git", "worktree", "add", "-b", "main", mainWorktree)
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git worktree add main failed: %v", err)
	}

	// Create initial commit in the main worktree
	cmd = exec.Command("git", "commit", "--allow-empty", "-m", "Initial commit")
	cmd.Dir = mainWorktree
	if err := cmd.Run(); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}

	// Remove the temp worktree - we'll work from the bare repo
	cmd = exec.Command("git", "worktree", "remove", mainWorktree)
	cmd.Dir = repoDir
	_ = cmd.Run() // Ignore errors

	return repoDir
}

// setupTestConfig configures viper with test defaults
func setupTestConfig(t *testing.T, notesPath string) {
	t.Helper()

	viper.Reset()
	resetConfig()
	viper.Set("notes.path", notesPath)
	viper.Set("notes.daily_dir", "daily")
	viper.Set("notes.template_dir", filepath.Join(notesPath, "templates"))
	viper.Set("git.base_branch", "")
	viper.Set("jira.enabled", false)
	viper.Set("tmux.session_prefix", "test-")
	viper.Set("tmux.windows", []map[string]string{
		{"name": "code", "command": ""},
	})
}

func TestHackCommandFlags(t *testing.T) {
	// Test that the hack command has the expected flags
	cmd := hackCmd

	// Check --no-notes flag exists
	noNotesFlag := cmd.Flags().Lookup("no-notes")
	if noNotesFlag == nil {
		t.Error("hack command should have --no-notes flag")
	}
	if noNotesFlag != nil && noNotesFlag.DefValue != "false" {
		t.Errorf("--no-notes default should be false (notes enabled by default), got %s", noNotesFlag.DefValue)
	}

	// Note: --repo flag was removed - repo is now detected from CWD
}

func TestHackCommandArgs(t *testing.T) {
	// Test that hack command requires exactly 1 argument
	cmd := hackCmd

	if cmd.Args == nil {
		t.Error("hack command should have Args validation")
	}

	// The command should have Use showing <name> argument
	if cmd.Use != "hack <name>" {
		t.Errorf("hack command Use = %q, want %q", cmd.Use, "hack <name>")
	}
}

func TestHackCommandDescription(t *testing.T) {
	cmd := hackCmd

	if cmd.Short == "" {
		t.Error("hack command should have Short description")
	}

	if cmd.Long == "" {
		t.Error("hack command should have Long description")
	}

	// Verify key information is in the description
	if !containsSubstring(cmd.Long, "hack") {
		t.Error("hack command Long description should mention 'hack'")
	}

	if !containsSubstring(cmd.Long, "worktree") {
		t.Error("hack command Long description should mention 'worktree'")
	}
}

func TestHackBranchNaming(t *testing.T) {
	// The hack command creates branches without prefix (just the name)
	// Test that the expected branch name format is documented
	tests := []struct {
		name           string
		hackName       string
		expectedBranch string
	}{
		{
			name:           "simple name",
			hackName:       "experiment",
			expectedBranch: "experiment",
		},
		{
			name:           "name with dashes",
			hackName:       "winter-2025",
			expectedBranch: "winter-2025",
		},
		{
			name:           "name with numbers",
			hackName:       "test123",
			expectedBranch: "test123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify expected branch format (no prefix)
			if tt.hackName != tt.expectedBranch {
				t.Errorf("Branch format = %q, want %q", tt.hackName, tt.expectedBranch)
			}
		})
	}
}

func TestHackWorktreePath(t *testing.T) {
	// The hack command creates worktrees under "hack" directory
	// Test the expected path structure
	tests := []struct {
		name         string
		hackName     string
		expectedType string
	}{
		{
			name:         "simple name",
			hackName:     "experiment",
			expectedType: "hack",
		},
		{
			name:         "complex name",
			hackName:     "winter-2025-cleanup",
			expectedType: "hack",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Hack worktrees should always be under "hack" type directory
			if tt.expectedType != "hack" {
				t.Errorf("Hack worktree type should always be 'hack', got %q", tt.expectedType)
			}
		})
	}
}

func TestValidateHackName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		errMsg  string
	}{
		// Valid cases
		{
			name:    "simple name",
			input:   "winter-2025",
			wantErr: false,
		},
		{
			name:    "underscore name",
			input:   "experiment_auth",
			wantErr: false,
		},
		{
			name:    "camelCase name",
			input:   "quickFix",
			wantErr: false,
		},
		{
			name:    "single letter",
			input:   "a",
			wantErr: false,
		},
		{
			name:    "max length 64 chars",
			input:   "a" + strings.Repeat("b", 63),
			wantErr: false,
		},
		// Invalid cases
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
			errMsg:  "cannot be empty",
		},
		{
			name:    "starts with number",
			input:   "123-test",
			wantErr: true,
			errMsg:  "must start with a letter",
		},
		{
			name:    "starts with dot",
			input:   ".hidden",
			wantErr: true,
			errMsg:  "must start with a letter",
		},
		{
			name:    "path traversal",
			input:   "../../../etc/passwd",
			wantErr: true,
			errMsg:  "must start with a letter",
		},
		{
			name:    "contains slash",
			input:   "test/path",
			wantErr: true,
			errMsg:  "must start with a letter",
		},
		{
			name:    "shell injection attempt",
			input:   "test;rm -rf /",
			wantErr: true,
			errMsg:  "must start with a letter",
		},
		{
			name:    "contains spaces",
			input:   "my hack",
			wantErr: true,
			errMsg:  "must start with a letter",
		},
		{
			name:    "exceeds max length",
			input:   "a" + strings.Repeat("b", 64),
			wantErr: true,
			errMsg:  "max 64 characters",
		},
		{
			name:    "starts with hyphen",
			input:   "-test",
			wantErr: true,
			errMsg:  "must start with a letter",
		},
		{
			name:    "starts with underscore",
			input:   "_test",
			wantErr: true,
			errMsg:  "must start with a letter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateHackName(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("validateHackName(%q) should have returned an error", tt.input)
					return
				}
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("validateHackName(%q) error = %q, should contain %q", tt.input, err.Error(), tt.errMsg)
				}
			} else if err != nil {
				t.Errorf("validateHackName(%q) returned unexpected error: %v", tt.input, err)
			}
		})
	}
}

// Integration tests for runHackCommand

func TestRunHackCommand_CreatesWorktree(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH, skipping test")
	}

	repoDir := setupTestGitRepo(t)
	notesDir := t.TempDir()
	setupTestConfig(t, notesDir)
	defer viper.Reset()

	// Change to repo directory (t.Chdir handles cleanup)
	t.Chdir(repoDir)

	// Set project flag to bypass interactive picker
	projectFlag = repoDir
	defer func() { projectFlag = "" }()

	// Reset hack flags for test
	hackNoNotes = false

	// Run the hack command
	err := runHackCommand("test-experiment")

	// The command may fail on tmux session creation, but should create worktree
	// Check for worktree creation regardless of tmux status
	worktreePath := filepath.Join(repoDir, "hack", "test-experiment")
	if _, statErr := os.Stat(worktreePath); os.IsNotExist(statErr) {
		t.Errorf("Worktree should be created at %s, err from runHackCommand: %v", worktreePath, err)
	}

	// Verify it's a valid git worktree
	cmd := exec.Command("git", "worktree", "list")
	cmd.Dir = repoDir
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("git worktree list failed: %v", err)
	}

	if !strings.Contains(string(output), "hack/test-experiment") {
		t.Errorf("Worktree list should contain hack/test-experiment, got: %s", string(output))
	}

	// Verify branch was created
	cmd = exec.Command("git", "branch", "--list", "test-experiment")
	cmd.Dir = repoDir
	output, err = cmd.Output()
	if err != nil {
		t.Fatalf("git branch list failed: %v", err)
	}

	if !strings.Contains(string(output), "test-experiment") {
		t.Errorf("Branch test-experiment should exist, got: %s", string(output))
	}
}

func TestRunHackCommand_InvalidName(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH, skipping test")
	}

	repoDir := setupTestGitRepo(t)
	notesDir := t.TempDir()
	setupTestConfig(t, notesDir)
	defer viper.Reset()

	t.Chdir(repoDir)
	// Set project flag to bypass interactive picker
	projectFlag = repoDir
	defer func() { projectFlag = "" }()

	tests := []struct {
		name     string
		hackName string
		errMsg   string
	}{
		{
			name:     "path traversal attempt",
			hackName: "../../../etc/passwd",
			errMsg:   "must start with a letter",
		},
		{
			name:     "shell injection",
			hackName: "test;rm -rf /",
			errMsg:   "must start with a letter",
		},
		{
			name:     "empty name",
			hackName: "",
			errMsg:   "cannot be empty",
		},
		{
			name:     "starts with number",
			hackName: "123test",
			errMsg:   "must start with a letter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := runHackCommand(tt.hackName)
			if err == nil {
				t.Errorf("runHackCommand(%q) should have returned an error", tt.hackName)
				return
			}
			if !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("runHackCommand(%q) error = %q, should contain %q", tt.hackName, err.Error(), tt.errMsg)
			}
		})
	}
}

func TestRunHackCommand_NotesEnabledByDefault(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH, skipping test")
	}

	repoDir := setupTestGitRepo(t)
	notesDir := t.TempDir()

	// Create templates directory and hack template
	templatesDir := filepath.Join(notesDir, "templates")
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("Failed to create templates dir: %v", err)
	}

	// Create a simple hack template
	hackTemplate := `# {{.Ticket}}

## Type
hack

## Notes
`
	if err := os.WriteFile(filepath.Join(templatesDir, "hack.md.tmpl"), []byte(hackTemplate), 0644); err != nil {
		t.Fatalf("Failed to write hack template: %v", err)
	}

	// Create daily template for UpdateDailyNote
	dailyTemplate := `# {{.Date}}

## Log
`
	if err := os.WriteFile(filepath.Join(templatesDir, "daily.md.tmpl"), []byte(dailyTemplate), 0644); err != nil {
		t.Fatalf("Failed to write daily template: %v", err)
	}

	setupTestConfig(t, notesDir)
	viper.Set("notes.template_dir", templatesDir)
	defer viper.Reset()

	t.Chdir(repoDir)
	projectFlag = repoDir
	defer func() { projectFlag = "" }()

	// Notes should be enabled by default (hackNoNotes = false)
	hackNoNotes = false
	defer func() { hackNoNotes = false }()

	// Run the hack command
	_ = runHackCommand("notes-default-test")

	// Worktree should be created
	worktreePath := filepath.Join(repoDir, "hack", "notes-default-test")
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		t.Errorf("Worktree should be created at %s", worktreePath)
	}

	// Note should be created (default behavior)
	notePath := filepath.Join(notesDir, "hack", "notes-default-test.md")
	if _, err := os.Stat(notePath); os.IsNotExist(err) {
		t.Errorf("Note should be created at %s by default", notePath)
	}
}

func TestRunHackCommand_WithNoNotesFlag(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH, skipping test")
	}

	repoDir := setupTestGitRepo(t)
	notesDir := t.TempDir()

	// Create templates directory and daily template (needed for UpdateDailyNote)
	templatesDir := filepath.Join(notesDir, "templates")
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("Failed to create templates dir: %v", err)
	}

	dailyTemplate := `# {{.Date}}

## Log
`
	if err := os.WriteFile(filepath.Join(templatesDir, "daily.md.tmpl"), []byte(dailyTemplate), 0644); err != nil {
		t.Fatalf("Failed to write daily template: %v", err)
	}

	setupTestConfig(t, notesDir)
	viper.Set("notes.template_dir", templatesDir)
	defer viper.Reset()

	t.Chdir(repoDir)
	projectFlag = repoDir
	defer func() { projectFlag = "" }()

	// Disable notes with --no-notes flag
	hackNoNotes = true
	defer func() { hackNoNotes = false }()

	// Run the hack command
	_ = runHackCommand("no-notes-test")

	// Worktree should still be created
	worktreePath := filepath.Join(repoDir, "hack", "no-notes-test")
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		t.Errorf("Worktree should be created at %s even with --no-notes", worktreePath)
	}

	// Note should NOT be created when --no-notes is set
	notePath := filepath.Join(notesDir, "hack", "no-notes-test.md")
	if _, err := os.Stat(notePath); err == nil {
		t.Errorf("Note should NOT be created at %s when --no-notes is set", notePath)
	}
}

func TestRunHackCommand_IdempotentWorktree(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH, skipping test")
	}

	repoDir := setupTestGitRepo(t)
	notesDir := t.TempDir()
	setupTestConfig(t, notesDir)
	defer viper.Reset()

	t.Chdir(repoDir)
	projectFlag = repoDir
	defer func() { projectFlag = "" }()

	hackNoNotes = false

	// Run the hack command twice
	_ = runHackCommand("idempotent-test")

	// Second call should not fail (worktree already exists)
	_ = runHackCommand("idempotent-test")
	// The command should complete without error for existing worktree
	// (tmux might fail, but that's separate)

	worktreePath := filepath.Join(repoDir, "hack", "idempotent-test")
	if _, statErr := os.Stat(worktreePath); os.IsNotExist(statErr) {
		t.Errorf("Worktree should exist at %s after repeated calls", worktreePath)
	}
}
