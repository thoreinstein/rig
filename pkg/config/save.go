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

	if err := os.WriteFile(configFile, data, 0600); err != nil {
		return errors.Wrapf(err, "failed to write config file %q", configFile)
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
