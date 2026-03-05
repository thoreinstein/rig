package ticket

import (
	"thoreinstein.com/rig/pkg/config"
	"thoreinstein.com/rig/pkg/plugin"
)

// NewProviderWithManager creates a ticket provider based on the configuration.
func NewProviderWithManager(cfg *config.Config, manager *plugin.Manager, projectPath string, verbose bool) (Provider, error) {
	providerName := cfg.Ticket.Provider
	if providerName == "" || providerName == "local" {
		return NewLocalProvider(cfg, projectPath, verbose), nil
	}

	if manager == nil {
		// Fallback to local if no manager provided
		return NewLocalProvider(cfg, projectPath, verbose), nil
	}

	return NewPluginProvider(manager, providerName), nil
}
