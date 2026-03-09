package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/pelletier/go-toml/v2"
	"github.com/zalando/go-keyring"
)

// StoreKeychainSecret stores a value in the system keychain and returns the URI.
func StoreKeychainSecret(service, account, value string) (string, error) {
	if err := keyring.Set(service, account, value); err != nil {
		return "", errors.Wrapf(err, "failed to store secret in keychain (%s/%s)", service, account)
	}
	return fmt.Sprintf("%s%s/%s", KeychainPrefix, service, account), nil
}

// StoreConfigValue updates a key-value pair in the user's config file.
// If the key is already present, it's updated. If not, it's added.
func StoreConfigValue(key, value string) error {
	// Validate the key to prevent empty segments that would create invalid TOML.
	if key == "" {
		return errors.New("config key must not be empty")
	}
	for _, seg := range strings.Split(key, ".") {
		if seg == "" {
			return errors.Newf("config key %q contains an empty segment", key)
		}
	}
	home, err := UserHomeDir()
	if err != nil {
		return errors.Wrap(err, "failed to get home directory")
	}

	configFile := filepath.Join(home, ".config", "rig", "config.toml")

	// Read existing config
	data, err := os.ReadFile(configFile)
	if err != nil && !os.IsNotExist(err) {
		return errors.Wrapf(err, "failed to read config file %q", configFile)
	}

	var m map[string]interface{}
	if len(data) > 0 {
		if err := toml.Unmarshal(data, &m); err != nil {
			return errors.Wrapf(err, "failed to unmarshal config file %q", configFile)
		}
	}

	if m == nil {
		m = make(map[string]interface{})
		// If file doesn't exist, ensure directory exists
		if _, err := os.Stat(configFile); os.IsNotExist(err) {
			if err := os.MkdirAll(filepath.Dir(configFile), 0700); err != nil {
				return errors.Wrapf(err, "failed to create config directory %q", filepath.Dir(configFile))
			}
		}
	}

	// Set the value (handling dotted keys)
	setRecursive(m, strings.Split(key, "."), value)

	// Write back
	data, err = toml.Marshal(m)
	if err != nil {
		return errors.Wrap(err, "failed to marshal config")
	}

	// Atomic write using temp-file-and-rename pattern
	dir := filepath.Dir(configFile)
	tmpFile, err := os.CreateTemp(dir, "config.*.toml.tmp")
	if err != nil {
		return errors.Wrapf(err, "failed to create temp config file in %q", dir)
	}
	tmpPath := tmpFile.Name()
	closed := false
	defer func() {
		if !closed {
			_ = tmpFile.Close()
		}
		_ = os.Remove(tmpPath) // No-op if rename succeeded
	}()

	// Get original file permissions (default to 0600)
	perm := os.FileMode(0600)
	if info, err := os.Stat(configFile); err == nil {
		perm = info.Mode().Perm()
	}

	// Set permissions before writing data so content is never on disk with wrong mode
	if err := os.Chmod(tmpPath, perm); err != nil {
		return errors.Wrapf(err, "failed to chmod temp config file %q", tmpPath)
	}

	if _, err := tmpFile.Write(data); err != nil {
		return errors.Wrapf(err, "failed to write to temp config file %q", tmpPath)
	}
	if err := tmpFile.Sync(); err != nil {
		return errors.Wrapf(err, "failed to sync temp config file %q", tmpPath)
	}
	if err := tmpFile.Close(); err != nil {
		return errors.Wrapf(err, "failed to close temp config file %q", tmpPath)
	}
	closed = true

	if err := os.Rename(tmpPath, configFile); err != nil {
		return errors.Wrapf(err, "failed to rename temp config file %q to %q", tmpPath, configFile)
	}

	return nil
}

func setRecursive(m map[string]interface{}, keys []string, value interface{}) {
	if len(keys) == 0 {
		return
	}

	if len(keys) == 1 {
		m[keys[0]] = value
		return
	}

	next, ok := m[keys[0]].(map[string]interface{})
	if !ok {
		next = make(map[string]interface{})
		m[keys[0]] = next
	}

	setRecursive(next, keys[1:], value)
}
