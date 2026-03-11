package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/pelletier/go-toml/v2"
)

// StoreKeychainSecret stores a value in the system keychain and returns the URI.
func StoreKeychainSecret(service, account, value string) (string, error) {
	if err := getKeyringImpl().Set(service, account, value); err != nil {
		return "", errors.Wrapf(err, "failed to store secret in keychain (%s/%s)", service, account)
	}
	return fmt.Sprintf("%s%s/%s", KeychainPrefix, service, account), nil
}

// UpdateKeychainSecret updates a secret in the keychain and returns a rollback function.
// The rollback function will either restore the previous value or delete the new entry
// if it didn't exist before. The rollback function is only intended to be called once.
func UpdateKeychainSecret(service, account, newValue string) (string, func() error, error) {
	// Capture provider at closure creation time so rollback uses the same impl.
	impl := getKeyringImpl()

	// 1. Pre-flight: Read old value (if any)
	oldValue, err := impl.Get(service, account)
	exists := true
	if err != nil {
		if ClassifyKeyringError(err) == ErrorClassNotFound {
			exists = false
		} else {
			return "", nil, errors.Wrapf(err, "pre-flight check failed for keychain (%s/%s)", service, account)
		}
	}

	// 2. Set new value
	uri, err := StoreKeychainSecret(service, account, newValue)
	if err != nil {
		return "", nil, err
	}

	// 3. Construct rollback closure
	rollback := func() error {
		defer func() { oldValue = "" }() // Zero sensitive data in all paths
		if !exists {
			return impl.Delete(service, account)
		}
		return impl.Set(service, account, oldValue)
	}

	return uri, rollback, nil
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
	defer func() {
		_ = tmpFile.Close()    // Benign double-close after successful path
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
