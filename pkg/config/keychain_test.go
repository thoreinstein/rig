package config

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zalando/go-keyring"
)

func TestKeychainSecrets(t *testing.T) {
	keyring.MockInit()

	tests := []struct {
		name    string
		service string
		account string
		value   string
	}{
		{
			name:    "basic secret",
			service: "test-service",
			account: "test-account",
			value:   "secret-value",
		},
		{
			name:    "empty value",
			service: "test-service",
			account: "empty-account",
			value:   "",
		},
		{
			name:    "special characters",
			service: "test-service",
			account: "special.key",
			value:   "p@ss/w0rd!",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set secret
			err := keyring.Set(tt.service, tt.account, tt.value)
			require.NoError(t, err, "keyring.Set")

			// Get secret
			got, err := GetKeychainSecret(tt.service, tt.account)
			require.NoError(t, err, "GetKeychainSecret")
			require.Equal(t, tt.value, got)

			// Delete secret
			err = DeleteKeychainSecret(tt.service, tt.account)
			require.NoError(t, err, "DeleteKeychainSecret")

			// Verify deletion
			_, err = GetKeychainSecret(tt.service, tt.account)
			require.Error(t, err, "expected error after deletion")
		})
	}
}
