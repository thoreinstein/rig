package plugin

import (
	"context"
	"os"
	"strings"
	"testing"
)

type mockExecutor struct {
	startFunc         func(ctx context.Context, p *Plugin) error
	stopFunc          func(p *Plugin) error
	prepareClientFunc func(p *Plugin) error
	handshakeFunc     func(ctx context.Context, p *Plugin, rigVersion, apiVersion string) error
}

func (m *mockExecutor) Start(ctx context.Context, p *Plugin) error {
	if m.startFunc != nil {
		return m.startFunc(ctx, p)
	}
	return nil
}

func (m *mockExecutor) Stop(p *Plugin) error {
	if m.stopFunc != nil {
		return m.stopFunc(p)
	}
	return nil
}

func (m *mockExecutor) PrepareClient(p *Plugin) error {
	if m.prepareClientFunc != nil {
		return m.prepareClientFunc(p)
	}
	return nil
}

func (m *mockExecutor) Handshake(ctx context.Context, p *Plugin, rigVersion, apiVersion string) error {
	if m.handshakeFunc != nil {
		return m.handshakeFunc(ctx, p, rigVersion, apiVersion)
	}
	return nil
}

func TestManager_GetOrStartPlugin_Compatibility(t *testing.T) {
	// Setup a temporary plugin directory
	tmpDir := t.TempDir()
	pluginDir := tmpDir + "/plugins"
	_ = os.Mkdir(pluginDir, 0755)

	// Create a dummy executable
	pluginPath := pluginDir + "/test-plugin"
	_ = os.WriteFile(pluginPath, []byte("#!/bin/sh\necho test"), 0755)

	// Create a manifest that requires a higher Rig version
	manifestPath := pluginDir + "/test-plugin.manifest.yaml"
	_ = os.WriteFile(manifestPath, []byte(`
name: test-plugin
version: 1.0.0
requirements:
  rig: ">= 2.0.0"
`), 0644)

	scanner := &Scanner{Paths: []string{pluginDir}}

	t.Run("Incompatible plugin is rejected", func(t *testing.T) {
		executor := &mockExecutor{
			handshakeFunc: func(ctx context.Context, p *Plugin, rigVersion, apiVersion string) error {
				// Handshake normally updates metadata
				p.Version = "1.0.0"
				return nil
			},
		}

		m := NewManager(&Executor{}, scanner, "1.5.0") // Rig 1.5.0 < 2.0.0
		m.executor = executor                          // Inject mock

		_, err := m.getOrStartPlugin(t.Context(), "test-plugin")
		if err == nil {
			t.Fatal("expected error for incompatible plugin, got nil")
		}

		if !strings.Contains(err.Error(), "incompatible") {
			t.Errorf("expected incompatibility error, got: %v", err)
		}
	})

	t.Run("Compatible plugin is accepted", func(t *testing.T) {
		executor := &mockExecutor{}
		m := NewManager(&Executor{}, scanner, "2.1.0") // Rig 2.1.0 >= 2.0.0
		m.executor = executor

		p, err := m.getOrStartPlugin(t.Context(), "test-plugin")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if p.Name != "test-plugin" {
			t.Errorf("expected plugin name test-plugin, got %q", p.Name)
		}
		if p.Status != StatusCompatible {
			t.Errorf("expected status Compatible, got %q", p.Status)
		}
	})
}
