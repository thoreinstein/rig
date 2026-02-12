package plugin

import (
	"github.com/Masterminds/semver/v3"
	"github.com/cockroachdb/errors"
)

// ValidateCompatibility checks if the plugin is compatible with the current version of Rig.
// It mutates p.Status and p.Error directly to reflect the compatibility result.
func ValidateCompatibility(p *Plugin, rigVersion string) {
	if p.Manifest == nil {
		return
	}

	if p.Manifest.Requirements.Rig == "" {
		return
	}

	constraint, err := semver.NewConstraint(p.Manifest.Requirements.Rig)
	if err != nil {
		p.Status = StatusError
		p.Error = errors.Wrap(err, "invalid rig version constraint")
		return
	}

	// Handle 'dev' version by assuming compatibility
	if rigVersion == "dev" || rigVersion == "" {
		return
	}

	v, err := semver.NewVersion(rigVersion)
	if err != nil {
		p.Status = StatusError
		p.Error = errors.Wrapf(err, "invalid rig version %q", rigVersion)
		return
	}

	if !constraint.Check(v) {
		p.Status = StatusIncompatible
		p.Error = errors.Newf("plugin requires rig %s, but running %s", p.Manifest.Requirements.Rig, rigVersion)
		return
	}
}
