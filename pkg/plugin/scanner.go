package plugin

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
)

// Scanner scans one or more directories for plugins.
// When multiple paths are configured, later paths take precedence for
// name conflicts (e.g. project-level overrides system-level).
type Scanner struct {
	Paths []string
}

// NewScanner creates a new scanner for the default system-level plugin path
// (~/.config/rig/plugins).
func NewScanner() (*Scanner, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, errors.Wrap(err, "failed to determine home directory")
	}
	path := filepath.Join(homeDir, ".config", "rig", "plugins")
	return &Scanner{
		Paths: []string{path},
	}, nil
}

// NewScannerWithProjectRoot creates a scanner that searches both the system-level
// plugin path and a project-level path (<projectRoot>/.rig/plugins).
// Project-level plugins override system-level plugins with the same name.
func NewScannerWithProjectRoot(projectRoot string) (*Scanner, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, errors.Wrap(err, "failed to determine home directory")
	}
	systemPath := filepath.Join(homeDir, ".config", "rig", "plugins")
	projectPath := filepath.Join(projectRoot, ".rig", "plugins")
	return &Scanner{
		Paths: []string{systemPath, projectPath},
	}, nil
}

// findExecutable returns the path of the first executable file in a directory.
// Entries are sorted alphabetically to ensure consistent behavior across different
// filesystems and platforms.
func findExecutable(dir string) (string, bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}

	// Sort entries to ensure deterministic selection of the primary executable
	slices.SortFunc(entries, func(a, b os.DirEntry) int {
		return strings.Compare(a.Name(), b.Name())
	})

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		// Strictly Unix execute bits
		if info.Mode()&0111 != 0 {
			return filepath.Join(dir, entry.Name()), true
		}
	}
	return "", false
}

// sourceLabel returns "project" if the path is the last entry in Paths
// (the project-level path), otherwise "system".
func (s *Scanner) sourceLabel(idx int) string {
	if len(s.Paths) > 1 && idx == len(s.Paths)-1 {
		return "project"
	}
	return "system"
}

// Scan finds plugins across all configured paths. Later paths take
// precedence: if two paths contain a plugin with the same name, the
// one discovered last wins.
func (s *Scanner) Scan() (*Result, error) {
	start := time.Now()
	scanned := 0

	// Use a map for dedup — last writer wins.
	seen := make(map[string]*Plugin)
	// Preserve insertion order for stable output.
	var order []string

	for i, dir := range s.Paths {
		plugins, n, err := scanDir(dir, s.sourceLabel(i))
		if err != nil {
			return nil, err
		}
		scanned += n
		for _, p := range plugins {
			if _, exists := seen[p.Name]; !exists {
				order = append(order, p.Name)
			}
			seen[p.Name] = p
		}
	}

	merged := make([]*Plugin, 0, len(order))
	for _, name := range order {
		merged = append(merged, seen[name])
	}

	return &Result{
		Plugins:  merged,
		Scanned:  scanned,
		Duration: time.Since(start),
	}, nil
}

// scanDir scans a single directory for plugins. Returns the discovered
// plugins, the count of items scanned, and any error. Missing directories
// are silently skipped.
func scanDir(dir, source string) ([]*Plugin, int, error) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, 0, nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, 0, err
	}

	scanned := 0
	plugins := make([]*Plugin, 0, len(entries))

	for _, entry := range entries {
		fullPath := filepath.Join(dir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}

		// Handle subdirectories (each plugin in its own folder)
		if entry.IsDir() {
			// Verify the directory contains at least one executable
			execPath, found := findExecutable(fullPath)
			if !found {
				continue
			}

			scanned++
			plugin := &Plugin{
				Name:        entry.Name(),
				Path:        execPath,
				Source:      source,
				Status:      StatusCompatible,
				DiscoveryAt: time.Now(),
			}

			// Check for optional manifest.yaml inside the directory
			manifestPath := filepath.Join(fullPath, "manifest.yaml")
			if _, err := os.Stat(manifestPath); err == nil {
				manifest, err := loadManifest(manifestPath)
				if err != nil {
					plugin.Status = StatusError
					plugin.Error = errors.Wrap(err, "failed to load manifest")
				} else {
					if manifest.Name != "" {
						plugin.Name = manifest.Name
					}
					plugin.Version = manifest.Version
					plugin.Description = manifest.Description
					plugin.Manifest = manifest
				}
			}

			plugins = append(plugins, plugin)
			continue
		}

		// Skip manifest files themselves
		if strings.HasSuffix(strings.ToLower(entry.Name()), ".yaml") || strings.HasSuffix(strings.ToLower(entry.Name()), ".yml") {
			continue
		}

		// Only check executable files (strictly Unix execute bits)
		if info.Mode()&0111 == 0 {
			continue
		}

		scanned++

		plugin := &Plugin{
			Name:        entry.Name(),
			Path:        fullPath,
			Source:      source,
			DiscoveryAt: time.Now(),
			Status:      StatusCompatible,
		}

		// Look for manifest sidecar: <name>.manifest.yaml alongside the executable
		// Strip common extensions (like .sh, .py) to find the logical manifest name
		baseName := entry.Name()
		if ext := filepath.Ext(baseName); ext != "" {
			baseName = strings.TrimSuffix(baseName, ext)
		}

		manifestBase := filepath.Join(dir, baseName)
		manifestPaths := []string{
			manifestBase + ".manifest.yaml",
			manifestBase + ".manifest.yml",
		}

		var parseErr error
		var manifest *Manifest
		for _, mp := range manifestPaths {
			if _, err := os.Stat(mp); err == nil {
				manifest, parseErr = loadManifest(mp)
				if parseErr == nil {
					break
				}
				// If we found a manifest file but it failed to parse, stop and report it
				break
			}
		}

		if parseErr != nil {
			plugin.Status = StatusError
			plugin.Error = errors.Wrap(parseErr, "failed to load manifest")
		} else if manifest != nil {
			plugin.Manifest = manifest
			if manifest.Name != "" {
				plugin.Name = manifest.Name
			}
			plugin.Version = manifest.Version
			plugin.Description = manifest.Description
		}

		plugins = append(plugins, plugin)
	}

	return plugins, scanned, nil
}
