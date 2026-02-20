package daemon

import (
	"os"
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
