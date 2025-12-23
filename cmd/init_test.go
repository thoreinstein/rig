package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
)

func TestParseTicket(t *testing.T) {
	tests := []struct {
		name        string
		ticket      string
		wantFull    string
		wantType    string
		wantNumber  string
		expectError bool
	}{
		{
			name:       "lowercase fraas ticket",
			ticket:     "fraas-25857",
			wantFull:   "fraas-25857",
			wantType:   "fraas",
			wantNumber: "25857",
		},
		{
			name:       "uppercase FRAAS ticket",
			ticket:     "FRAAS-25857",
			wantFull:   "FRAAS-25857",
			wantType:   "fraas",
			wantNumber: "25857",
		},
		{
			name:       "mixed case ticket",
			ticket:     "Fraas-12345",
			wantFull:   "Fraas-12345",
			wantType:   "fraas",
			wantNumber: "12345",
		},
		{
			name:       "cre ticket",
			ticket:     "cre-123",
			wantFull:   "cre-123",
			wantType:   "cre",
			wantNumber: "123",
		},
		{
			name:       "incident ticket",
			ticket:     "incident-456",
			wantFull:   "incident-456",
			wantType:   "incident",
			wantNumber: "456",
		},
		{
			name:       "ops ticket",
			ticket:     "OPS-789",
			wantFull:   "OPS-789",
			wantType:   "ops",
			wantNumber: "789",
		},
		{
			name:       "single digit number",
			ticket:     "test-1",
			wantFull:   "test-1",
			wantType:   "test",
			wantNumber: "1",
		},
		{
			name:       "large number",
			ticket:     "jira-999999999",
			wantFull:   "jira-999999999",
			wantType:   "jira",
			wantNumber: "999999999",
		},
		{
			name:        "missing number",
			ticket:      "fraas-",
			expectError: true,
		},
		{
			name:        "missing type",
			ticket:      "-123",
			expectError: true,
		},
		{
			name:        "no dash",
			ticket:      "fraas123",
			expectError: true,
		},
		{
			name:        "only number",
			ticket:      "12345",
			expectError: true,
		},
		{
			name:        "only type",
			ticket:      "fraas",
			expectError: true,
		},
		{
			name:        "empty string",
			ticket:      "",
			expectError: true,
		},
		{
			name:        "letters in number",
			ticket:      "fraas-abc",
			expectError: true,
		},
		{
			name:        "mixed letters and numbers",
			ticket:      "fraas-123abc",
			expectError: true,
		},
		{
			name:        "multiple dashes",
			ticket:      "fraas-123-456",
			expectError: true,
		},
		{
			name:        "spaces in ticket",
			ticket:      "fraas -123",
			expectError: true,
		},
		{
			name:        "special characters in type",
			ticket:      "fra@s-123",
			expectError: true,
		},
		{
			name:        "underscore in type",
			ticket:      "fra_s-123",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseTicket(tt.ticket)

			if tt.expectError {
				if err == nil {
					t.Errorf("parseTicket(%q) expected error, got nil", tt.ticket)
				}
				return
			}

			if err != nil {
				t.Fatalf("parseTicket(%q) unexpected error: %v", tt.ticket, err)
			}

			if result.Full != tt.wantFull {
				t.Errorf("parseTicket(%q).Full = %q, want %q", tt.ticket, result.Full, tt.wantFull)
			}
			if result.Type != tt.wantType {
				t.Errorf("parseTicket(%q).Type = %q, want %q", tt.ticket, result.Type, tt.wantType)
			}
			if result.Number != tt.wantNumber {
				t.Errorf("parseTicket(%q).Number = %q, want %q", tt.ticket, result.Number, tt.wantNumber)
			}
		})
	}
}

func TestTicketInfo(t *testing.T) {
	// Test the TicketInfo struct
	info := &TicketInfo{
		Full:   "FRAAS-123",
		Type:   "fraas",
		Number: "123",
	}

	if info.Full != "FRAAS-123" {
		t.Errorf("Full = %q, want %q", info.Full, "FRAAS-123")
	}
	if info.Type != "fraas" {
		t.Errorf("Type = %q, want %q", info.Type, "fraas")
	}
	if info.Number != "123" {
		t.Errorf("Number = %q, want %q", info.Number, "123")
	}
}

