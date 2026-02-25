package project

import (
	"fmt"
	"path/filepath"

	"github.com/cockroachdb/errors"
)

// MarkerKind represents the types of markers used for project discovery.
type MarkerKind int

const (
	markerUnknown MarkerKind = iota // zero value; never assigned deliberately
	MarkerGit
	MarkerRigToml
	MarkerBeads
)

func (m MarkerKind) String() string {
	switch m {
	case MarkerGit:
		return ".git"
	case MarkerRigToml:
		return ".rig.toml"
	case MarkerBeads:
		return ".beads"
	default:
		return fmt.Sprintf("Unknown(%d)", m)
	}
}

// ProjectContext holds the discovery result for a project.
type ProjectContext struct {
	// RootPath is the directory where the walk stopped (highest boundary).
	RootPath string
	// Markers maps the marker kind to the absolute path where it was found.
	Markers map[MarkerKind]string
	// Origin is the starting directory for the discovery walk.
	Origin string
}

// HasMarker returns true if the specified marker kind was found during discovery.
func (pc *ProjectContext) HasMarker(kind MarkerKind) bool {
	_, ok := pc.Markers[kind]
	return ok
}

// ConfigFile returns the path to the primary .rig.toml if found.
func (pc *ProjectContext) ConfigFile() string {
	return pc.Markers[MarkerRigToml]
}

// ErrorNoProjectContext is returned when no project markers are found.
type ErrorNoProjectContext struct {
	Reached string
}

func (e *ErrorNoProjectContext) Error() string {
	return fmt.Sprintf("no rig project found (reached %s)", e.value())
}

func (e *ErrorNoProjectContext) value() string {
	if e.Reached == "" || e.Reached == "/" || e.Reached == "." {
		return "filesystem root"
	}
	// Detect Windows drive roots like "C:\"
	if vol := filepath.VolumeName(e.Reached); vol != "" && e.Reached == vol+`\` {
		return "filesystem root"
	}
	return e.Reached
}

// IsNoProjectContext returns true if the error is (or wraps) an ErrorNoProjectContext.
func IsNoProjectContext(err error) bool {
	var target *ErrorNoProjectContext
	return errors.As(err, &target)
}
