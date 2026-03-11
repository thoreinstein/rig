package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	toml "github.com/pelletier/go-toml/v2"
	"github.com/stretchr/testify/require"
	"github.com/zalando/go-keyring"
)

func TestStoreKeychainSecret(t *testing.T) {
	keyring.MockInit()

	service := "test-service"
	account := "test-account"
	value := "secret-value"

	uri, err := StoreKeychainSecret(service, account, value)
	if err != nil {
		t.Fatalf("StoreKeychainSecret failed: %v", err)
	}

	expectedURI := KeychainPrefix + service + "/" + account
	if uri != expectedURI {
		t.Errorf("expected URI %q, got %q", expectedURI, uri)
	}

	got, err := keyring.Get(service, account)
	if err != nil {
		t.Fatalf("failed to get from keyring: %v", err)
	}
	if got != value {
		t.Errorf("expected value %q, got %q", value, got)
	}
}

func TestUpdateKeychainSecret(t *testing.T) {
	mock := newMockKeyringProvider()
	SetKeyringProvider(mock)
	t.Cleanup(func() { SetKeyringProvider(nil) })

	service, account := "rig", "jira.token"

	t.Run("new secret update and rollback", func(t *testing.T) {
		mock.storage = make(map[string]string) // Ensure empty
		uri, rollback, isNew, err := UpdateKeychainSecret(service, account, "new-token")
		require.NoError(t, err)
		require.True(t, isNew)
		require.Equal(t, "keychain://rig/jira.token", uri)
		require.Equal(t, "new-token", mock.storage[service+":"+account])

		// Rollback should delete it
		err = rollback()
		require.NoError(t, err)
		require.Empty(t, mock.storage)
	})

	t.Run("existing secret update and rollback", func(t *testing.T) {
		mock.storage = map[string]string{service + ":" + account: "old-token"}
		uri, rollback, isNew, err := UpdateKeychainSecret(service, account, "new-token")
		require.NoError(t, err)
		require.False(t, isNew)
		require.Equal(t, "keychain://rig/jira.token", uri)
		require.Equal(t, "new-token", mock.storage[service+":"+account])

		// Rollback should restore it
		err = rollback()
		require.NoError(t, err)
		require.Equal(t, "old-token", mock.storage[service+":"+account])
	})

	t.Run("pre-flight failure", func(t *testing.T) {
		mock.storage = make(map[string]string)
		mock.getErr = errors.New("unauthorized (-25293)") // Permission error
		_, _, _, err := UpdateKeychainSecret(service, account, "new-token")
		require.Error(t, err)
		require.Contains(t, err.Error(), "pre-flight check failed")
		mock.getErr = nil
	})
}

func TestStoreConfigValue(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	configFile := filepath.Join(tmpDir, ".config", "rig", "config.toml")

	// 1. Initial store
	if err := StoreConfigValue("notes.path", "~/Notes"); err != nil {
		t.Fatalf("StoreConfigValue failed: %v", err)
	}

	// 2. Update existing key
	if err := StoreConfigValue("notes.path", "~/WorkNotes"); err != nil {
		t.Fatalf("StoreConfigValue (update) failed: %v", err)
	}

	// 3. Store nested key
	if err := StoreConfigValue("jira.token", "token-val"); err != nil {
		t.Fatalf("StoreConfigValue (nested) failed: %v", err)
	}

	// Verify content
	data, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatalf("failed to read config file: %v", err)
	}

	var parsed map[string]interface{}
	if err := toml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to parse config TOML: %v", err)
	}
	notes, _ := parsed["notes"].(map[string]interface{})
	if notes == nil || notes["path"] != "~/WorkNotes" {
		t.Errorf("expected notes.path = ~/WorkNotes, got %v", parsed)
	}
	jira, _ := parsed["jira"].(map[string]interface{})
	if jira == nil || jira["token"] != "token-val" {
		t.Errorf("expected jira.token = token-val, got %v", parsed)
	}

	// 4. Store in a comment-only file
	commentFile := filepath.Join(tmpDir, "comment.toml")
	if err := os.WriteFile(commentFile, []byte("# Just a comment\n"), 0644); err != nil {
		t.Fatalf("failed to write comment file: %v", err)
	}
	// We need to point StoreConfigValue to this file, but it's hardcoded to ~/.config/rig/config.toml.
	// The test already sets HOME to tmpDir, so let's just use that.
	os.Remove(configFile)
	if err := os.WriteFile(configFile, []byte("# Just a comment\n"), 0644); err != nil {
		t.Fatalf("failed to write comment file: %v", err)
	}
	if err := StoreConfigValue("foo.bar", "baz"); err != nil {
		t.Fatalf("StoreConfigValue (comment-only) failed: %v", err)
	}
	data, _ = os.ReadFile(configFile)
	var parsed2 map[string]interface{}
	if err := toml.Unmarshal(data, &parsed2); err != nil {
		t.Fatalf("failed to parse config TOML after comment-only load: %v", err)
	}
	foo, _ := parsed2["foo"].(map[string]interface{})
	if foo == nil || foo["bar"] != "baz" {
		t.Errorf("expected foo.bar = baz, got %v", parsed2)
	}
}

func TestStoreConfigValuePermissions(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	configDir := filepath.Join(tmpDir, ".config", "rig")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	configFile := filepath.Join(configDir, "config.toml")

	// 1. Pre-create file with specific permissions (e.g. 0644)
	if err := os.WriteFile(configFile, []byte("key = \"initial\"\n"), 0644); err != nil {
		t.Fatalf("failed to create config file: %v", err)
	}

	// 2. Update config
	if err := StoreConfigValue("key", "updated"); err != nil {
		t.Fatalf("StoreConfigValue failed: %v", err)
	}

	// 3. Verify permissions are still 0644
	info, err := os.Stat(configFile)
	if err != nil {
		t.Fatalf("failed to stat config file: %v", err)
	}
	if info.Mode().Perm() != 0644 {
		t.Errorf("expected permissions 0644, got %04o", info.Mode().Perm())
	}
}
