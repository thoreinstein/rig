package plugin

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
)

// ValidateCompatibility checks if the plugin is compatible with the current version of Rig
func ValidateCompatibility(p *Plugin, rigVersion string) error {
	if p.Manifest == nil {
		return nil
	}

	if p.Manifest.Requirements.Rig == "" {
		return nil
	}

	constraint, err := semver.NewConstraint(p.Manifest.Requirements.Rig)
	if err != nil {
		p.Status = StatusError
		p.Error = fmt.Errorf("invalid rig version constraint: %w", err)
		return p.Error
	}

	// Handle 'dev' version by assuming compatibility
	if rigVersion == "dev" || rigVersion == "" {
		return nil
	}

	v, err := semver.NewVersion(rigVersion)
	if err != nil {
		p.Status = StatusError
		p.Error = fmt.Errorf("invalid rig version %q: %w", rigVersion, err)
		return p.Error
	}

	if !constraint.Check(v) {
		p.Status = StatusIncompatible
		p.Error = fmt.Errorf("plugin requires rig %s, but running %s", p.Manifest.Requirements.Rig, rigVersion)
		return p.Error
	}

	return nil
}
