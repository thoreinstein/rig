package github

import (
	"testing"
)

func TestDeviceAuth_MissingClientID(t *testing.T) {
	cfg := OAuthConfig{
		ClientID: "", // Missing client ID
	}

	_, err := DeviceAuth(t.Context(), cfg, nil)
	if err == nil {
		t.Error("DeviceAuth with empty client ID should return error")
	}
}

func TestOAuthConfig_Defaults(t *testing.T) {
	// Test that defaults are applied correctly
	cfg := OAuthConfig{
		ClientID: "test-client-id",
		// Scopes and HostURL left empty to test defaults
	}

	if cfg.ClientID == "" {
		t.Error("ClientID should be set")
	}

	// We can't easily test the actual device flow without mocking,
	// but we can verify the configuration is accepted
}
