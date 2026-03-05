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
}
