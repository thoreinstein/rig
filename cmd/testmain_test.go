package cmd

import (
	"os"
	"testing"

	"thoreinstein.com/rig/pkg/tmux"
)

// TestMain handles test setup and teardown for the cmd package.
// It uses socket-based tmux isolation so tests don't affect the user's session.
func TestMain(m *testing.M) {
	// Signal that we are in a test environment
	os.Setenv("GO_TEST", "true")

	// Configure all SessionManagers to use isolated test socket
	tmux.SetupTestSocket()

	// Run all tests
	code := m.Run()

	// Clean up test tmux server
	_ = tmux.KillTestServer()

	os.Exit(code)
}
