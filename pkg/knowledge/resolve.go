package knowledge

import (
	"reflect"

	"github.com/cockroachdb/errors"

	"thoreinstein.com/rig/pkg/config"
)

// NewProviderWithManager returns a Knowledge Provider based on configuration.
func NewProviderWithManager(cfg *config.Config, manager PluginManager, verbose bool) (Provider, error) {
	providerName := cfg.Notes.Provider
	if providerName == "" || providerName == "local" {
		return NewLocalProvider(cfg, verbose), nil
	}

	// Guard against typed-nil interface values (e.g., (*plugin.Manager)(nil) stored as PluginManager).
	if manager != nil {
		if v := reflect.ValueOf(manager); v.Kind() == reflect.Ptr && v.IsNil() {
			manager = nil
		}
	}

	if manager == nil {
		return nil, errors.Newf("knowledge provider %q requires a plugin manager", providerName)
	}

	return NewPluginProvider(manager, providerName), nil
}
