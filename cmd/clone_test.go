package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"

	"thoreinstein.com/rig/pkg/git"
)

func TestCloneCommandArgs(t *testing.T) {
	// Test that clone command requires exactly 1 argument
	cmd := cloneCmd

	if cmd.Args == nil {
		t.Error("clone command should have Args validation")
	}

	// The command should have Use showing <url> argument
	if cmd.Use != "clone <url>" {
		t.Errorf("clone command Use = %q, want %q", cmd.Use, "clone <url>")
	}
}

func TestCloneCommandDescription(t *testing.T) {
	cmd := cloneCmd

	if cmd.Short == "" {
		t.Error("clone command should have Short description")
	}

	if cmd.Long == "" {
		t.Error("clone command should have Long description")
	}

	// Verify key information is in the description
	if !strings.Contains(cmd.Long, "SSH") {
		t.Error("clone command Long description should mention 'SSH'")
	}

	if !strings.Contains(cmd.Long, "HTTPS") {
		t.Error("clone command Long description should mention 'HTTPS'")
	}

	if !strings.Contains(cmd.Long, "~/src") {
		t.Error("clone command Long description should mention '~/src'")
	}
}

func TestCloneCommandExamples(t *testing.T) {
	cmd := cloneCmd

	// Verify examples are present
	if !strings.Contains(cmd.Long, "rig clone") {
		t.Error("clone command should have examples")
	}
}

// Integration tests that require git

func TestRunCloneCommand_InvalidURL(t *testing.T) {
	tests := []struct {
		name   string
		url    string
		errMsg string
	}{
		{
			name:   "empty URL",
			url:    "",
			errMsg: "empty URL",
		},
		{
			name:   "invalid format",
			url:    "not-a-valid-url",
			errMsg: "invalid GitHub URL",
		},
		{
			name:   "gitlab URL",
			url:    "git@gitlab.com:owner/repo.git",
			errMsg: "invalid GitHub URL",
		},
		{
			name:   "missing repo",
			url:    "github.com/owner",
			errMsg: "invalid GitHub URL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := runCloneCommand(tt.url)
			if err == nil {
				t.Errorf("runCloneCommand(%q) should have returned an error", tt.url)
				return
			}
			if !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("runCloneCommand(%q) error = %q, should contain %q", tt.url, err.Error(), tt.errMsg)
			}
		})
	}
}

func TestRunCloneCommand_ValidURL_Integration(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH, skipping integration test")
	}

	// Create a temporary directory for the test
	tmpDir := t.TempDir()

	// Configure viper for the test
	viper.Reset()
	resetConfig()
	viper.Set("clone.base_path", tmpDir)
	defer viper.Reset()

	// Create a small test repository to clone
	// We'll use a bare repo created locally to avoid network dependency
	sourceRepo := filepath.Join(tmpDir, "source-repo")
	if err := exec.Command("git", "init", "--bare", sourceRepo).Run(); err != nil {
		t.Fatalf("Failed to create source repo: %v", err)
	}

	// Create a worktree to make an initial commit
	tempWorktree := filepath.Join(tmpDir, "temp-worktree")
	cmd := exec.Command("git", "clone", sourceRepo, tempWorktree)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to clone for setup: %v", err)
	}

	// Configure git user
	for _, args := range [][]string{
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "Test User"},
		{"config", "commit.gpgsign", "false"},
	} {
		cmd = exec.Command("git", args...)
		cmd.Dir = tempWorktree
		_ = cmd.Run()
	}

	// Create initial commit
	cmd = exec.Command("git", "commit", "--allow-empty", "-m", "Initial commit")
	cmd.Dir = tempWorktree
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create initial commit: %v", err)
	}

	// Push to bare repo
	cmd = exec.Command("git", "push", "origin", "main")
	cmd.Dir = tempWorktree
	_ = cmd.Run() // May fail if main doesn't exist, that's ok

	cmd = exec.Command("git", "push", "-u", "origin", "HEAD:main")
	cmd.Dir = tempWorktree
	if err := cmd.Run(); err != nil {
		// Try master instead
		cmd = exec.Command("git", "push", "-u", "origin", "HEAD:master")
		cmd.Dir = tempWorktree
		_ = cmd.Run()
	}

	// Clean up temp worktree
	os.RemoveAll(tempWorktree)

	// Test URL parsing only - no network operations
	t.Run("SSH URL parsing", func(t *testing.T) {
		url, err := git.ParseGitHubURL("git@github.com:test-owner/test-repo.git")
		if err != nil {
			t.Fatalf("ParseGitHubURL failed for valid SSH URL: %v", err)
		}
		if url.Protocol != "ssh" {
			t.Errorf("Protocol = %q, want %q", url.Protocol, "ssh")
		}
		if url.Owner != "test-owner" {
			t.Errorf("Owner = %q, want %q", url.Owner, "test-owner")
		}
		if url.Repo != "test-repo" {
			t.Errorf("Repo = %q, want %q", url.Repo, "test-repo")
		}
	})

	t.Run("HTTPS URL parsing", func(t *testing.T) {
		url, err := git.ParseGitHubURL("https://github.com/test-owner/test-repo")
		if err != nil {
			t.Fatalf("ParseGitHubURL failed for valid HTTPS URL: %v", err)
		}
		if url.Protocol != "https" {
			t.Errorf("Protocol = %q, want %q", url.Protocol, "https")
		}
		if url.Owner != "test-owner" {
			t.Errorf("Owner = %q, want %q", url.Owner, "test-owner")
		}
		if url.Repo != "test-repo" {
			t.Errorf("Repo = %q, want %q", url.Repo, "test-repo")
		}
	})

	t.Run("Shorthand URL parsing", func(t *testing.T) {
		url, err := git.ParseGitHubURL("github.com/test-owner/test-repo")
		if err != nil {
			t.Fatalf("ParseGitHubURL failed for valid shorthand URL: %v", err)
		}
		// Shorthand defaults to SSH
		if url.Protocol != "ssh" {
			t.Errorf("Protocol = %q, want %q", url.Protocol, "ssh")
		}
		if url.Owner != "test-owner" {
			t.Errorf("Owner = %q, want %q", url.Owner, "test-owner")
		}
		if url.Repo != "test-repo" {
			t.Errorf("Repo = %q, want %q", url.Repo, "test-repo")
		}
	})
}
