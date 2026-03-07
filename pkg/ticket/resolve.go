package ticket

import (
	"github.com/cockroachdb/errors"

	"thoreinstein.com/rig/pkg/config"
	"thoreinstein.com/rig/pkg/internal"
)

// NewProviderWithManager creates a ticket provider based on the configuration.
func NewProviderWithManager(cfg *config.Config, manager PluginManager, projectPath string, verbose bool) (Provider, error) {
	providerName := cfg.Ticket.Provider
	if providerName == "" || providerName == "local" {
		return NewLocalProvider(cfg, projectPath, verbose), nil
	}

	if internal.IsNilInterface(manager) {
		return nil, errors.Newf("ticket provider %q requires a plugin manager", providerName)
	}

	return NewPluginProvider(manager, providerName), nil
}
