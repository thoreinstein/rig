package daemon

import (
	"os"
	"os/exec"
	"strconv"
	"testing"
)

func TestPIDFile(t *testing.T) {
	// Setup temporary directory for test
	tmpDir := t.TempDir()

	// Override daemonDir for testing
	t.Setenv("XDG_RUNTIME_DIR", tmpDir)

	if err := EnsureDir(); err != nil {
		t.Fatalf("EnsureDir failed: %v", err)
	}

	if err := WritePIDFile(); err != nil {
		t.Fatalf("WritePIDFile failed: %v", err)
	}

	pid, err := ReadPIDFile()
	if err != nil {
		t.Fatalf("ReadPIDFile failed: %v", err)
	}

	if pid != os.Getpid() {
		t.Errorf("expected PID %d, got %d", os.Getpid(), pid)
	}

	if !IsRunning() {
		t.Error("IsRunning returned false, expected true")
	}

	if err := RemovePIDFile(); err != nil {
		t.Fatalf("RemovePIDFile failed: %v", err)
	}

	if _, err := os.Stat(PIDFilePath()); !os.IsNotExist(err) {
		t.Error("PID file still exists after RemovePIDFile")
	}
}

func TestPIDFile_StalePID(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", tmpDir)

	if err := EnsureDir(); err != nil {
		t.Fatalf("EnsureDir failed: %v", err)
	}

	// Get a reliably stale PID by spawning a short-lived process
	cmd := exec.Command("sleep", "0.01")
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start helper process: %v", err)
	}
	stalePid := cmd.Process.Pid
	_ = cmd.Wait()

	// Write the stale PID
	if err := os.WriteFile(PIDFilePath(), []byte(strconv.Itoa(stalePid)), 0o600); err != nil {
		t.Fatalf("failed to write stale PID file: %v", err)
	}

	if IsRunning() {
		t.Errorf("IsRunning returned true for stale PID %d, expected false", stalePid)
	}
}

func TestPIDFile_MalformedPID(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", tmpDir)

	if err := EnsureDir(); err != nil {
		t.Fatalf("EnsureDir failed: %v", err)
	}

	// Write invalid content
	if err := os.WriteFile(PIDFilePath(), []byte("not-a-number"), 0o600); err != nil {
		t.Fatalf("failed to write malformed PID file: %v", err)
	}

	_, err := ReadPIDFile()
	if err == nil {
		t.Error("ReadPIDFile returned nil error for malformed content, expected error")
	}

	if IsRunning() {
		t.Error("IsRunning returned true for malformed PID, expected false")
	}

	_ = RemovePIDFile()
}
