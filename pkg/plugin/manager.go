package plugin

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"

	"github.com/google/uuid"
	"google.golang.org/grpc"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
	"thoreinstein.com/rig/pkg/errors"
	"thoreinstein.com/rig/pkg/ui"
)

type pluginExecutor interface {
	Start(ctx context.Context, p *Plugin) error
	Stop(p *Plugin) error
	PrepareClient(p *Plugin) error
	Handshake(ctx context.Context, p *Plugin, rigVersion, apiVersion string, configJSON []byte) error
	SetHostEndpoint(path string)
}

// ConfigProvider is a function that returns the JSON-serialized configuration for a plugin.
type ConfigProvider func(pluginName string) ([]byte, error)

// Manager manages a pool of active plugins.
type Manager struct {
	executor       pluginExecutor
	scanner        *Scanner
	rigVersion     string
	configProvider ConfigProvider

	// Host-side UI Proxy Service
	hostServer *grpc.Server
	hostUI     *ui.UIServer
	hostL      net.Listener
	hostPath   string
	hostDir    string

	mu      sync.Mutex
	plugins map[string]*Plugin
}

// NewManager creates a new plugin manager and starts the host-side UI Proxy Service.
func NewManager(executor *Executor, scanner *Scanner, rigVersion string, configProvider ConfigProvider) (*Manager, error) {
	// 1. Create a private directory and generate unique UDS path for the host server
	hostDir, err := os.MkdirTemp("", "rig-h-")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create temporary directory for host socket")
	}
	if err := os.Chmod(hostDir, 0o700); err != nil {
		_ = os.RemoveAll(hostDir)
		return nil, errors.Wrap(err, "failed to set permissions on temporary directory for host socket")
	}

	u, err := uuid.NewRandom()
	if err != nil {
		_ = os.RemoveAll(hostDir)
		return nil, errors.Wrap(err, "failed to generate unique identifier for host socket")
	}
	hostPath := filepath.Join(hostDir, fmt.Sprintf("rig-h-%s.sock", u.String()[:8]))

	// 2. Start host gRPC server
	lis, err := net.Listen("unix", hostPath)
	if err != nil {
		_ = os.RemoveAll(hostDir)
		return nil, errors.Wrapf(err, "failed to listen on host socket %q", hostPath)
	}
	// Restrict socket permissions to the current user only.
	if err := os.Chmod(hostPath, 0o600); err != nil {
		lis.Close()
		_ = os.RemoveAll(hostDir)
		return nil, errors.Wrapf(err, "failed to set permissions on host socket %q", hostPath)
	}

	uiServer := ui.NewUIServer()
	srv := grpc.NewServer()
	apiv1.RegisterUIServiceServer(srv, uiServer)

	go func() {
		if err := srv.Serve(lis); err != nil && err != grpc.ErrServerStopped {
			// In a real CLI, we might log this or handle it more gracefully
			fmt.Fprintf(os.Stderr, "Host UI server failed: %v\n", err)
		}
	}()

	// 3. Configure executor with host endpoint
	executor.SetHostEndpoint(hostPath)

	return &Manager{
		executor:       executor,
		scanner:        scanner,
		rigVersion:     rigVersion,
		configProvider: configProvider,
		hostServer:     srv,
		hostUI:         uiServer,
		hostL:          lis,
		hostPath:       hostPath,
		hostDir:        hostDir,
		plugins:        make(map[string]*Plugin),
	}, nil
}

// GetAssistantClient returns a gRPC client for the specified assistant plugin.
// If the plugin is not running, it will be started.
func (m *Manager) GetAssistantClient(ctx context.Context, name string) (apiv1.AssistantServiceClient, error) {
	p, err := m.getOrStartPlugin(ctx, name)
	if err != nil {
		return nil, err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Verify the plugin has the assistant capability
	hasAssistant := false
	for _, cap := range p.Capabilities {
		if cap.Name == AssistantCapability {
			hasAssistant = true
			break
		}
	}

	if !hasAssistant {
		return nil, errors.NewPluginError(name, "GetAssistantClient", "plugin does not support assistant capability")
	}

	if p.AssistantClient == nil {
		if p.conn == nil {
			return nil, errors.NewPluginError(name, "GetAssistantClient", "plugin connection not established")
		}
		p.AssistantClient = apiv1.NewAssistantServiceClient(p.conn)
	}

	return p.AssistantClient, nil
}

func (m *Manager) getOrStartPlugin(ctx context.Context, name string) (*Plugin, error) {
	m.mu.Lock()
	p, ok := m.plugins[name]
	m.mu.Unlock()

	if ok {
		p.mu.Lock()
		running := p.process != nil
		p.mu.Unlock()
		if running {
			return p, nil
		}
	}

	// Not running or not found, try to discover it
	result, err := m.scanner.Scan()
	if err != nil {
		return nil, errors.Wrap(err, "failed to scan plugins")
	}

	var target *Plugin
	for _, found := range result.Plugins {
		if found.Name == name {
			target = found
			break
		}
	}

	if target == nil {
		return nil, errors.NewPluginError(name, "Discovery", "plugin not found")
	}

	// Start the plugin
	if err := m.executor.Start(ctx, target); err != nil {
		return nil, errors.Wrapf(err, "failed to start plugin %q", name)
	}

	// Prepare the base client and handshake
	if err := m.executor.PrepareClient(target); err != nil {
		_ = m.executor.Stop(target)
		return nil, errors.Wrapf(err, "failed to prepare client for plugin %q", name)
	}

	// Fetch plugin configuration if provider is available
	var configJSON []byte
	if m.configProvider != nil {
		var err error
		configJSON, err = m.configProvider(name)
		if err != nil {
			// Don't fail if config provider fails, just use empty config
			configJSON = []byte("{}")
		}
	}

	// Perform handshake with host version and API contract version
	if err := m.executor.Handshake(ctx, target, m.rigVersion, APIVersion, configJSON); err != nil {
		_ = m.executor.Stop(target)
		return nil, errors.Wrapf(err, "handshake failed for plugin %q", name)
	}

	// Validate compatibility with host Rig version after handshake
	// (which might have updated the plugin's metadata/version).
	ValidateCompatibility(target, m.rigVersion)
	if target.Status == StatusIncompatible || target.Status == StatusError {
		_ = m.executor.Stop(target)
		if target.Error != nil {
			return nil, errors.Wrapf(target.Error, "plugin %q is incompatible", name)
		}
		return nil, errors.NewPluginError(name, "Compatibility", "plugin is incompatible with this version of rig")
	}

	m.mu.Lock()
	m.plugins[name] = target
	m.mu.Unlock()

	return target, nil
}

// StopAll stops all managed plugins and the host UI server.
func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, p := range m.plugins {
		_ = m.executor.Stop(p)
	}
	m.plugins = make(map[string]*Plugin)

	if m.hostServer != nil {
		m.hostServer.GracefulStop()
		m.hostServer = nil
	}

	if m.hostUI != nil {
		m.hostUI.Stop()
		m.hostUI = nil
	}

	if m.hostL != nil {
		_ = m.hostL.Close()
		m.hostL = nil
	}

	if m.hostDir != "" {
		_ = os.RemoveAll(m.hostDir)
		m.hostDir = ""
		m.hostPath = ""
	}

	// Reset host endpoint in executor to avoid stale environment variables
	m.executor.SetHostEndpoint("")
}
