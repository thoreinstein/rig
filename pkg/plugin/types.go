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

const (
	// AssistantCapability is the name of the capability for AI completion plugins.
	AssistantCapability = "assistant"

	// APIVersion is the current version of the Rig Plugin API contract.
	APIVersion = "v1"
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
	APIVersion   string `json:"api_version"`
	Path         string
	Args         []string
	Source       string // Origin of the plugin: "system" or "project"
	Status       Status
	Description  string
	Manifest     *Manifest
	Error        error
	DiscoveryAt  time.Time
	Capabilities []*apiv1.Capability

	// Runtime state
	mu              sync.Mutex
	process         *os.Process
	socketPath      string
	client          apiv1.PluginServiceClient
	AssistantClient apiv1.AssistantServiceClient
	conn            *grpc.ClientConn
	cancel          context.CancelFunc
}

// Result contains the outcome of a discovery scan, including found plugins and metadata.
type Result struct {
	// Plugins is the list of discovered plugins.
	Plugins []*Plugin
	// Scanned is the total number of items scanned.
	Scanned int
	// Duration is the time taken to complete the scan.
	Duration time.Duration
}
