package plugin

import (
	"context"
	"os"
	"sync"
	"time"

	"google.golang.org/grpc"

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

// Plugin represents a discovered plugin.
//
// Plugin instances are not safe for concurrent use across multiple goroutines
// during Start/Stop operations without external synchronization, although
// internal state is protected by a mutex for basic safety.
type Plugin struct {
	Name         string
	Version      string
	Path         string
	Status       Status
	Description  string
	Manifest     *Manifest
	Error        error
	DiscoveryAt  time.Time
	Capabilities []string

	// Runtime state
	mu         sync.Mutex
	process    *os.Process
	socketPath string
	client     apiv1.PluginServiceClient
	conn       *grpc.ClientConn
	cancel     context.CancelFunc
}

// Result contains the outcome of a discovery scan
type Result struct {
	Plugins  []*Plugin
	Scanned  int
	Duration time.Duration
}