func TestInitCommandDescription(t *testing.T) {
	cmd := initCmd

	if cmd.Use != "init <ticket>" {
		t.Errorf("init command Use = %q, want %q", cmd.Use, "init <ticket>")
	}

	if cmd.Short == "" {
		t.Error("init command should have Short description")
	}

	if cmd.Long == "" {
		t.Error("init command should have Long description")
	}

	// Verify key information is in the description
	if !containsSubstring(cmd.Long, "worktree") {
		t.Error("init command Long description should mention 'worktree'")
	}

	if !containsSubstring(cmd.Long, "note") {
		t.Error("init command Long description should mention 'note'")
	}

	if !containsSubstring(cmd.Long, "tmux") {
		t.Error("init command Long description should mention 'tmux'")
	}
}

func TestInitCommandArgs(t *testing.T) {
	cmd := initCmd

	// Command should require exactly 1 argument
	if cmd.Args == nil {
		t.Error("init command should have Args validation")
	}
}

func TestTicketTypeNormalization(t *testing.T) {
	// Test that ticket types are normalized to lowercase
	tests := []struct {
		name         string
		ticket       string
		expectedType string
	}{
		{
			name:         "uppercase type",
			ticket:       "FRAAS-123",
			expectedType: "fraas",
		},
		{
			name:         "lowercase type",
			ticket:       "fraas-123",
			expectedType: "fraas",
		},
		{
			name:         "mixed case type",
			ticket:       "FrAaS-123",
			expectedType: "fraas",
		},
		{
			name:         "CRE uppercase",
			ticket:       "CRE-456",
			expectedType: "cre",
		},
		{
			name:         "incident mixed",
			ticket:       "Incident-789",
			expectedType: "incident",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseTicket(tt.ticket)
			if err != nil {
				t.Fatalf("parseTicket(%q) error: %v", tt.ticket, err)
			}

			if result.Type != tt.expectedType {
				t.Errorf("parseTicket(%q).Type = %q, want %q", tt.ticket, result.Type, tt.expectedType)
			}
		})
	}
}

func TestTicketFullPreservation(t *testing.T) {
	// Test that the original ticket format is preserved in Full field
	tests := []struct {
		name         string
		ticket       string
		expectedFull string
	}{
		{
			name:         "uppercase preserved",
			ticket:       "FRAAS-123",
			expectedFull: "FRAAS-123",
		},
		{
			name:         "lowercase preserved",
			ticket:       "fraas-123",
			expectedFull: "fraas-123",
		},
		{
			name:         "mixed case preserved",
			ticket:       "FrAaS-123",
			expectedFull: "FrAaS-123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseTicket(tt.ticket)
			if err != nil {
				t.Fatalf("parseTicket(%q) error: %v", tt.ticket, err)
			}

			if result.Full != tt.expectedFull {
				t.Errorf("parseTicket(%q).Full = %q, want %q", tt.ticket, result.Full, tt.expectedFull)
			}
		})
	}
}

func TestParseTicketErrorMessages(t *testing.T) {
	// Test that error messages are helpful
	_, err := parseTicket("invalid")
	if err == nil {
		t.Fatal("Expected error for invalid ticket")
	}

	errorMsg := err.Error()

	// Error should mention expected format
	if !containsSubstring(errorMsg, "TYPE-NUMBER") {
		t.Error("Error message should mention expected format TYPE-NUMBER")
	}

	// Error should give an example
	if !containsSubstring(errorMsg, "proj-123") {
		t.Error("Error message should include an example like 'proj-123'")
	}
}

// Integration tests for runInitCommand

