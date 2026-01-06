package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
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
	if !strings.Contains(cmd.Long, "sre clone") {
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

	// Now test cloning using local path (simulating HTTPS clone behavior)
	// Since we can't easily test actual GitHub URLs without network,
	// we verify URL parsing works correctly
	t.Run("SSH URL parsing", func(t *testing.T) {
		// This only tests URL parsing, not actual clone
		err := runCloneCommand("git@github.com:test-owner/test-repo.git")
		// We expect this to fail (can't reach github.com), but it should get past URL parsing
		if err == nil {
			t.Error("Expected error (can't reach github.com)")
		}
		// It should NOT fail with "invalid GitHub URL"
		if strings.Contains(err.Error(), "invalid GitHub URL") {
			t.Error("URL parsing should succeed for valid SSH URL")
		}
	})

	t.Run("HTTPS URL parsing", func(t *testing.T) {
		err := runCloneCommand("https://github.com/test-owner/test-repo")
		// We expect this to fail (can't reach github.com), but it should get past URL parsing
		if err == nil {
			t.Error("Expected error (can't reach github.com)")
		}
		if strings.Contains(err.Error(), "invalid GitHub URL") {
			t.Error("URL parsing should succeed for valid HTTPS URL")
		}
	})

	t.Run("Shorthand URL parsing", func(t *testing.T) {
		err := runCloneCommand("github.com/test-owner/test-repo")
		if err == nil {
			t.Error("Expected error (can't reach github.com)")
		}
		if strings.Contains(err.Error(), "invalid GitHub URL") {
			t.Error("URL parsing should succeed for valid shorthand URL")
		}
	})
}
