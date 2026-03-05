package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"
)

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
