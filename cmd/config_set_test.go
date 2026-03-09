package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"
)

// TestConfigSet exercises the "config set" command. Subtests run sequentially
// because they share global rootCmd state (Cobra commands use package-level vars).
func TestConfigSet(t *testing.T) {
	keyring.MockInit()
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	configDir := filepath.Join(tmpDir, ".config", "rig")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	configFile := filepath.Join(configDir, "config.toml")
	if err := os.WriteFile(configFile, []byte(""), 0600); err != nil {
		t.Fatalf("failed to create config file: %v", err)
	}

	tests := []struct {
		name     string
		args     []string
		expected string
		key      string
		val      string
		keychain bool
	}{
		{
			name:     "set plaintext",
			args:     []string{"config", "set", "notes.path", "~/Notes"},
			expected: "Updated \"notes.path\" in configuration.",
			key:      "notes.path",
			val:      "~/Notes",
		},
		{
			name:     "set keychain",
			args:     []string{"config", "set", "jira.token", "secret-token", "--keychain"},
			expected: "Stored secret for \"jira.token\" in system keychain.",
			key:      "jira.token",
			val:      "keychain://rig/jira.token",
			keychain: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			prevOut := rootCmd.OutOrStdout()
			t.Cleanup(func() {
				rootCmd.SetOut(prevOut)
				rootCmd.SetArgs(nil)
			})
			rootCmd.SetOut(buf)
			rootCmd.SetArgs(tt.args)

			if err := rootCmd.Execute(); err != nil {
				t.Fatalf("Execute failed: %v", err)
			}

			output := buf.String()
			if !strings.Contains(output, tt.expected) {
				t.Errorf("expected output to contain %q, got %q", tt.expected, output)
			}

			// Verify in file
			data, err := os.ReadFile(configFile)
			if err != nil {
				t.Fatalf("failed to read config file: %v", err)
			}
			content := string(data)
			if !strings.Contains(content, tt.val) {
				t.Errorf("config file missing value %q: %s", tt.val, content)
			}

			if tt.keychain {
				got, err := keyring.Get("rig", "jira.token")
				if err != nil {
					t.Fatalf("failed to get from keyring: %v", err)
				}
				if got != "secret-token" {
					t.Errorf("expected secret-token in keyring, got %q", got)
				}
			}
		})
	}
}

func TestConfigSetRollback(t *testing.T) {
	keyring.MockInit()
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	configDir := filepath.Join(tmpDir, ".config", "rig")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	configFile := filepath.Join(configDir, "config.toml")
	// Start with empty config
	if err := os.WriteFile(configFile, []byte(""), 0600); err != nil {
		t.Fatalf("failed to create config file: %v", err)
	}

	// 1. Make config DIRECTORY non-writable so config update fails (at CreateTemp)
	if err := os.Chmod(configDir, 0500); err != nil {
		t.Fatalf("failed to chmod config dir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(configDir, 0755) })

	// Save and restore rootCmd state to prevent cross-test contamination.
	prevOut := rootCmd.OutOrStdout()
	t.Cleanup(func() {
		rootCmd.SetOut(prevOut)
		rootCmd.SetArgs(nil)
	})

	// 2. Try to set keychain value
	args := []string{"config", "set", "fail.key", "fail-val", "--keychain"}
	rootCmd.SetArgs(args)
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected Execute to fail, but it succeeded")
	}

	// 3. Verify error message
	if !strings.Contains(err.Error(), "failed to update configuration") {
		t.Errorf("expected error message to contain %q, got %q", "failed to update configuration", err.Error())
	}

	// 4. Verify keychain entry was deleted (rolled back)
	_, err = keyring.Get("rig", "fail.key")
	if err == nil {
		t.Error("expected keychain entry to be deleted, but it exists")
	}

	// 5. Verify pre-existing entry is NOT rolled back (Phase 2 constraint)
	// First, set a successful entry
	if err = os.Chmod(configDir, 0755); err != nil {
		t.Fatalf("failed to restore config dir permissions: %v", err)
	}
	args = []string{"config", "set", "existing.key", "existing-val", "--keychain"}
	rootCmd.SetArgs(args)
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("setup Execute failed: %v", err)
	}

	// Now try to update it with failure
	if err = os.Chmod(configDir, 0500); err != nil {
		t.Fatalf("failed to chmod config dir: %v", err)
	}
	args = []string{"config", "set", "existing.key", "new-val", "--keychain"}
	rootCmd.SetArgs(args)
	err = rootCmd.Execute()
	if err == nil {
		t.Fatal("expected update Execute to fail")
	}

	// Verify old value still exists in keychain (it shouldn't have been deleted by rollback)
	got, err := keyring.Get("rig", "existing.key")
	if err != nil {
		t.Fatalf("expected existing keychain entry to persist, got error: %v", err)
	}
	// Note: StoreKeychainSecret updates the value in keyring before StoreConfigValue is called.
	// For existing entries, the keychain value is overwritten even if config write fails.
	// This is a known data-loss edge case scoped for Phase 2 (rig-hka.2: save-and-restore).
	if got != "new-val" {
		t.Errorf("expected keychain to contain updated value %q, got %q", "new-val", got)
	}
}
