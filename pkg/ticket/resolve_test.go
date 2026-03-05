package ticket

import (
	"fmt"
	"testing"

	"thoreinstein.com/rig/pkg/config"
)

func TestNewProviderWithManager(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		provider string
		wantType string
		wantErr  bool
	}{
		{
			name:     "empty provider returns LocalProvider",
			provider: "",
			wantType: "*ticket.LocalProvider",
		},
		{
			name:     "local provider returns LocalProvider",
			provider: "local",
			wantType: "*ticket.LocalProvider",
		},
		{
			name:     "custom provider with nil manager returns error",
			provider: "custom-plugin",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{
				Ticket: config.TicketConfig{
					Provider: tt.provider,
				},
			}

			got, err := NewProviderWithManager(cfg, nil, t.TempDir(), false)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewProviderWithManager() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if got == nil {
				t.Fatal("NewProviderWithManager() returned nil provider")
			}
			gotType := fmt.Sprintf("%T", got)
			if gotType != tt.wantType {
				t.Errorf("NewProviderWithManager() type = %s, want %s", gotType, tt.wantType)
			}
		})
	}
}
