package cmd

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// testSessionPrefix is the prefix used to identify test-created tmux sessions.
// Sessions with this prefix are automatically cleaned up after tests run.
const testSessionPrefix = "test-"

// TestMain handles test setup and teardown for the cmd package.
// It sets a testing flag to prevent tmux attach operations,
// cleans up all test sessions after tests complete.
func TestMain(m *testing.M) {
	// Set testing flag to prevent tmux attach operations
	os.Setenv("SRE_TESTING", "1")

	// Run all tests
	code := m.Run()

	// Clean up test sessions
	cleanupTestSessions()

	// Unset testing flag
	os.Unsetenv("SRE_TESTING")

	os.Exit(code)
}

// cleanupTestSessions kills all tmux sessions with the test- prefix.
// It gracefully handles the case where tmux is not installed or no test
// sessions exist.
func cleanupTestSessions() {
	// Check if tmux is available
	if _, err := exec.LookPath("tmux"); err != nil {
		return
	}

	// List all sessions
	cmd := exec.Command("tmux", "list-sessions", "-F", "#{session_name}")
	output, err := cmd.Output()
	if err != nil {
		// tmux may return error if server not running or no sessions
		return
	}

	sessions := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, session := range sessions {
		session = strings.TrimSpace(session)
		if session == "" {
			continue
		}

		// Kill sessions that start with the test prefix
		if strings.HasPrefix(session, testSessionPrefix) {
			killCmd := exec.Command("tmux", "kill-session", "-t", session)
			_ = killCmd.Run() // Ignore errors, session may already be gone
		}
	}
}
