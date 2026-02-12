package plugin

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
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
	if p.process != nil {
		return errors.Newf("plugin %s is already running", p.Name)
	}

	// 1. Generate unique UDS path
	u, err := uuid.NewRandom()
	if err != nil {
		return errors.Wrap(err, "failed to generate unique identifier for plugin socket")
	}
	// Use shorter name to avoid AF_UNIX path length limits (typically 104-108 chars)
	p.socketPath = filepath.Join(os.TempDir(), fmt.Sprintf("rig-%s.sock", u.String()[:8]))

	// 2. Setup context for cancellation
	ctx, cancel := context.WithCancel(ctx)
	p.cancel = cancel

	// 3. Prepare the command
	// #nosec G204
	cmd := exec.CommandContext(ctx, p.Path)
	cmd.Env = append(os.Environ(), "RIG_PLUGIN_ENDPOINT="+p.socketPath)

	// Ensure we can capture some output if needed for debugging
	cmd.Stderr = os.Stderr

	// 4. Start the process
	if err := cmd.Start(); err != nil {
		p.cancel()
		p.process = nil
		p.cancel = nil
		return errors.Wrapf(err, "failed to start plugin process: %s", p.Name)
	}
	p.process = cmd.Process

	// 5. Handshake: Wait for the socket to appear
	if err := e.waitForSocket(ctx, p.socketPath); err != nil {
		_ = p.process.Kill()
		p.cancel()
		p.process = nil
		p.socketPath = ""
		p.cancel = nil
		return errors.Wrapf(err, "plugin %s failed to establish gRPC server within timeout", p.Name)
	}

	return nil
}

// Stop terminates the plugin process and cleans up resources.
func (e *Executor) Stop(p *Plugin) error {
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
		err = p.process.Kill()
		p.process = nil
	}

	if p.socketPath != "" {
		_ = os.Remove(p.socketPath)
		p.socketPath = ""
	}

	p.client = nil
	return err
}

func (e *Executor) waitForSocket(ctx context.Context, path string) error {
	timeout := time.After(HandshakeTimeout)
	ticker := time.NewTicker(HandshakePollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return errors.New("timeout waiting for plugin socket")
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
