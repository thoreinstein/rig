package config

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zalando/go-keyring"
)

type mockKeyringProvider struct {
	storage map[string]string
	getErr  error
	setErr  error
	delErr  error
}

func newMockKeyringProvider() *mockKeyringProvider {
	return &mockKeyringProvider{
		storage: make(map[string]string),
	}
}

func (m *mockKeyringProvider) Get(service, account string) (string, error) {
	if m.getErr != nil {
		return "", m.getErr
	}
	val, ok := m.storage[service+":"+account]
	if !ok {
		return "", keyring.ErrNotFound
	}
	return val, nil
}

func (m *mockKeyringProvider) Set(service, account, value string) error {
	if m.setErr != nil {
		return m.setErr
	}
	m.storage[service+":"+account] = value
	return nil
}

func (m *mockKeyringProvider) Delete(service, account string) error {
	if m.delErr != nil {
		return m.delErr
	}
	delete(m.storage, service+":"+account)
	return nil
}

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

func TestKeyringProviderInjection(t *testing.T) {
	mock := newMockKeyringProvider()
	SetKeyringProvider(mock)
	t.Cleanup(func() { SetKeyringProvider(nil) })

	service, account, value := "svc", "acc", "pwd"

	_, err := StoreKeychainSecret(service, account, value)
	require.NoError(t, err)
	require.Equal(t, value, mock.storage[service+":"+account])

	got, err := GetKeychainSecret(service, account)
	require.NoError(t, err)
	require.Equal(t, value, got)

	err = DeleteKeychainSecret(service, account)
	require.NoError(t, err)
	require.Empty(t, mock.storage)
}

func TestClassifyKeyringError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected ErrorClass
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: ErrorClassNone,
		},
		{
			name:     "not found (standard)",
			err:      keyring.ErrNotFound,
			expected: ErrorClassNotFound,
		},
		{
			name:     "not found (macOS)",
			err:      errors.New("The specified item could not be found in the keychain. (-25300)"),
			expected: ErrorClassNotFound,
		},
		{
			name:     "permission denied (macOS auth failed)",
			err:      errors.New("Authorization failed. (-25293)"),
			expected: ErrorClassPermission,
		},
		{
			name:     "permission denied (macOS interaction not allowed)",
			err:      errors.New("User interaction is not allowed. (-25308)"),
			expected: ErrorClassPermission,
		},
		{
			name:     "permission denied (Linux locked)",
			err:      errors.New("org.freedesktop.Secret.Error.IsLocked: Access to the secret is denied because the collection is locked"),
			expected: ErrorClassPermission,
		},
		{
			name:     "system error (unknown)",
			err:      errors.New("something went wrong with dbus"),
			expected: ErrorClassUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyKeyringError(tt.err)
			require.Equal(t, tt.expected, got)
		})
	}
}

func TestIsKeychainNotFound(t *testing.T) {
	require.True(t, IsKeychainNotFound(keyring.ErrNotFound))
	require.True(t, IsKeychainNotFound(errors.New("-25300")))
	require.False(t, IsKeychainNotFound(errors.New("-25293")))
	require.False(t, IsKeychainNotFound(nil))
}
