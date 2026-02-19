package plugin

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExecutor_StartStop(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not found, skipping test")
	}
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not found, skipping test")
	}

	tmpDir := t.TempDir()

	// Create a script that acts as a plugin and creates a UDS socket
	pluginPath := filepath.Join(tmpDir, "mock-plugin")
	// Use python3 to create a real listening UDS socket
	script := `#!/bin/bash
if [ -n "$RIG_PLUGIN_ENDPOINT" ]; then
    python3 -c "import socket, time; s = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM); s.bind('$RIG_PLUGIN_ENDPOINT'); s.listen(1); time.sleep(5)"
    exit 0
fi
exit 1
`
	if err := os.WriteFile(pluginPath, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	p := &Plugin{
		Name: "test-plugin",
		Path: pluginPath,
	}

	e := NewExecutor("")

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	// Start the plugin
	err := e.Start(ctx, p)
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// Verify runtime state
	if p.process == nil {
		t.Error("p.process is nil after Start")
	}
	if p.socketPath == "" {
		t.Error("p.socketPath is empty after Start")
	}
	if _, err := os.Stat(p.socketPath); os.IsNotExist(err) {
		t.Errorf("socket file %s does not exist", p.socketPath)
	}

	// Stop the plugin
	savedSocket := p.socketPath
	err = e.Stop(p)
	if err != nil {
		t.Errorf("Stop() failed: %v", err)
	}

	// Verify cleanup
	if p.process != nil {
		t.Error("p.process is not nil after Stop")
	}
	if p.socketPath != "" {
		t.Error("p.socketPath is not empty after Stop")
	}
	if _, err := os.Stat(savedSocket); err == nil {
		t.Errorf("socket file %s still exists after Stop", savedSocket)
	}
}

func TestExecutor_Start_Timeout(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not found, skipping test")
	}

	// Create a plugin that will never create a socket
	tmpDir := t.TempDir()
	pluginPath := filepath.Join(tmpDir, "slow-plugin")
	script := `#!/bin/bash
sleep 10
`
	if err := os.WriteFile(pluginPath, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	p := &Plugin{
		Name: "timeout-plugin",
		Path: pluginPath,
	}

	e := NewExecutor("")

	// Set short timeout
	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	defer cancel()

	err := e.Start(ctx, p)
	if err == nil {
		t.Error("expected timeout error, got nil")
	}

	// Verify that we can retry after a failure
	// If the bug exists, this will fail with "already running"
	ctx2, cancel2 := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel2()
	err = e.Start(ctx2, p)
	if err == nil {
		t.Error("expected second timeout error, got nil")
	}
	if err != nil && strings.Contains(err.Error(), "already running") {
		t.Errorf("Stale state detected: got %v, want timeout error", err)
	}
}
