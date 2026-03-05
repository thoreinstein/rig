package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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

	content := string(data)
	if !strings.Contains(content, `path = '~/WorkNotes'`) {
		t.Errorf("config missing updated notes.path: %s", content)
	}
	if !strings.Contains(content, `token = 'token-val'`) {
		t.Errorf("config missing jira.token: %s", content)
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
	if !strings.Contains(string(data), `bar = 'baz'`) {
		t.Errorf("config missing foo.bar after comment-only load: %s", string(data))
	}
}
