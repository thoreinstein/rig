package plugin

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type mockExecutor struct {
	start_func          func(ctx context.Context, p *Plugin) error
	stop_func           func(p *Plugin) error
	prepare_client_func func(p *Plugin) error
	handshake_func      func(ctx context.Context, p *Plugin, rigVersion, apiVersion string) error
}

func (m *mockExecutor) Start(ctx context.Context, p *Plugin) error {
	if m.start_func != nil {
		return m.start_func(ctx, p)
	}
	return nil
}

func (m *mockExecutor) Stop(p *Plugin) error {
	if m.stop_func != nil {
		return m.stop_func(p)
	}
	return nil
}

func (m *mockExecutor) PrepareClient(p *Plugin) error {
	if m.prepare_client_func != nil {
		return m.prepare_client_func(p)
	}
	return nil
}

func (m *mockExecutor) Handshake(ctx context.Context, p *Plugin, rigVersion, apiVersion string) error {
	if m.handshake_func != nil {
		return m.handshake_func(ctx, p, rigVersion, apiVersion)
	}
	return nil
}

func TestManager_GetOrStartPlugin_Compatibility(t *testing.T) {
	// Setup a temporary plugin directory
	tmpDir := t.TempDir()
	pluginDir := filepath.Join(tmpDir, "plugins")
	if err := os.Mkdir(pluginDir, 0755); err != nil {
		t.Fatalf("failed to create plugin dir: %v", err)
	}

	// Create a dummy executable
	pluginPath := filepath.Join(pluginDir, "test-plugin")
	if err := os.WriteFile(pluginPath, []byte("#!/bin/sh\necho test"), 0755); err != nil {
		t.Fatalf("failed to write dummy plugin: %v", err)
	}

	// Create a manifest that requires a higher Rig version
	manifestPath := filepath.Join(pluginDir, "test-plugin.manifest.yaml")
	manifestContent := `
name: test-plugin
version: 1.0.0
requirements:
  rig: ">= 2.0.0"
`
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	scanner := &Scanner{Paths: []string{pluginDir}}

	cases := []struct {
		name       string
		rigVersion string
		wantErr    bool
		wantStatus Status
	}{
		{
			name:       "Incompatible plugin is rejected",
			rigVersion: "1.5.0", // Rig 1.5.0 < 2.0.0
			wantErr:    true,
			wantStatus: StatusIncompatible,
		},
		{
			name:       "Compatible plugin is accepted",
			rigVersion: "2.1.0", // Rig 2.1.0 >= 2.0.0
			wantErr:    false,
			wantStatus: StatusCompatible,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var capturedPlugin *Plugin
			executor := &mockExecutor{
				handshake_func: func(ctx context.Context, p *Plugin, rigVersion, apiVersion string) error {
					// Handshake normally updates metadata
					p.Version = "1.0.0"
					capturedPlugin = p
					return nil
				},
			}

			m := NewManager(&Executor{}, scanner, tc.rigVersion)
			m.executor = executor // Inject mock

			p, err := m.getOrStartPlugin(t.Context(), "test-plugin")
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error for incompatible plugin, got nil")
				}
				if !strings.Contains(err.Error(), "incompatible") {
					t.Errorf("expected incompatibility error, got: %v", err)
				}
				// Verify the status was set correctly on the plugin object
				if capturedPlugin != nil && capturedPlugin.Status != tc.wantStatus {
					t.Errorf("expected plugin status %q, got %q", tc.wantStatus, capturedPlugin.Status)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if p.Name != "test-plugin" {
				t.Errorf("expected plugin name test-plugin, got %q", p.Name)
			}
			if p.Status != tc.wantStatus {
				t.Errorf("expected status %q, got %q", tc.wantStatus, p.Status)
			}
		})
	}
}
