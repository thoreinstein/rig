package ui

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"thoreinstein.com/rig/pkg/discovery"
)

var (
	// ErrCancelled is returned when the user cancels the selection
	ErrCancelled = errors.New("selection cancelled")
	// ErrNoProjects is returned when there are no projects to select from
	ErrNoProjects = errors.New("no projects found")
)

// SelectProject prompts the user to select a project using fzf
func SelectProject(projects []discovery.Project) (*discovery.Project, error) {
	if len(projects) == 0 {
		return nil, ErrNoProjects
	}

	// Check if fzf is installed
	fzfPath, err := exec.LookPath("fzf")
	if err != nil {
		return nil, fmt.Errorf("fzf not found in PATH: %w", err)
	}

	// Prepare input
	var input bytes.Buffer
	for _, p := range projects {
		// Format: Name <tab> Path
		// We use tab as delimiter so fzf can potentially handle fields if needed,
		// and it provides a nice visual separation.
		input.WriteString(fmt.Sprintf("%s\t%s\n", p.Name, p.Path))
	}

	// Run fzf
	// --height=40%: Match typical fzf behavior
	// --layout=reverse: Top-down list
	// --delimiter=\t: Use tab as delimiter
	// --with-nth=1,2: Display and search both name and path
	// #nosec G204 - fzf binary is looked up in PATH, no user-controlled arguments are passed directly
	cmd := exec.Command(fzfPath,
		"--height=40%",
		"--layout=reverse",
		"--delimiter=\t",
		"--with-nth=1,2",
		"--cycle", // Enable cycling
	)
	cmd.Stdin = &input
	cmd.Stderr = os.Stderr // fzf uses stderr for UI rendering
	var output bytes.Buffer
	cmd.Stdout = &output

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			// fzf returns 130 on cancellation (ESC, Ctrl-C, Ctrl-G)
			if exitErr.ExitCode() == 130 {
				return nil, ErrCancelled
			}
		}
		return nil, fmt.Errorf("fzf failed: %w", err)
	}

	// Parse output
	selectedLine := strings.TrimSpace(output.String())
	if selectedLine == "" {
		return nil, ErrCancelled
	}

	parts := strings.Split(selectedLine, "\t")
	if len(parts) < 2 {
		// Fallback: search by path suffix if tab splitting fails?
		// Or maybe the user didn't select anything?
		return nil, fmt.Errorf("invalid selection output: %q", selectedLine)
	}

	selectedPath := parts[1]

	// Find the project object that matches the selected path
	for _, p := range projects {
		if p.Path == selectedPath {
			return &p, nil
		}
	}

	return nil, fmt.Errorf("selected project path %q not found in original list", selectedPath)
}
