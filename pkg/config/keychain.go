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

		switch val := v.(type) {
		case string:
			if strings.HasPrefix(val, KeychainPrefix) {
				uri := strings.TrimPrefix(val, KeychainPrefix)
				parts := strings.SplitN(uri, "/", 2)
				if len(parts) != 2 {
					if verbose {
						fmt.Fprintf(os.Stderr, "Warning: invalid keychain URI for key %q: %q (expected keychain://service/account)\n", key, val)
					}
					continue
				}

				service := parts[0]
				account := parts[1]

				secret, err := keyring.Get(service, account)
				if err != nil {
					if verbose {
						fmt.Fprintf(os.Stderr, "Warning: failed to resolve keychain secret for key %q (%s/%s): %v\n", key, service, account, err)
					}
					continue
				}

				// Resolve successful
				m[k] = secret
				sources[key] = SourceEntry{
					Value:  secret,
					Source: SourceKeychain,
					File:   fmt.Sprintf("%s/%s", service, account),
				}
			}
		case map[string]interface{}:
			if err := resolveRecursive(val, sources, key, verbose); err != nil {
				return err
			}
		}
	}
	return nil
}
