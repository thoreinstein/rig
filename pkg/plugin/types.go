package plugin

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"google.golang.org/grpc"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
)

// contextKey is a private type for context value keys used across the plugin package.
type contextKey string

const (
	// pluginNameKey is the context key for the plugin name injected by the interceptor.
	pluginNameKey contextKey = "pluginName"
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

	// CommandCapability is the name of the capability for plugins that provide CLI commands.
	CommandCapability = "command"

	// NodeCapability is the name of the capability for plugins that can execute workflow nodes.
	NodeCapability = "node"

	// VCSCapability is the name of the capability for Version Control System plugins.
	VCSCapability = "vcs"

	// TicketCapability is the name of the capability for ticketing integration plugins.
	TicketCapability = "ticket"

	// KnowledgeCapability is the name of the capability for knowledge management plugins.
	KnowledgeCapability = "knowledge"

	// APIVersion is the current version of the Rig Plugin API contract.
	APIVersion = "v1"
)

// CommandDescriptor represents a CLI command provided by a plugin.
type CommandDescriptor struct {
	Name    string   `yaml:"name"`
	Short   string   `yaml:"short"`
	Long    string   `yaml:"long"`
	Aliases []string `yaml:"aliases"`
}

// Manifest represents the metadata for a plugin found in manifest.yaml
type Manifest struct {
	Name         string `yaml:"name"`
	Version      string `yaml:"version"`
	Description  string `yaml:"description"`
	Author       string `yaml:"author"`
	Requirements struct {
		Rig string `yaml:"rig"` // SemVer requirement for Rig
	} `yaml:"requirements"`
	Commands []CommandDescriptor `yaml:"commands"`
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
	lastUsed     time.Time
	Capabilities []*apiv1.Capability
	EnvAllowList []string // Per-plugin environment variable allow-list

	// Runtime state
	mu              sync.Mutex
	activeSessions  int
	process         *os.Process
	socketDir       string
	socketPath      string
	hostPath        string
	hostListener    net.Listener
	hostServer      *grpc.Server
	client          apiv1.PluginServiceClient
	AssistantClient apiv1.AssistantServiceClient
	CommandClient   apiv1.CommandServiceClient
	NodeClient      apiv1.NodeExecutionServiceClient
	VCSClient       apiv1.VCSServiceClient
	TicketClient    apiv1.TicketServiceClient
	KnowledgeClient apiv1.KnowledgeServiceClient
	conn            *grpc.ClientConn
	cancel          context.CancelFunc
}

// LastUsedTime returns the time the plugin was last used.
func (p *Plugin) LastUsedTime() time.Time {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.lastUsed
}

// SetLastUsed updates the time the plugin was last used.
func (p *Plugin) SetLastUsed(t time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lastUsed = t
}

// cleanupHost tears down the per-plugin host server, listener, and socket
// directory. It snapshots the host fields under p.mu to avoid racing with
// concurrent getOrStartPlugin calls.
func (p *Plugin) cleanupHost() {
	p.mu.Lock()
	srv := p.hostServer
	lis := p.hostListener
	path := p.hostPath
	p.hostServer = nil
	p.hostListener = nil
	p.hostPath = ""
	p.mu.Unlock()

	if srv != nil {
		srv.GracefulStop()
	}
	if lis != nil {
		_ = lis.Close()
	}
	if path != "" {
		_ = os.RemoveAll(filepath.Dir(path))
	}
}

// AcquireSession increments the active session count.
func (p *Plugin) AcquireSession() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.activeSessions++
	p.lastUsed = time.Now()
}

// ReleaseSession decrements the active session count and updates lastUsed.
func (p *Plugin) ReleaseSession() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.activeSessions > 0 {
		p.activeSessions--
	}
	p.lastUsed = time.Now()
}

// IsBusy returns true if the plugin has active sessions.
func (p *Plugin) IsBusy() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.activeSessions > 0
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

var ErrPluginNotFound = errors.New("plugin not found")
