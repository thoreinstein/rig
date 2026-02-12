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
func NewScanner(verbose bool) *Scanner {
	homeDir, _ := os.UserHomeDir()
	path := filepath.Join(homeDir, ".config", "rig", "plugins")
	return &Scanner{
		Path:    path,
		Verbose: verbose,
	}
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
		if entry.IsDir() {
			continue
		}
		
		// Skip manifest files themselves
		if filepath.Ext(entry.Name()) == ".yaml" || filepath.Ext(entry.Name()) == ".yml" {
			continue
		}

		scanned++

		fullPath := filepath.Join(s.Path, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}

		// Only check executable files
		if info.Mode()&0111 == 0 {
			continue
		}

		plugin := Plugin{
			Name:        entry.Name(),
			Path:        fullPath,
			DiscoveryAt: time.Now(),
			Status:      StatusCompatible, // Default, will be validated later
		}

		// Look for manifest sidecar
		// Supporting both plugin.manifest.yaml and manifest.yaml in a subdirectory (though we only scan one level)
		// The requirement says "metadata read from manifest.yaml sidecar"
		// Usually sidecar means plugin-name.manifest.yaml or similar if they are in the same dir.
		// Let's check for both <name>.manifest.yaml and manifest.yaml if it's a directory (but we skipped dirs).
		// If they are all in one dir, it must be <name>.manifest.yaml.
		manifestPath := fullPath + ".manifest.yaml"
		if _, err := os.Stat(manifestPath); err != nil {
			// Try .manifest.yml
			manifestPath = fullPath + ".manifest.yml"
		}

		if _, err := os.Stat(manifestPath); err == nil {
			manifest, err := loadManifest(manifestPath)
			if err != nil {
				plugin.Status = StatusError
				plugin.Error = fmt.Errorf("failed to load manifest: %w", err)
			} else {
				plugin.Manifest = manifest
				if manifest.Name != "" {
					plugin.Name = manifest.Name
				}
				plugin.Version = manifest.Version
				plugin.Description = manifest.Description
			}
		}

		plugins = append(plugins, plugin)
	}

	return &Result{
		Plugins:  plugins,
		Scanned:  scanned,
		Duration: time.Since(start),
	}, nil
}
