package tmux

import (
	"os"
	"os/exec"
)

// TestSocketName is the socket name used for test isolation.
// Tests run on a separate tmux server that doesn't affect the user's session.
const TestSocketName = "rig-test"

// SetupTestSocket configures the test environment to use an isolated tmux socket.
// Call this in TestMain before running tests. This ensures ALL SessionManagers
// (including those created by production code) use the test socket.
func SetupTestSocket() {
	os.Setenv("RIG_TEST_TMUX_SOCKET", TestSocketName)
}

// NewTestSessionManager creates a SessionManager configured for test isolation.
// It uses a separate tmux socket so tests don't affect the user's tmux session.
func NewTestSessionManager(prefix string, windows []WindowConfig) *SessionManager {
	sm := NewSessionManager(prefix, windows, false)
	sm.SocketName = TestSocketName
	return sm
}

// KillTestServer kills the test tmux server, cleaning up all test sessions.
// This should be called in TestMain cleanup.
func KillTestServer() error {
	return exec.Command("tmux", "-L", TestSocketName, "kill-server").Run()
}
