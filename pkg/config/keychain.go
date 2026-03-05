package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/zalando/go-keyring"
)

const KeychainPrefix = "keychain://"

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

			secret, err := keyring.Get(service, account)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to resolve keychain secret for key %q (%s/%s): %v\n", key, service, account, err)
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
