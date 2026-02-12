package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Scanner scans a directory for plugins
type Scanner struct {
	Path    string
	Verbose bool
}

// NewScanner creates a new scanner for the default plugin path
func NewScanner(verbose bool) (*Scanner, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to determine home directory: %w", err)
	}
	path := filepath.Join(homeDir, ".config", "rig", "plugins")
	return &Scanner{
		Path:    path,
		Verbose: verbose,
	}, nil
}

// isExecutable checks if a file is an executable binary
func isExecutable(path string, info os.FileInfo) bool {
	// Check common executable extensions regardless of platform to support tests
	ext := filepath.Ext(path)
	if ext == ".exe" || ext == ".bat" || ext == ".cmd" {
		return true
	}

	// On Unix, also check execute bits
	if os.PathSeparator == '/' {
		return info.Mode()&0111 != 0
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
				manifest, err := loadManifest(manifestPath)
				if err == nil {
					// Found a plugin directory with a manifest
					scanned++
					plugins = append(plugins, Plugin{
						Name:        manifest.Name,
						Version:     manifest.Version,
						Path:        fullPath,
						Status:      StatusCompatible,
						Description: manifest.Description,
						Manifest:    manifest,
						DiscoveryAt: time.Now(),
					})
				}
			}
			continue
		}

		// Skip manifest files themselves
		if filepath.Ext(entry.Name()) == ".yaml" || filepath.Ext(entry.Name()) == ".yml" {
			continue
		}

		// Only check executable files
		if !isExecutable(fullPath, info) {
			continue
		}

		scanned++

		plugin := Plugin{
			Name:        entry.Name(),
			Path:        fullPath,
			DiscoveryAt: time.Now(),
			Status:      StatusCompatible,
		}

		// Look for manifest sidecar: <name>.manifest.yaml or manifest.yaml in the same dir
		manifestPaths := []string{
			fullPath + ".manifest.yaml",
			fullPath + ".manifest.yml",
			filepath.Join(s.Path, "manifest.yaml"), // Only makes sense if there's only one plugin
		}

		var manifest *Manifest
		for _, mp := range manifestPaths {
			if _, err := os.Stat(mp); err == nil {
				manifest, err = loadManifest(mp)
				if err == nil {
					break
				}
			}
		}

		if manifest != nil {
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
