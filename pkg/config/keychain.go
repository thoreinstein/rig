package config

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/zalando/go-keyring"
)

const KeychainPrefix = "keychain://"

// ErrorClass categorizes keychain errors for higher-level decision making.
type ErrorClass int

const (
	ErrorClassNone    ErrorClass = iota // Zero value: nil input
	ErrorClassUnknown                   // Non-nil error that doesn't match any known pattern
	ErrorClassTransient
	ErrorClassPermission
	ErrorClassSystem
	ErrorClassNotFound
)

// KeyringProvider defines the interface for interacting with the system keychain.
type KeyringProvider interface {
	Get(service, account string) (string, error)
	Set(service, account, value string) error
	Delete(service, account string) error
}

// DefaultKeyringProvider delegates to the zalando/go-keyring package.
type DefaultKeyringProvider struct{}

func (p *DefaultKeyringProvider) Get(service, account string) (string, error) {
	return keyring.Get(service, account)
}

func (p *DefaultKeyringProvider) Set(service, account, value string) error {
	return keyring.Set(service, account, value)
}

func (p *DefaultKeyringProvider) Delete(service, account string) error {
	return keyring.Delete(service, account)
}

var (
	keyringMu   sync.RWMutex
	keyringImpl KeyringProvider = &DefaultKeyringProvider{}
)

// getKeyringImpl returns the current keyring provider under a read lock.
func getKeyringImpl() KeyringProvider {
	keyringMu.RLock()
	defer keyringMu.RUnlock()
	return keyringImpl
}

// SetKeyringProvider allows injecting a mock provider for testing.
// Passing nil resets to the default provider.
func SetKeyringProvider(p KeyringProvider) {
	keyringMu.Lock()
	defer keyringMu.Unlock()
	if p == nil {
		keyringImpl = &DefaultKeyringProvider{}
		return
	}
	keyringImpl = p
}

// ClassifyKeyringError maps OS-specific keychain errors to internal ErrorClass.
func ClassifyKeyringError(err error) ErrorClass {
	if err == nil {
		return ErrorClassNone
	}

	if errors.Is(err, keyring.ErrNotFound) {
		return ErrorClassNotFound
	}

	msg := strings.ToLower(err.Error())

	// macOS Security framework errors
	// errSecAuthFailed (-25293), errSecInteractionNotAllowed (-25308)
	if strings.Contains(msg, "-25293") || strings.Contains(msg, "-25308") || strings.Contains(msg, "auth failed") {
		return ErrorClassPermission
	}
	if strings.Contains(msg, "-25300") { // errSecItemNotFound
		return ErrorClassNotFound
	}

	// Linux Secret Service (libsecret/dbus) errors
	if strings.Contains(msg, "org.freedesktop.secret.error.islocked") || strings.Contains(msg, "access denied") {
		return ErrorClassPermission
	}

	// Windows Credential Manager errors
	if strings.Contains(msg, "the specified item could not be found") {
		return ErrorClassNotFound
	}

	return ErrorClassSystem
}

// ResolveKeychainValues recursively walks the settings map and resolves keychain:// URIs
func ResolveKeychainValues(settings map[string]interface{}, sources SourceMap, verbose bool) error {
	return resolveRecursive(settings, sources, "", verbose)
}

func resolveRecursive(m map[string]interface{}, sources SourceMap, prefix string, verbose bool) error {
	for k, v := range m {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}

		resolved, err := ResolveValue(v, sources, key, verbose)
		if err != nil {
			return err
		}
		m[k] = resolved
	}
	return nil
}

// IsKeychainNotFound reports whether the error indicates a missing keychain entry.
func IsKeychainNotFound(err error) bool {
	if errors.Is(err, keyring.ErrNotFound) {
		return true
	}
	return ClassifyKeyringError(err) == ErrorClassNotFound
}

// GetKeychainSecret retrieves a secret from the system keychain.
func GetKeychainSecret(service, account string) (string, error) {
	return getKeyringImpl().Get(service, account)
}

// DeleteKeychainSecret removes a secret from the system keychain.
func DeleteKeychainSecret(service, account string) error {
	return getKeyringImpl().Delete(service, account)
}

// ResolveValue resolves a single value, handling keychain:// URIs if present.
func ResolveValue(v interface{}, sources SourceMap, key string, verbose bool) (interface{}, error) {
	switch val := v.(type) {
	case string:
		if strings.HasPrefix(val, KeychainPrefix) {
			uri := strings.TrimPrefix(val, KeychainPrefix)
			parts := strings.SplitN(uri, "/", 2)
			if len(parts) != 2 {
				if verbose {
					fmt.Fprintf(os.Stderr, "Warning: invalid keychain URI for key %q: %q (expected keychain://service/account)\n", key, val)
				}
				return val, nil
			}

			service := parts[0]
			account := parts[1]

			secret, err := getKeyringImpl().Get(service, account)
			if err != nil {
				if verbose {
					fmt.Fprintf(os.Stderr, "Warning: failed to resolve keychain secret for key %q (%s/%s): %v\n", key, service, account, err)
				} else {
					fmt.Fprintf(os.Stderr, "Warning: failed to resolve keychain secret for key %q\n", key)
				}
				return val, nil
			}

			if sources != nil {
				sources[key] = SourceEntry{
					Value:  secret,
					Source: SourceKeychain,
					File:   fmt.Sprintf("%s/%s", service, account),
				}
			}
			return secret, nil
		}
		return val, nil

	case map[string]interface{}:
		if err := resolveRecursive(val, sources, key, verbose); err != nil {
			return nil, err
		}
		return val, nil

	case []interface{}:
		for i, elem := range val {
			// Slices don't have individual source attribution in our SourceMap yet,
			// but we still resolve them. Use index suffix for key tracking if needed.
			elemKey := fmt.Sprintf("%s[%d]", key, i)
			res, err := ResolveValue(elem, sources, elemKey, verbose)
			if err != nil {
				return nil, err
			}
			val[i] = res
		}
		return val, nil

	default:
		return v, nil
	}
}
