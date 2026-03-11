package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/require"
	"github.com/zalando/go-keyring"

	"thoreinstein.com/rig/pkg/config"
	rigerrors "thoreinstein.com/rig/pkg/errors"
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

	// Verify existing keychain entry was restored to its original value on rollback.
	got, err := keyring.Get("rig", "existing.key")
	if err != nil {
		t.Fatalf("expected existing keychain entry to persist, got error: %v", err)
	}
	if got != "existing-val" {
		t.Errorf("expected keychain to contain original value %q after rollback, got %q", "existing-val", got)
	}
}

// mockKeyringProvider is duplicated here from pkg/config to avoid circular dependency
// or exporting it unnecessarily.
type mockKeyringProvider struct {
	storage        map[string]string
	getErr         error
	setErr         error
	setRollbackErr error // Error to return on the second Set call (rollback)
	delErr         error
	setCalls       int // Tracks Set call count for rollback detection
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
	m.setCalls++
	// First Set is the forward write; second Set is rollback (restore).
	if m.setCalls == 1 && m.setErr != nil {
		return m.setErr
	}
	if m.setCalls > 1 && m.setRollbackErr != nil {
		return m.setRollbackErr
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

func TestConfigSetFailureModes(t *testing.T) {
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

	// Use our mock provider for fine-grained failure control
	mock := &mockKeyringProvider{storage: make(map[string]string)}
	config.SetKeyringProvider(mock)
	t.Cleanup(func() { config.SetKeyringProvider(nil) })

	tests := []struct {
		name               string
		args               []string
		setupKeyring       map[string]string
		setupConfig        string
		makeDirRead        bool // if true, make configDir read-only (0500)
		mockGetErr         error
		mockSetErr         error
		mockSetRollbackErr error
		mockDelErr         error
		expectErr          string
		expectSplit        bool
		expectKeyring      map[string]string
	}{
		{
			name:          "initial update failure (no rollback needed)",
			args:          []string{"config", "set", "new.key", "val", "--keychain"},
			mockSetErr:    errors.New("keychain locked"),
			expectErr:     "keychain locked",
			expectKeyring: map[string]string{},
		},
		{
			name:          "set succeeds, config fails, rollback (delete) succeeds",
			args:          []string{"config", "set", "new.key", "val", "--keychain"},
			makeDirRead:   true,
			expectErr:     "failed to update configuration",
			expectKeyring: map[string]string{}, // rolled back
		},
		{
			name:          "set succeeds, config fails, rollback (delete) fails (SplitBrain)",
			args:          []string{"config", "set", "new.key", "val", "--keychain"},
			makeDirRead:   true,
			mockDelErr:    errors.New("delete failed"),
			expectErr:     "split-brain",
			expectSplit:   true,
			expectKeyring: map[string]string{"rig:new.key": "val"}, // orphaned
		},
		{
			name:          "set succeeds, config fails, rollback (restore) succeeds",
			args:          []string{"config", "set", "old.key", "new-val", "--keychain"},
			setupKeyring:  map[string]string{"rig:old.key": "old-val"},
			makeDirRead:   true,
			expectErr:     "failed to update configuration",
			expectKeyring: map[string]string{"rig:old.key": "old-val"}, // restored
		},
		{
			name:               "set succeeds, config fails, rollback (restore) fails (SplitBrain)",
			args:               []string{"config", "set", "old.key", "new-val", "--keychain"},
			setupKeyring:       map[string]string{"rig:old.key": "old-val"},
			makeDirRead:        true,
			mockSetRollbackErr: errors.New("restore failed"), // second set (rollback) fails
			expectErr:          "split-brain",
			expectSplit:        true,
			expectKeyring:      map[string]string{"rig:old.key": "new-val"}, // inconsistent
		},
		{
			name:       "pre-flight read failure (permission denied)",
			args:       []string{"config", "set", "old.key", "new-val", "--keychain"},
			mockGetErr: errors.New("unauthorized (-25293)"),
			expectErr:  "pre-flight check failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Fresh mock per subtest to avoid state leakage
			mock = &mockKeyringProvider{storage: make(map[string]string)}
			for k, v := range tt.setupKeyring {
				mock.storage[k] = v
			}
			mock.getErr = tt.mockGetErr
			mock.setErr = tt.mockSetErr
			mock.setRollbackErr = tt.mockSetRollbackErr
			mock.delErr = tt.mockDelErr
			config.SetKeyringProvider(mock)

			if tt.makeDirRead {
				require.NoError(t, os.Chmod(configDir, 0500))
			} else {
				require.NoError(t, os.Chmod(configDir, 0755))
			}
			t.Cleanup(func() { _ = os.Chmod(configDir, 0755) })

			buf := new(bytes.Buffer)
			prevOut := rootCmd.OutOrStdout()
			t.Cleanup(func() {
				rootCmd.SetOut(prevOut)
				rootCmd.SetArgs(nil)
			})
			rootCmd.SetOut(buf)
			rootCmd.SetArgs(tt.args)

			err := rootCmd.Execute()
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.expectErr)

			if tt.expectSplit {
				require.True(t, rigerrors.IsSplitBrainError(err), "expected SplitBrainError")
			} else {
				require.False(t, rigerrors.IsSplitBrainError(err), "did not expect SplitBrainError")
			}

			if tt.expectKeyring != nil {
				require.Equal(t, tt.expectKeyring, mock.storage)
			}
		})
	}
}
