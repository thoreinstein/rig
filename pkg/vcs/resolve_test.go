package vcs

import (
	"fmt"
	"testing"
)

func TestNewProviderWithManager(t *testing.T) {
	tests := []struct {
		name         string
		providerName string
		wantType     string
		wantErr      bool
	}{
		{
			name:         "empty provider returns local",
			providerName: "",
			wantType:     "*vcs.LocalProvider",
		},
		{
			name:         "git provider returns local",
			providerName: "git",
			wantType:     "*vcs.LocalProvider",
		},
		{
			name:         "custom provider without manager returns error",
			providerName: "my-custom-vcs",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewProviderWithManager(nil, tt.providerName, false)
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
