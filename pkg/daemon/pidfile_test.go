package daemon

import (
	"os"
	"os/exec"
	"strconv"
	"testing"
)

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	// Exit immediately to create a stale PID
	os.Exit(0)
}

func TestPIDFile_TableDriven(t *testing.T) {
	tests := []struct {
		name     string
		preWrite func(tmpDir string) error
		check    func(t *testing.T)
	}{
		{
			name: "HappyPath",
			check: func(t *testing.T) {
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
			},
		},
		{
			name: "StalePID",
			check: func(t *testing.T) {
				// Get a reliably stale PID by spawning a short-lived process
				// #nosec G204 - Intentional spawning of test binary for helper process
				cmd := exec.Command(os.Args[0], "-test.run=TestHelperProcess")
				cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
				if err := cmd.Start(); err != nil {
					t.Fatalf("failed to start helper process: %v", err)
				}
				stalePid := cmd.Process.Pid
				_ = cmd.Wait()

				if err := os.WriteFile(PIDFilePath(), []byte(strconv.Itoa(stalePid)), 0o600); err != nil {
					t.Fatalf("failed to write stale PID file: %v", err)
				}

				if IsRunning() {
					t.Errorf("IsRunning returned true for stale PID %d, expected false", stalePid)
				}
			},
		},
		{
			name: "MalformedPID",
			check: func(t *testing.T) {
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
			},
		},
		{
			name:     "MissingDir",
			preWrite: os.RemoveAll,
			check: func(t *testing.T) {
				// ReadPIDFile should fail if dir is missing
				_, err := ReadPIDFile()
				if err == nil {
					t.Error("ReadPIDFile returned nil error for missing directory, expected error")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			t.Setenv("XDG_RUNTIME_DIR", tmpDir)

			if err := EnsureDir(); err != nil {
				t.Fatalf("EnsureDir failed: %v", err)
			}

			if tt.preWrite != nil {
				if err := tt.preWrite(tmpDir); err != nil {
					t.Fatal(err)
				}
			}

			tt.check(t)
			_ = RemovePIDFile()
		})
	}
}