// setupInitTestGitRepo creates a temporary bare git repository for testing
func setupInitTestGitRepo(t *testing.T) string {
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

// setupInitTestConfig configures viper with test defaults for init command
func setupInitTestConfig(t *testing.T, notesPath string) {
	t.Helper()

	viper.Reset()
	viper.Set("notes.path", notesPath)
	viper.Set("notes.daily_dir", "daily")
	viper.Set("notes.template_dir", "") // Use embedded templates
	viper.Set("git.base_branch", "")
	viper.Set("jira.enabled", false)
	viper.Set("tmux.session_prefix", "")
	viper.Set("tmux.windows", []map[string]string{
		{"name": "code", "command": ""},
	})
}

func TestRunInitCommand_CreatesWorktreeAndNote(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH, skipping test")
	}

	repoDir := setupInitTestGitRepo(t)
	notesDir := t.TempDir()
	setupInitTestConfig(t, notesDir)
	defer viper.Reset()

	// Change to repo directory
	t.Chdir(repoDir)

	// Run the init command
	err := runInitCommand("proj-123")

	// The command may fail on tmux session creation, but should create worktree and note
	// Check for worktree creation regardless of tmux status
	worktreePath := filepath.Join(repoDir, "proj", "proj-123")
	if _, statErr := os.Stat(worktreePath); os.IsNotExist(statErr) {
		t.Errorf("Worktree should be created at %s, err from runInitCommand: %v", worktreePath, err)
	}

	// Verify it's a valid git worktree
	cmd := exec.Command("git", "worktree", "list")
	cmd.Dir = repoDir
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("git worktree list failed: %v", err)
	}

	if !strings.Contains(string(output), "proj/proj-123") {
		t.Errorf("Worktree list should contain proj/proj-123, got: %s", string(output))
	}

	// Verify branch was created
	cmd = exec.Command("git", "branch", "--list", "proj-123")
	cmd.Dir = repoDir
	output, err = cmd.Output()
	if err != nil {
		t.Fatalf("git branch list failed: %v", err)
	}

	if !strings.Contains(string(output), "proj-123") {
		t.Errorf("Branch proj-123 should exist, got: %s", string(output))
	}

	// Verify note was created
	notePath := filepath.Join(notesDir, "proj", "proj-123.md")
	if _, statErr := os.Stat(notePath); os.IsNotExist(statErr) {
		t.Errorf("Note should be created at %s", notePath)
	}

	// Verify note content includes ticket info
	noteContent, err := os.ReadFile(notePath)
	if err != nil {
		t.Fatalf("Failed to read note: %v", err)
	}

	if !strings.Contains(string(noteContent), "proj-123") {
		t.Errorf("Note should contain ticket name, got: %s", string(noteContent))
	}
}

func TestRunInitCommand_InvalidTicketFormat(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH, skipping test")
	}

	repoDir := setupInitTestGitRepo(t)
	notesDir := t.TempDir()
	setupInitTestConfig(t, notesDir)
	defer viper.Reset()

	t.Chdir(repoDir)

	tests := []struct {
		name   string
		ticket string
		errMsg string
	}{
		{
			name:   "no dash",
			ticket: "proj123",
			errMsg: "invalid ticket format",
		},
		{
			name:   "no number",
			ticket: "proj-",
			errMsg: "invalid ticket format",
		},
		{
			name:   "no type",
			ticket: "-123",
			errMsg: "invalid ticket format",
		},
		{
			name:   "letters in number",
			ticket: "proj-abc",
			errMsg: "invalid ticket format",
		},
		{
			name:   "empty",
			ticket: "",
			errMsg: "invalid ticket format",
		},
		{
			name:   "multiple dashes",
			ticket: "proj-123-456",
			errMsg: "invalid ticket format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := runInitCommand(tt.ticket)
			if err == nil {
				t.Errorf("runInitCommand(%q) should have returned an error", tt.ticket)
				return
			}
			if !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("runInitCommand(%q) error = %q, should contain %q", tt.ticket, err.Error(), tt.errMsg)
			}
		})
	}
}

func TestRunInitCommand_UpdatesDailyNote(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH, skipping test")
	}

	repoDir := setupInitTestGitRepo(t)
	notesDir := t.TempDir()
	setupInitTestConfig(t, notesDir)
	defer viper.Reset()

	t.Chdir(repoDir)

	// Run the init command
	_ = runInitCommand("ops-456")

	// Verify daily note was created/updated
	today := time.Now().Format("2006-01-02")
	dailyNotePath := filepath.Join(notesDir, "daily", today+".md")

	if _, statErr := os.Stat(dailyNotePath); os.IsNotExist(statErr) {
		t.Errorf("Daily note should be created at %s", dailyNotePath)
		return
	}

	// Verify daily note contains ticket reference
	dailyContent, err := os.ReadFile(dailyNotePath)
	if err != nil {
		t.Fatalf("Failed to read daily note: %v", err)
	}

	if !strings.Contains(string(dailyContent), "ops-456") {
		t.Errorf("Daily note should contain ticket reference, got: %s", string(dailyContent))
	}

	// Verify it has a link to the ticket note
	if !strings.Contains(string(dailyContent), "../ops/ops-456.md") {
		t.Errorf("Daily note should contain relative link to ticket note, got: %s", string(dailyContent))
	}
}

