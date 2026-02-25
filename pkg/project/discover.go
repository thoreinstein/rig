package project

import (
	"os"
	"path/filepath"

	"github.com/cockroachdb/errors"
)

// Discover resolves the physical path of startDir and traverses upward looking for markers.
// It stops at the first .git directory/file found or when the filesystem root is reached.
func Discover(startDir string) (*ProjectContext, error) {
	if startDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, errors.Wrap(err, "failed to get current working directory")
		}
		startDir = cwd
	}

	// Resolve physical path (handle symlinks)
	origin, err := filepath.EvalSymlinks(startDir)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to resolve physical path for %q", startDir)
	}
	origin = filepath.Clean(origin)

	ctx := &ProjectContext{
		Origin:  origin,
		Markers: make(map[MarkerKind]string),
	}

	current := origin
	for {
		found, stop, err := checkMarkers(current, ctx.Markers)
		if err != nil {
			return nil, err
		}

		if found && ctx.RootPath == "" {
			ctx.RootPath = current
		}

		if stop {
			ctx.RootPath = current
			return ctx, nil
		}

		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}

	if len(ctx.Markers) == 0 {
		return nil, &ErrorNoProjectContext{Reached: current}
	}

	return ctx, nil
}

// checkMarkers checks for markers in the given directory and updates the markers map.
// returns (foundAny, stopWalk, error)
func checkMarkers(dir string, markers map[MarkerKind]string) (bool, bool, error) {
	foundAny := false
	stopWalk := false

	// 1. Check for .rig.toml (file)
	rigPath := filepath.Join(dir, ".rig.toml")
	if info, err := os.Stat(rigPath); err == nil {
		if !info.IsDir() {
			if _, exists := markers[MarkerRigToml]; !exists {
				markers[MarkerRigToml] = rigPath
				foundAny = true
			}
		}
	} else if !os.IsNotExist(err) {
		return false, false, errors.Wrapf(err, "failed to stat %q", rigPath)
	}

	// 2. Check for .beads (dir containing issues.jsonl)
	beadsPath := filepath.Join(dir, ".beads", "issues.jsonl")
	if info, err := os.Stat(beadsPath); err == nil {
		if !info.IsDir() {
			if _, exists := markers[MarkerBeads]; !exists {
				markers[MarkerBeads] = filepath.Dir(beadsPath)
				foundAny = true
			}
		}
	} else if !os.IsNotExist(err) {
		return false, false, errors.Wrapf(err, "failed to stat %q", beadsPath)
	}

	// 3. Check for .git (dir or file for worktrees)
	// This is the project boundary - we stop walking if found.
	// Store the directory containing .git, not the .git path itself.
	gitPath := filepath.Join(dir, ".git")
	if _, err := os.Stat(gitPath); err == nil {
		if _, exists := markers[MarkerGit]; !exists {
			markers[MarkerGit] = dir
		}
		foundAny = true
		stopWalk = true
	} else if !os.IsNotExist(err) {
		return false, false, errors.Wrapf(err, "failed to stat %q", gitPath)
	}

	return foundAny, stopWalk, nil
}
