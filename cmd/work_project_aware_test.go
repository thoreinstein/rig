package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func TestParseTicketWithProject(t *testing.T) {
	tests := []struct {
		name        string
		ticket      string
		wantFull    string
		wantProject string
		wantID      string
		wantType    string
		wantNumber  string
		expectError bool
	}{
		{
			name:        "standard ticket",
			ticket:      "proj-123",
			wantFull:    "proj-123",
			wantProject: "",
			wantID:      "proj-123",
			wantType:    "proj",
			wantNumber:  "123",
		},
		{
			name:        "ticket with project",
			ticket:      "rig:proj-123",
			wantFull:    "rig:proj-123",
			wantProject: "rig",
			wantID:      "proj-123",
			wantType:    "proj",
			wantNumber:  "123",
		},
		{
			name:        "ticket with owner/repo project",
			ticket:      "thoreinstein/rig:proj-123",
			wantFull:    "thoreinstein/rig:proj-123",
			wantProject: "thoreinstein/rig",
			wantID:      "proj-123",
			wantType:    "proj",
			wantNumber:  "123",
		},
		{
			name:        "beads ticket with project",
			ticket:      "rig:rig-80a",
			wantFull:    "rig:rig-80a",
			wantProject: "rig",
			wantID:      "rig-80a",
			wantType:    "rig",
			wantNumber:  "80a",
		},
		{
			name:        "uppercase preserved in ID",
			ticket:      "rig:PROJ-123",
			wantFull:    "rig:PROJ-123",
			wantProject: "rig",
			wantID:      "PROJ-123",
			wantType:    "proj",
			wantNumber:  "123",
		},
		{
			name:        "invalid project format (empty)",
			ticket:      ":proj-123",
			expectError: true,
		},
		{
			name:        "invalid ticket after colon",
			ticket:      "rig:invalid",
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
			if result.Project != tt.wantProject {
				t.Errorf("parseTicket(%q).Project = %q, want %q", tt.ticket, result.Project, tt.wantProject)
			}
			if result.ID != tt.wantID {
				t.Errorf("parseTicket(%q).ID = %q, want %q", tt.ticket, result.ID, tt.wantID)
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

func TestRunWorkCommand_ProjectAware(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH, skipping test")
	}

	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("failed to create src dir: %v", err)
	}

	notesDir := t.TempDir()
	setupWorkTestConfig(t, notesDir)
	viper.Set("clone.base_path", srcDir)
	viper.Set("discovery.search_paths", []string{srcDir})
	viper.Set("discovery.cache_path", filepath.Join(tmpDir, "cache.json"))
	defer viper.Reset()

	// Helper to setup a bare git repo
	setupBareRepo := func(t *testing.T, path string) {
		if err := exec.Command("git", "init", "--bare", path).Run(); err != nil {
			t.Fatalf("git init --bare failed: %v", err)
		}
		// We need at least one commit for worktree add to work.
		// So we create a temporary clone, commit, and push.
		tmpClone := filepath.Join(t.TempDir(), "rig-test-clone")
		if err := exec.Command("git", "clone", path, tmpClone).Run(); err != nil {
			t.Fatalf("git clone failed: %v", err)
		}
		if err := exec.Command("git", "-C", tmpClone, "config", "user.email", "test@example.com").Run(); err != nil {
			t.Fatalf("git config email failed: %v", err)
		}
		if err := exec.Command("git", "-C", tmpClone, "config", "user.name", "Test User").Run(); err != nil {
			t.Fatalf("git config name failed: %v", err)
		}
		if err := exec.Command("git", "-C", tmpClone, "commit", "--allow-empty", "-m", "Initial commit").Run(); err != nil {
			t.Fatalf("git commit failed: %v", err)
		}
		if err := exec.Command("git", "-C", tmpClone, "push", "origin", "HEAD").Run(); err != nil {
			t.Fatalf("git push failed: %v", err)
		}
	}

	repo1Path := filepath.Join(srcDir, "owner1", "repo1")
	if err := os.MkdirAll(repo1Path, 0755); err != nil {
		t.Fatalf("failed to create repo1 dir: %v", err)
	}
	setupBareRepo(t, repo1Path)

	// Run from a neutral directory
	t.Chdir(tmpDir)

	err := runWorkCommand("repo1:proj-123")
	if err != nil {
		t.Logf("runWorkCommand warning (likely tmux): %v", err)
	}

	// Check if worktree was created in repo1
	worktreePath := filepath.Join(repo1Path, "proj", "proj-123")
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		t.Errorf("Worktree should be created at %s", worktreePath)
	}

	// Verify note was created
	notePath := filepath.Join(notesDir, "proj", "proj-123.md")
	if _, err := os.Stat(notePath); os.IsNotExist(err) {
		t.Errorf("Note should be created at %s", notePath)
	}
}