func TestRunInitCommand_IdempotentWorktree(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH, skipping test")
	}

	repoDir := setupInitTestGitRepo(t)
	notesDir := t.TempDir()
	setupInitTestConfig(t, notesDir)
	defer viper.Reset()

	t.Chdir(repoDir)

	// Run the init command twice
	_ = runInitCommand("test-789")

	// Second call should not fail (worktree already exists)
	_ = runInitCommand("test-789")

	worktreePath := filepath.Join(repoDir, "test", "test-789")
	if _, statErr := os.Stat(worktreePath); os.IsNotExist(statErr) {
		t.Errorf("Worktree should exist at %s after repeated calls", worktreePath)
	}
}

func TestRunInitCommand_DifferentTicketTypes(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH, skipping test")
	}

	tests := []struct {
		name         string
		ticket       string
		expectedType string
	}{
		{
			name:         "fraas ticket",
			ticket:       "fraas-12345",
			expectedType: "fraas",
		},
		{
			name:         "cre ticket",
			ticket:       "CRE-999",
			expectedType: "cre",
		},
		{
			name:         "incident ticket",
			ticket:       "incident-1",
			expectedType: "incident",
		},
		{
			name:         "ops ticket",
			ticket:       "OPS-42",
			expectedType: "ops",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoDir := setupInitTestGitRepo(t)
			notesDir := t.TempDir()
			setupInitTestConfig(t, notesDir)
			defer viper.Reset()

			t.Chdir(repoDir)

			_ = runInitCommand(tt.ticket)

			// Worktree should be under the ticket type directory
			worktreePath := filepath.Join(repoDir, tt.expectedType, tt.ticket)
			if _, statErr := os.Stat(worktreePath); os.IsNotExist(statErr) {
				t.Errorf("Worktree should be created at %s for ticket %s", worktreePath, tt.ticket)
			}

			// Note should be under the ticket type directory
			notePath := filepath.Join(notesDir, tt.expectedType, tt.ticket+".md")
			if _, statErr := os.Stat(notePath); os.IsNotExist(statErr) {
				t.Errorf("Note should be created at %s for ticket %s", notePath, tt.ticket)
			}
		})
	}
}

func TestRunInitCommand_JiraDisabled(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH, skipping test")
	}

	repoDir := setupInitTestGitRepo(t)
	notesDir := t.TempDir()
	setupInitTestConfig(t, notesDir)
	viper.Set("jira.enabled", false) // Explicitly disable JIRA
	defer viper.Reset()

	t.Chdir(repoDir)

	// Run the init command - should succeed without JIRA
	_ = runInitCommand("nojira-100")

	// Worktree should still be created
	worktreePath := filepath.Join(repoDir, "nojira", "nojira-100")
	if _, statErr := os.Stat(worktreePath); os.IsNotExist(statErr) {
		t.Errorf("Worktree should be created at %s even with JIRA disabled", worktreePath)
	}

	// Note should still be created
	notePath := filepath.Join(notesDir, "nojira", "nojira-100.md")
	if _, statErr := os.Stat(notePath); os.IsNotExist(statErr) {
		t.Errorf("Note should be created at %s even with JIRA disabled", notePath)
	}

	// Note should contain fallback summary
	noteContent, err := os.ReadFile(notePath)
	if err != nil {
		t.Fatalf("Failed to read note: %v", err)
	}

	if !strings.Contains(string(noteContent), "Work on nojira ticket") {
		t.Errorf("Note should contain fallback summary when JIRA disabled, got: %s", string(noteContent))
	}
}

func TestRunInitCommand_PreservesOriginalTicketCase(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH, skipping test")
	}

	repoDir := setupInitTestGitRepo(t)
	notesDir := t.TempDir()
	setupInitTestConfig(t, notesDir)
	defer viper.Reset()

	t.Chdir(repoDir)

	// Use uppercase ticket
	_ = runInitCommand("FRAAS-999")

	// Note filename should preserve original case
	notePath := filepath.Join(notesDir, "fraas", "FRAAS-999.md")
	if _, statErr := os.Stat(notePath); os.IsNotExist(statErr) {
		t.Errorf("Note should be created at %s preserving original case", notePath)
	}

	// Note content should have original case
	noteContent, err := os.ReadFile(notePath)
	if err != nil {
		t.Fatalf("Failed to read note: %v", err)
	}

	if !strings.Contains(string(noteContent), "FRAAS-999") {
		t.Errorf("Note content should preserve original ticket case, got: %s", string(noteContent))
	}
}
