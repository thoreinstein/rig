package ticket

import (
	"reflect"

	"github.com/cockroachdb/errors"

	"thoreinstein.com/rig/pkg/config"
)

// NewProviderWithManager creates a ticket provider based on the configuration.
func NewProviderWithManager(cfg *config.Config, manager PluginManager, projectPath string, verbose bool) (Provider, error) {
	providerName := cfg.Ticket.Provider
	if providerName == "" || providerName == "local" {
		return NewLocalProvider(cfg, projectPath, verbose), nil
	}

	// Guard against typed-nil interface values (e.g., (*plugin.Manager)(nil) stored as PluginManager).
	if manager != nil {
		if v := reflect.ValueOf(manager); v.Kind() == reflect.Ptr && v.IsNil() {
			manager = nil
		}
	}

	if manager == nil {
		return nil, errors.Newf("ticket provider %q requires a plugin manager", providerName)
	}

	return NewPluginProvider(manager, providerName), nil
}
