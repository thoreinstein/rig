package daemon

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"thoreinstein.com/rig/pkg/errors"
)

const (
	daemonDirName = "rig-daemon"
	socketName    = "rig-daemon.sock"
	pidFileName   = "rig-daemon.pid"
)

// SocketPath returns the absolute path to the daemon's Unix Domain Socket.
func SocketPath() string {
	return filepath.Join(daemonDir(), socketName)
}

// PIDFilePath returns the absolute path to the daemon's PID file.
func PIDFilePath() string {
	return filepath.Join(daemonDir(), pidFileName)
}

// daemonDir returns the directory where daemon artifacts are stored.
func daemonDir() string {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = os.TempDir()
	}
	return filepath.Join(dir, daemonDirName)
}

// EnsureDir ensures the daemon directory exists with correct permissions (0700).
func EnsureDir() error {
	dir := daemonDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return errors.Wrapf(err, "failed to create daemon directory %q", dir)
	}
	return os.Chmod(dir, 0o700)
}

// WritePIDFile writes the current process ID to the PID file.
func WritePIDFile() error {
	if err := EnsureDir(); err != nil {
		return err
	}
	pid := os.Getpid()
	return os.WriteFile(PIDFilePath(), []byte(strconv.Itoa(pid)), 0o600)
}

// ReadPIDFile reads the process ID from the PID file.
func ReadPIDFile() (int, error) {
	data, err := os.ReadFile(PIDFilePath())
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

// RemovePIDFile removes the PID file and the socket file.
func RemovePIDFile() error {
	_ = os.Remove(SocketPath())
	return os.Remove(PIDFilePath())
}

// IsRunning checks if the daemon is currently running by verifying the PID file
// and checking if the process exists.
// On Windows, process existence check is best-effort.
func IsRunning() bool {
	pid, err := ReadPIDFile()
	if err != nil {
		return false
	}

	return isProcessRunning(pid)
}
