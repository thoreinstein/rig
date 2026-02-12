package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Scanner scans a directory for plugins
type Scanner struct {
	Path string
}

// NewScanner creates a new scanner for the default plugin path
func NewScanner() (*Scanner, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to determine home directory: %w", err)
	}
	path := filepath.Join(homeDir, ".config", "rig", "plugins")
	return &Scanner{
		Path: path,
	}, nil
}

// hasExecutable checks if a directory contains at least one executable file
func hasExecutable(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}

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
			return true
		}
	}
	return false
}

// Scan finds plugins in the scanner's path
func (s *Scanner) Scan() (*Result, error) {
	start := time.Now()
	var plugins []Plugin
	scanned := 0

	// Ensure the directory exists
	if _, err := os.Stat(s.Path); os.IsNotExist(err) {
		return &Result{Duration: time.Since(start)}, nil
	}

	entries, err := os.ReadDir(s.Path)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		fullPath := filepath.Join(s.Path, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}

		// Handle subdirectories (each plugin in its own folder)
		if entry.IsDir() {
			// Check for manifest.yaml inside the directory
			manifestPath := filepath.Join(fullPath, "manifest.yaml")
			if _, err := os.Stat(manifestPath); err == nil {
				// Verify the directory contains at least one executable
				if !hasExecutable(fullPath) {
					continue
				}

				scanned++
				manifest, err := loadManifest(manifestPath)
				plugin := Plugin{
					Name:        entry.Name(),
					Path:        fullPath,
					Status:      StatusCompatible,
					DiscoveryAt: time.Now(),
				}

				if err != nil {
					plugin.Status = StatusError
					plugin.Error = fmt.Errorf("failed to load manifest: %w", err)
				} else {
					if manifest.Name != "" {
						plugin.Name = manifest.Name
					}
					plugin.Version = manifest.Version
					plugin.Description = manifest.Description
					plugin.Manifest = manifest
				}
				plugins = append(plugins, plugin)
			}
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

		plugin := Plugin{
			Name:        entry.Name(),
			Path:        fullPath,
			DiscoveryAt: time.Now(),
			Status:      StatusCompatible,
		}

		// Look for manifest sidecar: <name>.manifest.yaml alongside the executable
		// Strip common extensions (like .sh, .py) to find the logical manifest name
		baseName := entry.Name()
		if ext := filepath.Ext(baseName); ext != "" {
			baseName = strings.TrimSuffix(baseName, ext)
		}

		manifestBase := filepath.Join(s.Path, baseName)
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
			plugin.Error = fmt.Errorf("failed to load manifest: %w", parseErr)
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

	return &Result{
		Plugins:  plugins,
		Scanned:  scanned,
		Duration: time.Since(start),
	}, nil
}
