package daemon

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
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
		slog.Debug("NewClient failed for existing daemon; removing stale PID file", "path", rigPath, "error", err)
		_ = RemovePIDFile()
	}

	// Start daemon process
	cmd := exec.Command(rigPath, "daemon", "start")
	configureSysProcAttr(cmd)
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		return nil, errors.Wrap(err, "failed to start daemon")
	}

	// Poll for socket readiness
	path := SocketPath()
	timeout := time.After(5 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			_ = cmd.Process.Kill()
			return nil, ctx.Err()
		case <-timeout:
			_ = cmd.Process.Kill()
			_ = os.Remove(path)
			return nil, errors.New("timeout waiting for daemon to start")
		case <-ticker.C:
			if _, err := os.Stat(path); err == nil {
				// Attempt to connect with retries
				var client *DaemonClient
				var connectErr error
				for range 3 {
					client, connectErr = NewClient(ctx)
					if connectErr == nil {
						return client, nil
					}
					time.Sleep(100 * time.Millisecond)
				}
				// All connection attempts failed
				_ = cmd.Process.Kill()
				_ = os.Remove(path)
				return nil, errors.Wrap(connectErr, "daemon started but connection failed")
			}
		}
	}
}
