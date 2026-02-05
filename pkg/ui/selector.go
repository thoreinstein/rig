package ui

import (
	"errors"

	"thoreinstein.com/rig/pkg/discovery"
)

var (
	// ErrCancelled is returned when the user cancels the selection
	ErrCancelled = errors.New("selection cancelled")
	// ErrNoProjects is returned when there are no projects to select from
	ErrNoProjects = errors.New("no projects found")
)

// SelectProject prompts the user to select a project
func SelectProject(projects []discovery.Project) (*discovery.Project, error) {
	if len(projects) == 0 {
		return nil, ErrNoProjects
	}
	return nil, errors.New("not implemented")
}
