package cmd

import (
	"os"

	"github.com/cockroachdb/errors"

	"thoreinstein.com/rig/pkg/config"
	"thoreinstein.com/rig/pkg/discovery"
	"thoreinstein.com/rig/pkg/ui"
)

var projectFlag string

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
