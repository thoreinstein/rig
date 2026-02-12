package plugin

import (
	"os"

	"gopkg.in/yaml.v3"
)

// loadManifest reads and parses a manifest.yaml file
func loadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, err
	}

	return &m, nil
}
