package vcs

import (
	"reflect"

	"github.com/cockroachdb/errors"
)

// NewProvider creates a new VCS provider based on the provider name.
// If providerName is empty or "git", it returns a LocalProvider.
// This is a convenience wrapper for NewProviderWithManager(nil, ...).
func NewProvider(providerName string, verbose bool) (Provider, error) {
	return NewProviderWithManager(nil, providerName, verbose)
}

// NewProviderWithManager creates a new VCS provider based on the provider name.
// If providerName is "git" or empty, it returns a LocalProvider.
// Otherwise, it returns a PluginProvider using the provided manager.
func NewProviderWithManager(manager PluginManager, providerName string, verbose bool) (Provider, error) {
	// Default to local git provider
	if providerName == "" || providerName == "git" {
		return NewLocalProvider(verbose), nil
	}

	// Guard against typed-nil interface values (e.g., (*plugin.Manager)(nil) stored as PluginManager).
	if manager != nil {
		if v := reflect.ValueOf(manager); v.Kind() == reflect.Ptr && v.IsNil() {
			manager = nil
		}
	}

	// If manager is provided, use it for plugin provider
	if manager != nil {
		return NewPluginProvider(manager, providerName), nil
	}

	// No manager available for a non-default provider — this is a config error
	return nil, errors.Newf("VCS provider %q requires a plugin manager", providerName)
}
