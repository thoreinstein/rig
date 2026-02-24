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

	// Ensure the process is either released (on success) or reaped (on error/timeout).
	// This prevents zombie processes if the daemon fails to initialize properly.
	var released bool
	defer func() {
		if !released {
			_ = cmd.Process.Kill()
			_, _ = cmd.Process.Wait()
			_ = RemovePIDFile()
		}
	}()

	// Poll for socket readiness
	path := SocketPath()
	timeout := time.After(5 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeout:
			return nil, errors.New("timeout waiting for daemon to start")
		case <-ticker.C:
			if _, err := os.Stat(path); err == nil {
				// Attempt to connect with retries
				var client *DaemonClient
				var connectErr error
				for range 3 {
					client, connectErr = NewClient(ctx)
					if connectErr == nil {
						// Success! Mark as released and detach the process.
						released = true
						_ = cmd.Process.Release()
						return client, nil
					}
					time.Sleep(100 * time.Millisecond)
				}
				// Connection failed despite socket existing.
				// This might be a stale socket file or the daemon is still binding.
				// Continue waiting instead of returning early to respect the full timeout.
			}
		}
	}
}
