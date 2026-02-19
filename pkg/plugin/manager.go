package plugin

import (
	"context"
	"sync"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
	"thoreinstein.com/rig/pkg/errors"
)

type pluginExecutor interface {
	Start(ctx context.Context, p *Plugin) error
	Stop(p *Plugin) error
	PrepareClient(p *Plugin) error
	Handshake(ctx context.Context, p *Plugin, rigVersion, apiVersion string) error
}

// Manager manages a pool of active plugins.
type Manager struct {
	executor   pluginExecutor
	scanner    *Scanner
	rigVersion string

	mu      sync.Mutex
	plugins map[string]*Plugin
}

// NewManager creates a new plugin manager.
func NewManager(executor *Executor, scanner *Scanner, rigVersion string) *Manager {
	return &Manager{
		executor:   executor,
		scanner:    scanner,
		rigVersion: rigVersion,
		plugins:    make(map[string]*Plugin),
	}
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

	// Perform handshake with host version and API contract version
	if err := m.executor.Handshake(ctx, target, m.rigVersion, APIVersion); err != nil {
		_ = m.executor.Stop(target)
		return nil, errors.Wrapf(err, "handshake failed for plugin %q", name)
	}

	// Validate compatibility with host Rig version after handshake
	// (which might have updated the plugin's metadata/version).
	ValidateCompatibility(target, m.rigVersion)
	if target.Status != StatusCompatible {
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

// StopAll stops all managed plugins.
func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, p := range m.plugins {
		_ = m.executor.Stop(p)
	}
	m.plugins = make(map[string]*Plugin)
}
