package plugin

import (
	"context"
	"os"
	"time"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
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
	Name         string `yaml:"name"`
	Version      string `yaml:"version"`
	Description  string `yaml:"description"`
	Author       string `yaml:"author"`
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

	// Runtime state
	process    *os.Process
	socketPath string
	client     apiv1.PluginServiceClient
	cancel     context.CancelFunc
}

// Result contains the outcome of a discovery scan
type Result struct {
	Plugins  []Plugin
	Scanned  int
	Duration time.Duration
}
