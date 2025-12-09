package tmux

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestNewSessionManager(t *testing.T) {
	windows := []WindowConfig{
		{Name: "code", Command: "nvim", WorkingDir: "{worktree_path}"},
		{Name: "term", WorkingDir: "{worktree_path}"},
	}

	sm := NewSessionManager("prefix-", windows, true)

	if sm.SessionPrefix != "prefix-" {
		t.Errorf("SessionPrefix = %q, want %q", sm.SessionPrefix, "prefix-")
	}
	if len(sm.Windows) != 2 {
		t.Errorf("Windows count = %d, want 2", len(sm.Windows))
	}
	if !sm.Verbose {
		t.Error("Verbose should be true")
	}
}

func TestGetSessionName(t *testing.T) {
	tests := []struct {
		name     string
		prefix   string
		ticket   string
		expected string
	}{
		{
			name:     "no prefix",
			prefix:   "",
			ticket:   "FRAAS-123",
			expected: "FRAAS-123",
		},
		{
			name:     "with prefix",
			prefix:   "sre-",
			ticket:   "FRAAS-123",
			expected: "sre-FRAAS-123",
		},
		{
			name:     "prefix with underscore",
			prefix:   "work_",
			ticket:   "CRE-456",
			expected: "work_CRE-456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm := NewSessionManager(tt.prefix, nil, false)
			result := sm.GetSessionName(tt.ticket)
			if result != tt.expected {
				t.Errorf("GetSessionName(%q) = %q, want %q", tt.ticket, result, tt.expected)
			}
		})
	}
}

func TestExpandPath(t *testing.T) {
	sm := NewSessionManager("", nil, false)

	homeDir, _ := os.UserHomeDir()

	tests := []struct {
		name         string
		template     string
		worktreePath string
		notePath     string
		expected     string
	}{
		{
			name:         "worktree placeholder",
			template:     "{worktree_path}",
			worktreePath: "/home/user/project",
			notePath:     "",
			expected:     "/home/user/project",
		},
		{
			name:         "note placeholder",
			template:     "{note_path}",
			worktreePath: "",
			notePath:     "/home/user/vault/note.md",
			expected:     "/home/user/vault/note.md",
		},
		{
			name:         "both placeholders",
			template:     "nvim {note_path}",
			worktreePath: "/project",
			notePath:     "/vault/note.md",
			expected:     "nvim /vault/note.md",
		},
		{
			name:         "tilde expansion",
			template:     "~/Documents",
			worktreePath: "",
			notePath:     "",
			expected:     filepath.Join(homeDir, "Documents"),
		},
		{
			name:         "no placeholders",
			template:     "/usr/local/bin/nvim",
			worktreePath: "/project",
			notePath:     "/note.md",
			expected:     "/usr/local/bin/nvim",
		},
		{
			name:         "command with worktree",
			template:     "cd {worktree_path} && make build",
			worktreePath: "/home/user/myproject",
			notePath:     "",
			expected:     "cd /home/user/myproject && make build",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sm.expandPath(tt.template, tt.worktreePath, tt.notePath)
			if result != tt.expected {
				t.Errorf("expandPath(%q) = %q, want %q", tt.template, result, tt.expected)
			}
		})
	}
}

func TestSessionExists_NoTmux(t *testing.T) {
	// Skip if tmux is not available
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not found in PATH, skipping test")
	}

	sm := NewSessionManager("", nil, false)

	// Test with a session name that definitely doesn't exist
	exists := sm.SessionExists("nonexistent-session-xyz-123456")
	if exists {
		t.Error("SessionExists() should return false for non-existent session")
	}
}

// Integration tests - these require tmux to be running
func TestCreateAndKillSession_Integration(t *testing.T) {
	// Skip if tmux is not available
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not found in PATH, skipping integration test")
	}

	// Create a temporary worktree directory
	tmpDir, err := os.MkdirTemp("", "tmux-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Simple windows config for testing
	windows := []WindowConfig{
		{Name: "test", WorkingDir: "{worktree_path}"},
	}

	sm := NewSessionManager("test-", windows, false)
	sessionName := sm.GetSessionName("integration-test")

	// Clean up any existing session first
	exec.Command("tmux", "kill-session", "-t", sessionName).Run()

	// Create a detached session for testing (we can't attach in test)
	cmd := exec.Command("tmux", "new-session", "-d", "-s", sessionName, "-c", tmpDir)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create test session: %v", err)
	}

	// Verify session exists
	if !sm.SessionExists(sessionName) {
		t.Error("Session should exist after creation")
	}

	// Kill the session
	err = sm.KillSession("integration-test")
	if err != nil {
		t.Fatalf("KillSession() error: %v", err)
	}

	// Verify session is gone
	if sm.SessionExists(sessionName) {
		t.Error("Session should not exist after killing")
	}
}

func TestListSessions_Integration(t *testing.T) {
	// Skip if tmux is not available
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not found in PATH, skipping integration test")
	}

	sm := NewSessionManager("", nil, false)

	// This test just verifies ListSessions doesn't error
	// We can't guarantee any sessions exist
	sessions, err := sm.ListSessions()
	if err != nil {
		// If tmux server isn't running, that's ok for this test
		if sessions == nil {
			t.Skip("tmux server not running, skipping test")
		}
		t.Fatalf("ListSessions() error: %v", err)
	}

	// sessions could be empty or have items - both are valid
	t.Logf("Found %d sessions", len(sessions))
}

func TestKillSession_NonExistent(t *testing.T) {
	// Skip if tmux is not available
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not found in PATH, skipping test")
	}

	sm := NewSessionManager("", nil, false)

	err := sm.KillSession("definitely-does-not-exist-xyz-999")
	if err == nil {
		t.Error("KillSession() should return error for non-existent session")
	}
}

func TestCreateSession_NonExistentWorktree(t *testing.T) {
	// Skip if tmux is not available
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not found in PATH, skipping test")
	}

	sm := NewSessionManager("test-", nil, false)

	err := sm.CreateSession("test-ticket", "/nonexistent/path/that/does/not/exist", "")
	if err == nil {
		t.Error("CreateSession() should return error for non-existent worktree path")
	}
}

func TestWindowConfig(t *testing.T) {
	// Test that WindowConfig struct works as expected
	config := WindowConfig{
		Name:       "editor",
		Command:    "nvim {note_path}",
		WorkingDir: "{worktree_path}",
	}

	if config.Name != "editor" {
		t.Errorf("Name = %q, want %q", config.Name, "editor")
	}
	if config.Command != "nvim {note_path}" {
		t.Errorf("Command = %q, want %q", config.Command, "nvim {note_path}")
	}
	if config.WorkingDir != "{worktree_path}" {
		t.Errorf("WorkingDir = %q, want %q", config.WorkingDir, "{worktree_path}")
	}
}
