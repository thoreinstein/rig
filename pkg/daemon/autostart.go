package daemon

import (
	"context"
	"os"
	"os/exec"
	"syscall"
	"time"

	"thoreinstein.com/rig/pkg/errors"
)

// EnsureRunning checks if the daemon is running and starts it if not.
// It returns a connected client on success.
func EnsureRunning(ctx context.Context, rigPath string) (*DaemonClient, error) {
	if IsRunning() {
		client, err := NewClient(ctx)
		if err == nil {
			return client, nil
		}
		// Stale PID? Remove it and start fresh
		_ = RemovePIDFile()
	}

	// Start daemon process
	cmd := exec.Command(rigPath, "daemon", "start")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		return nil, errors.Wrap(err, "failed to start daemon")
	}

	// Poll for socket readiness
	path := SocketPath()
	timeout := time.After(2 * time.Second)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeout:
			return nil, errors.New("timeout waiting for daemon to start")
		case <-ticker.C:
			if _, err := os.Stat(path); err == nil {
				return NewClient(ctx)
			}
		}
	}
}
