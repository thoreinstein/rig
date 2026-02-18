package plugin

import (
	"context"
	"reflect"
	"testing"

	"google.golang.org/grpc"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
)

func TestExecutor_Handshake_Logic(t *testing.T) {
	tests := []struct {
		name          string
		initialPlugin *Plugin
		mockResp      *apiv1.HandshakeResponse
		wantPlugin    *Plugin
	}{
		{
			name:          "Modern plugin with all new fields",
			initialPlugin: &Plugin{Name: ""},
			mockResp: &apiv1.HandshakeResponse{
				PluginId:     "test-plugin",
				ApiVersion:   "v1.0.0",
				PluginSemver: "1.2.3",
				Capabilities: []*apiv1.Capability{
					{Name: "git.clone", Version: "1.0.0"},
				},
			},
			wantPlugin: &Plugin{
				Name:       "test-plugin",
				APIVersion: "v1.0.0",
				Version:    "1.2.3",
				Capabilities: []*apiv1.Capability{
					{Name: "git.clone", Version: "1.0.0"},
				},
			},
		},
		{
			name:          "Legacy plugin with deprecated fields",
			initialPlugin: &Plugin{Name: "my-plugin"},
			mockResp: &apiv1.HandshakeResponse{
				PluginVersion:          "0.1.0",
				CapabilitiesDeprecated: []string{"git.clone", "git.push"},
			},
			wantPlugin: &Plugin{
				Name:    "my-plugin", // Name preserved if not returned in new fields and already set
				Version: "0.1.0",
				Capabilities: []*apiv1.Capability{
					{Name: "git.clone", Version: "0.0.0"},
					{Name: "git.push", Version: "0.0.0"},
				},
			},
		},
		{
			name:          "Priority given to new fields",
			initialPlugin: &Plugin{Name: ""},
			mockResp: &apiv1.HandshakeResponse{
				PluginId:               "new-name",
				PluginVersion:          "old-version",
				PluginSemver:           "new-version",
				ApiVersion:             "v1.0.0",
				Capabilities:           []*apiv1.Capability{{Name: "new.cap", Version: "1.0.0"}},
				CapabilitiesDeprecated: []string{"old.cap"},
			},
			wantPlugin: &Plugin{
				Name:       "new-name",
				Version:    "new-version",
				APIVersion: "v1.0.0",
				Capabilities: []*apiv1.Capability{
					{Name: "new.cap", Version: "1.0.0"},
				},
			},
		},
		{
			name: "Empty response clears capabilities",
			initialPlugin: &Plugin{
				Name: "stale-plugin",
				Capabilities: []*apiv1.Capability{
					{Name: "stale.cap", Version: "1.0.0"},
				},
			},
			mockResp: &apiv1.HandshakeResponse{},
			wantPlugin: &Plugin{
				Name:         "stale-plugin",
				Capabilities: nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockPluginServiceClient{
				HandshakeFunc: func(ctx context.Context, in *apiv1.HandshakeRequest, opts ...grpc.CallOption) (*apiv1.HandshakeResponse, error) {
					return tt.mockResp, nil
				},
			}

			p := tt.initialPlugin
			p.client = mockClient

			e := NewExecutor()
			err := e.Handshake(t.Context(), p, "1.0.0", "v1")
			if err != nil {
				t.Fatalf("Handshake() failed: %v", err)
			}

			if p.Name != tt.wantPlugin.Name {
				t.Errorf("Plugin.Name = %q, want %q", p.Name, tt.wantPlugin.Name)
			}
			if p.Version != tt.wantPlugin.Version {
				t.Errorf("Plugin.Version = %q, want %q", p.Version, tt.wantPlugin.Version)
			}
			if p.APIVersion != tt.wantPlugin.APIVersion {
				t.Errorf("Plugin.APIVersion = %q, want %q", p.APIVersion, tt.wantPlugin.APIVersion)
			}

			if !reflect.DeepEqual(p.Capabilities, tt.wantPlugin.Capabilities) {
				t.Errorf("Plugin.Capabilities = %v, want %v", p.Capabilities, tt.wantPlugin.Capabilities)
			}
		})
	}
}
