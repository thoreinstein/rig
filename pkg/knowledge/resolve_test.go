package knowledge

import (
	"reflect"
	"testing"

	"thoreinstein.com/rig/pkg/config"
	"thoreinstein.com/rig/pkg/plugin"
)

func TestNewProviderWithManager(t *testing.T) {
	tests := []struct {
		name         string
		providerName string
		nilManager   bool
		wantType     string
		wantErr      bool
	}{
		{
			name:         "Default is local",
			providerName: "",
			wantType:     "*knowledge.LocalProvider",
		},
		{
			name:         "Explicit local",
			providerName: "local",
			wantType:     "*knowledge.LocalProvider",
		},
		{
			name:         "Plugin provider",
			providerName: "obsidian-plugin",
			wantType:     "*knowledge.PluginProvider",
		},
		{
			name:         "Plugin without manager",
			providerName: "foo",
			nilManager:   true,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Notes: config.NotesConfig{
					Provider: tt.providerName,
				},
			}

			var mgr PluginManager
			if !tt.nilManager && tt.providerName != "" && tt.providerName != "local" {
				mgr = &plugin.Manager{} // Non-nil dummy
			}

			got, err := NewProviderWithManager(cfg, mgr, false)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewProviderWithManager() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil {
				gotType := reflect.TypeOf(got).String()
				if gotType != tt.wantType {
					t.Errorf("NewProviderWithManager() got type = %v, want %v", gotType, tt.wantType)
				}
			}
		})
	}
}
