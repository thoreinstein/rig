package cmd

import (
	"log/slog"
	"os"

	"github.com/cockroachdb/errors"

	"thoreinstein.com/rig/pkg/config"
	"thoreinstein.com/rig/pkg/discovery"
	"thoreinstein.com/rig/pkg/plugin"
	"thoreinstein.com/rig/pkg/project"
	"thoreinstein.com/rig/pkg/ticket"
	"thoreinstein.com/rig/pkg/ui"
	"thoreinstein.com/rig/pkg/vcs"
)

var projectFlag string

// getVCSProvider returns the VCS provider based on configuration.
func getVCSProvider(cfg *config.Config) (vcs.Provider, func(), error) {
	if cfg.VCS.Provider == "" || cfg.VCS.Provider == "git" {
		return vcs.NewLocalProvider(verbose), func() {}, nil
	}

	// For plugin providers, we need a manager
	var scanner *plugin.Scanner
	var err error

	if ctx, ctxErr := project.CachedDiscover(""); ctxErr == nil && ctx.HasMarker(project.MarkerGit) {
		scanner, err = plugin.NewScannerWithProjectRoot(ctx.Markers[project.MarkerGit])
	} else {
		scanner, err = plugin.NewScanner()
	}

	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to initialize plugin scanner")
	}

	executor := plugin.NewExecutor("")

	// Create manager
	manager, err := plugin.NewManager(executor, scanner, GetVersion(), cfg.PluginConfig, slog.Default())
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to initialize plugin manager")
	}

	cleanup := func() {
		manager.StopAll()
	}

	provider, err := vcs.NewProviderWithManager(manager, cfg.VCS.Provider, verbose)
	if err != nil {
		cleanup()
		return nil, nil, err
	}

	return provider, cleanup, nil
}

// getTicketProvider returns the ticketing provider based on configuration.
func getTicketProvider(cfg *config.Config, projectPath string) (ticket.Provider, func(), error) {
	if cfg.Ticket.Provider == "" || cfg.Ticket.Provider == "local" {
		return ticket.NewLocalProvider(cfg, projectPath, verbose), func() {}, nil
	}

	// For plugin providers, we need a manager
	var scanner *plugin.Scanner
	var err error

	if ctx, ctxErr := project.CachedDiscover(projectPath); ctxErr == nil && ctx.HasMarker(project.MarkerGit) {
		scanner, err = plugin.NewScannerWithProjectRoot(ctx.Markers[project.MarkerGit])
	} else {
		scanner, err = plugin.NewScanner()
	}

	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to initialize plugin scanner")
	}

	executor := plugin.NewExecutor("")

	// Create manager
	manager, err := plugin.NewManager(executor, scanner, GetVersion(), cfg.PluginConfig, slog.Default())
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to initialize plugin manager")
	}

	cleanup := func() {
		manager.StopAll()
	}

	provider, err := ticket.NewProviderWithManager(cfg, manager, projectPath, verbose)
	if err != nil {
		cleanup()
		return nil, nil, err
	}

	return provider, cleanup, nil
}

// resolveProjectContext determines the project root directory
// Returns the absolute path to the project root
func resolveProjectContext(cfg *config.Config, flagValue string, nameOverride string) (string, error) {
	// 1. Check flag (explicit path)
	if flagValue != "" {
		// Validate path exists?
		if _, err := os.Stat(flagValue); err != nil {
			return "", errors.Wrapf(err, "invalid project path: %s", flagValue)
		}
		return flagValue, nil
	}

	// 2. Interactive Selection or Name Lookup
	engine := discovery.NewEngine(&cfg.Discovery, verbose)
	projects, err := engine.GetProjects(false)
	if err != nil {
		return "", errors.Wrap(err, "failed to discover projects")
	}

	// 2a. Check name override
	if nameOverride != "" {
		for _, p := range projects {
			if p.Name == nameOverride {
				return p.Path, nil
			}
		}
		return "", errors.Newf("project %q not found in known projects", nameOverride)
	}

	// 2b. Show Picker
	selected, err := ui.SelectProject(projects)
	if err != nil {
		if errors.Is(err, ui.ErrCancelled) {
			return "", err // Propagate cancellation cleanly
		}
		if errors.Is(err, ui.ErrNoProjects) {
			return "", errors.New("No projects found. Check your configuration or run 'rig config'.")
		}
		return "", err
	}

	return selected.Path, nil
}
