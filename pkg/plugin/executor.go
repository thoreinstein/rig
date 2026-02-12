package plugin

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"thoreinstein.com/rig/pkg/errors"
)

const (
	// HandshakeTimeout is the maximum time to wait for a plugin to start and establish a socket.
	HandshakeTimeout = 5 * time.Second
	// HandshakePollInterval is how often to check for the existence of the UDS socket.
	HandshakePollInterval = 100 * time.Millisecond
)

// Executor manages the lifecycle of a plugin process.
type Executor struct{}

// NewExecutor creates a new plugin executor.
func NewExecutor() *Executor {
	return &Executor{}
}

// Start launches the plugin process and establishes the IPC handshake.
func (e *Executor) Start(ctx context.Context, p *Plugin) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.process != nil {
		return errors.NewPluginError(p.Name, "Start", "plugin is already running")
	}

	// 1. Generate unique UDS path
	u, err := uuid.NewRandom()
	if err != nil {
		return errors.NewPluginError(p.Name, "Start", "failed to generate unique identifier for plugin socket").WithCause(err)
	}
	// Use shorter name to avoid AF_UNIX path length limits (typically 104-108 chars)
	p.socketPath = filepath.Join(os.TempDir(), fmt.Sprintf("rig-%s.sock", u.String()[:8]))

	// 2. Setup internal context for the process lifecycle
	// We don't shadow the incoming ctx so waitForSocket can respect its deadline.
	procCtx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel

	// 3. Prepare the command
	// #nosec G204
	cmd := exec.CommandContext(procCtx, p.Path)
	cmd.Env = append(os.Environ(), "RIG_PLUGIN_ENDPOINT="+p.socketPath)

	// Ensure we can capture some output if needed for debugging
	cmd.Stderr = os.Stderr

	// 4. Start the process
	if err := cmd.Start(); err != nil {
		_ = p.cleanup()
		return errors.NewPluginError(p.Name, "Start", "failed to launch plugin process").WithCause(err)
	}
	p.process = cmd.Process

	// 5. Handshake: Wait for the socket to appear
	if err := e.waitForSocket(ctx, p.socketPath); err != nil {
		_ = p.cleanup()
		return errors.NewPluginError(p.Name, "Start", "handshake failed").WithCause(err)
	}

	return nil
}

// Stop terminates the plugin process and cleans up resources.
func (e *Executor) Stop(p *Plugin) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.cleanup()
}

// cleanup performs resource cleanup for a plugin. mu must be held by the caller.
func (p *Plugin) cleanup() error {
	if p.cancel != nil {
		p.cancel()
		p.cancel = nil
	}

	if p.conn != nil {
		_ = p.conn.Close()
		p.conn = nil
	}

	var err error
	if p.process != nil {
		// Signal termination
		_ = p.process.Kill()
		// Wait to prevent zombies
		_, err = p.process.Wait()
		p.process = nil
	}

	if p.socketPath != "" {
		_ = os.Remove(p.socketPath)
		p.socketPath = ""
	}

	p.client = nil
	return err
}

// waitForSocket waits for the plugin's Unix Domain Socket to be created and becomes ready
// for connections, respecting the provided context's deadline and HandshakeTimeout.
func (e *Executor) waitForSocket(ctx context.Context, path string) error {
	// Create a combined deadline: use HandshakeTimeout unless ctx has an earlier one.
	handshakeDeadline := time.Now().Add(HandshakeTimeout)
	if d, ok := ctx.Deadline(); ok && d.Before(handshakeDeadline) {
		handshakeDeadline = d
	}

	ticker := time.NewTicker(HandshakePollInterval)
	defer ticker.Stop()

	remaining := time.Until(handshakeDeadline)
	if remaining <= 0 {
		return errors.NewPluginError("", "Handshake", "timeout waiting for plugin socket")
	}
	timer := time.NewTimer(remaining)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return errors.NewPluginError("", "Handshake", "timeout waiting for plugin socket")
		case <-ticker.C:
			if _, err := os.Stat(path); err == nil {
				// Socket file exists, try to dial it to ensure it's ready
				conn, err := net.DialTimeout("unix", path, HandshakePollInterval)
				if err == nil {
					conn.Close()
					return nil
				}
			}
		}
	}
}
