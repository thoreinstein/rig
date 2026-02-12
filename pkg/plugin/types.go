package plugin

import (
	"time"
)

// Status represents the compatibility status of a plugin
type Status string

const (
	StatusCompatible   Status = "Compatible"
	StatusIncompatible Status = "Incompatible"
	StatusError        Status = "Error"
)

// Manifest represents the metadata for a plugin found in manifest.yaml
type Manifest struct {
	Name        string `yaml:"name"`
	Version     string `yaml:"version"`
	Description string `yaml:"description"`
	Author      string `yaml:"author"`
	Requirements struct {
		Rig string `yaml:"rig"` // SemVer requirement for Rig
	} `yaml:"requirements"`
}

// Plugin represents a discovered plugin
type Plugin struct {
	Name        string
	Version     string
	Path        string
	Status      Status
	Description string
	Manifest    *Manifest
	Error       error
	DiscoveryAt time.Time
}

// Result contains the outcome of a discovery scan
type Result struct {
	Plugins  []Plugin
	Scanned  int
	Duration time.Duration
}
