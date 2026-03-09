package plugin

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
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

	e := NewExecutor()

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
	if err := e.Stop(p); err != nil {
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

func TestExecutor_Start_TimeoutRetry(t *testing.T) {
	tmpDir := t.TempDir()

	// Compile the mock plugin binary
	pluginBin := filepath.Join(tmpDir, "mock-plugin-bin-timeout")
	cmd := exec.Command("go", "build", "-o", pluginBin, "testdata/mock_plugin.go")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to compile mock plugin: %v\nOutput: %s", err, string(out))
	}

	p := &Plugin{
		Name: "timeout-plugin",
		Path: pluginBin,
	}

	e := NewExecutor()

	// First attempt: already-expired context should fail
	expiredCtx, expiredCancel := context.WithTimeout(t.Context(), 0)
	expiredCancel()

	err := e.Start(expiredCtx, p)
	if err == nil {
		_ = e.Stop(p)
		t.Fatal("expected Start to fail with expired context")
	}

	// Second attempt: normal timeout should succeed (no stale "already running" state)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	if err := e.Start(ctx, p); err != nil {
		t.Fatalf("second Start() failed: %v (stale state from first attempt?)", err)
	}

	if err := e.Stop(p); err != nil {
		t.Errorf("Stop() failed: %v", err)
	}
}

func TestExecutor_EnvSanitization(t *testing.T) {
	tmpDir := t.TempDir()

	// Compile the mock plugin binary
	pluginBin := filepath.Join(tmpDir, "mock-plugin-bin-env")
	cmd := exec.Command("go", "build", "-o", pluginBin, "testdata/mock_plugin.go")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to compile mock plugin: %v\nOutput: %s", err, string(out))
	}

	// Set some test environment variables on the host
	t.Setenv("RIG_SECRET", "should-be-blocked")
	t.Setenv("RIG_ALLOWED_GLOBAL", "should-be-allowed")
	t.Setenv("RIG_ALLOWED_PLUGIN", "should-be-allowed")
	t.Setenv("RIG_PREFIX_1", "prefix-1")
	t.Setenv("RIG_PREFIX_2", "prefix-2")

	p := &Plugin{
		Name:         "env-plugin",
		Path:         pluginBin,
		EnvAllowList: []string{"RIG_ALLOWED_PLUGIN", "RIG_PREFIX_*"},
	}

	e := NewExecutor()
	e.SetGlobalEnvAllowList([]string{"RIG_ALLOWED_GLOBAL"})

	// Tell the mock plugin what to expect via special env vars that WE pass during Start
	p.EnvAllowList = append(p.EnvAllowList, "EXPECTED_ENV_VARS", "BLOCKED_ENV_VARS")

	t.Setenv("EXPECTED_ENV_VARS", "PATH,RIG_ALLOWED_GLOBAL,RIG_ALLOWED_PLUGIN,RIG_PREFIX_1,RIG_PREFIX_2")
	t.Setenv("BLOCKED_ENV_VARS", "RIG_SECRET")

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	if err := e.Start(ctx, p); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	if err := e.Stop(p); err != nil {
		t.Errorf("Stop() failed: %v", err)
	}
}

func TestExecutor_HostEndpoint(t *testing.T) {
	tmpDir := t.TempDir()
	hostSocket := filepath.Join(tmpDir, "host.sock")

	// Compile the mock plugin binary once to avoid 'go run' overhead in CI
	pluginBin := filepath.Join(tmpDir, "mock-plugin-bin")
	cmd := exec.Command("go", "build", "-o", pluginBin, "testdata/mock_plugin.go")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to compile mock plugin: %v\nOutput: %s", err, string(out))
	}

	p := &Plugin{
		Name:     "env-plugin",
		Path:     pluginBin,
		hostPath: hostSocket,
	}

	// Tell the mock plugin what to expect
	t.Setenv("EXPECTED_HOST_ENDPOINT", hostSocket)
	t.Setenv("EXPECTED_ENV_VARS", "RIG_HOST_ENDPOINT")
	// We need to allow these through sanitization for the test to work
	p.EnvAllowList = []string{"EXPECTED_HOST_ENDPOINT", "EXPECTED_ENV_VARS"}

	e := NewExecutor()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	if err := e.Start(ctx, p); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	if err := e.Stop(p); err != nil {
		t.Errorf("Stop() failed: %v", err)
	}
}
