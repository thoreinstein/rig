package knowledge

import (
	"github.com/cockroachdb/errors"

	"thoreinstein.com/rig/pkg/config"
	"thoreinstein.com/rig/pkg/plugin"
)

// NewProviderWithManager returns a Knowledge Provider based on configuration.
func NewProviderWithManager(cfg *config.Config, manager *plugin.Manager, verbose bool) (Provider, error) {
	providerName := cfg.Notes.Provider
	if providerName == "" || providerName == "local" {
		return NewLocalProvider(cfg, verbose), nil
	}

	if manager == nil {
		return nil, errors.Newf("knowledge provider %q requires a plugin manager", providerName)
	}

	return NewPluginProvider(manager, providerName), nil
}
