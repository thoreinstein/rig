package sdk

import (
	"testing"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
)

type mockPlugin struct {
	info Info
}

func (m *mockPlugin) Info() Info {
	return m.info
}

func TestPluginBridge_Handshake(t *testing.T) {
	info := Info{
		ID:         "test-plugin",
		APIVersion: "v1",
		SemVer:     "1.2.3",
		Capabilities: []Capability{
			{Name: "test-cap", Version: "1.0.0"},
		},
		Commands: []CommandDescriptor{
			{Name: "test-cmd", Short: "test short"},
		},
	}

	bridge := newPluginBridge(&mockPlugin{info: info})
	resp, err := bridge.Handshake(t.Context(), &apiv1.HandshakeRequest{})
	if err != nil {
		t.Fatalf("Handshake failed: %v", err)
	}

	if resp.PluginId != info.ID {
		t.Errorf("got PluginId %q, want %q", resp.PluginId, info.ID)
	}
	if resp.ApiVersion != info.APIVersion {
		t.Errorf("got ApiVersion %q, want %q", resp.ApiVersion, info.APIVersion)
	}
	if resp.PluginSemver != info.SemVer {
		t.Errorf("got PluginSemver %q, want %q", resp.PluginSemver, info.SemVer)
	}
	if len(resp.Capabilities) != 1 || resp.Capabilities[0].Name != "test-cap" {
		t.Errorf("got unexpected capabilities: %v", resp.Capabilities)
	}
	if len(resp.Commands) != 1 || resp.Commands[0].Name != "test-cmd" {
		t.Errorf("got unexpected commands: %v", resp.Commands)
	}
}
