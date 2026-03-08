package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/zalando/go-keyring"
	"golang.org/x/sync/singleflight"
	"google.golang.org/grpc"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
	"thoreinstein.com/rig/pkg/config"
	"thoreinstein.com/rig/pkg/errors"
	"thoreinstein.com/rig/pkg/ui"
)

// pluginExecutor defines the interface for starting and stopping plugin processes.
// Implementations MUST be mutable types (e.g., pointers) if they maintain state
// across method calls, such as the host endpoint path.
type pluginExecutor interface {
	Start(ctx context.Context, p *Plugin) error
	Stop(p *Plugin) error
	PrepareClient(p *Plugin) error
	Handshake(ctx context.Context, p *Plugin, rigVersion, apiVersion string, configJSON []byte) error
	SetHostEndpoint(path string)
	SetGlobalEnvAllowList(list []string)
}

// ConfigProvider is a function that returns the JSON-serialized configuration for a plugin.
// A nil ConfigProvider is safe: plugins receive an empty JSON object ("{}") as config.
// Secret resolution errors are surfaced through the host's secret-resolution mechanism
// (for example as generic failures or omitted keys), and no specific error string is guaranteed.
type ConfigProvider func(pluginName string) ([]byte, error)

// Manager manages a pool of active plugins.
// ManagerOption defines a functional option for configuring a Manager.
type ManagerOption func(*Manager)

// WithUIServer sets a custom UI server for the manager to use for plugin callbacks.
func WithUIServer(srv apiv1.UIServiceServer) ManagerOption {
	return func(m *Manager) {
		m.hostUI = srv
	}
}

// WithPluginContext sets the environment context provided to plugins.
// The actual HostContextProxy is built once in NewManager with the canonical tokenStore.
func WithPluginContext(ctx PluginContext) ManagerOption {
	return func(m *Manager) {
		m.pluginCtx = &ctx
	}
}

// WithGlobalEnvAllowList sets the global environment allow-list.
func WithGlobalEnvAllowList(list []string) ManagerOption {
	return func(m *Manager) {
		m.globalEnvAllowList = list
	}
}

// Manager manages a pool of active plugins.
type Manager struct {
	executor       pluginExecutor
	scanner        *Scanner
	rigVersion     string
	configProvider ConfigProvider
	logger         *slog.Logger

	// Host-side Proxy Services
	hostServer         *grpc.Server
	hostUI             apiv1.UIServiceServer
	tokenStore         *tokenStore
	secretProxy        *HostSecretProxy
	contextProxy       *HostContextProxy
	pluginCtx          *PluginContext // set by WithPluginContext, consumed once by NewManager
	hostL              net.Listener
	hostPath           string
	hostDir            string
	globalEnvAllowList []string

	secretCache   sync.Map           // map[pluginName]map[string]any — shared with resolver
	secretCacheSF singleflight.Group // deduplicates concurrent cache-miss resolutions per plugin

	mu      sync.Mutex
	plugins map[string]*Plugin
}

// NewManager creates a new plugin manager and starts the host-side UI Proxy Service.
func NewManager(executor pluginExecutor, scanner *Scanner, rigVersion string, configProvider ConfigProvider, logger *slog.Logger, opts ...ManagerOption) (*Manager, error) {
	// 1. Create a private directory and generate unique UDS path for the host server
	hostDir, err := os.MkdirTemp("", "rig-h-")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create temporary directory for host socket")
	}
	if err := os.Chmod(hostDir, 0o700); err != nil {
		_ = os.RemoveAll(hostDir)
		return nil, errors.Wrap(err, "failed to set permissions on temporary directory for host socket")
	}

	u, err := uuid.NewRandom()
	if err != nil {
		_ = os.RemoveAll(hostDir)
		return nil, errors.Wrap(err, "failed to generate unique identifier for host socket")
	}
	hostPath := filepath.Join(hostDir, fmt.Sprintf("rig-h-%s.sock", u.String()[:8]))

	// 2. Start host gRPC server
	lis, err := net.Listen("unix", hostPath)
	if err != nil {
		_ = os.RemoveAll(hostDir)
		return nil, errors.Wrapf(err, "failed to listen on host socket %q", hostPath)
	}
	// Restrict socket permissions to the current user only.
	if err := os.Chmod(hostPath, 0o600); err != nil {
		lis.Close()
		_ = os.RemoveAll(hostDir)
		return nil, errors.Wrapf(err, "failed to set permissions on host socket %q", hostPath)
	}

	m := &Manager{
		executor:       executor,
		scanner:        scanner,
		rigVersion:     rigVersion,
		configProvider: configProvider,
		logger:         logger,
		hostL:          lis,
		hostPath:       hostPath,
		hostDir:        hostDir,
		plugins:        make(map[string]*Plugin),
	}

	for _, opt := range opts {
		opt(m)
	}

	if m.hostUI == nil {
		m.hostUI = ui.NewUIServer()
	}

	m.executor.SetGlobalEnvAllowList(m.globalEnvAllowList)

	// Host secret proxy uses the config provider to resolve secrets.
	// The resolver receives pluginName and secretKey as separate, validated
	// parameters — no dot-delimited key parsing required.
	//
	// The secretCache (on Manager) caches the parsed secrets map per plugin
	// to avoid calling configProvider and json.Unmarshal on every GetSecret
	// request. Plugin configs are immutable at runtime, so the cache never
	// needs invalidation. The cache is also seeded by getOrStartPlugin to
	// avoid a redundant parse on first GetSecret.
	resolver := func(pluginName, secretKey string) (string, error) {
		// 1. Look up or populate the per-plugin secrets map.
		// singleflight deduplicates concurrent cache-miss resolutions for
		// the same plugin so only one goroutine parses the config.
		var secrets map[string]any
		if cached, ok := m.secretCache.Load(pluginName); ok {
			secrets = cached.(map[string]any)
		} else {
			val, err, _ := m.secretCacheSF.Do(pluginName, func() (any, error) {
				// Double-check after winning the flight.
				if cached, ok := m.secretCache.Load(pluginName); ok {
					return cached, nil
				}

				if m.configProvider == nil {
					return nil, errors.New("no config provider available")
				}

				data, err := m.configProvider(pluginName)
				if err != nil {
					return nil, err
				}

				var cfg map[string]any
				if err := json.Unmarshal(data, &cfg); err != nil {
					return nil, errors.Wrap(err, "failed to unmarshal plugin config")
				}

				sec, ok := cfg["secrets"].(map[string]any)
				if !ok {
					return nil, errors.Wrap(ErrSecretNotFound, fmt.Sprintf("no 'secrets' section found for plugin %q", pluginName))
				}

				m.secretCache.Store(pluginName, sec)
				return sec, nil
			})
			if err != nil {
				return "", err
			}
			secrets = val.(map[string]any)
		}

		// 2. Look up the requested key.
		val, ok := secrets[secretKey]
		if !ok {
			return "", errors.Wrap(ErrSecretNotFound, fmt.Sprintf("secret %q not found for plugin %q", secretKey, pluginName))
		}

		// 3. Resolve keychain:// URI if present, otherwise return as-is.
		strVal, ok := val.(string)
		if !ok {
			return "", errors.Newf("secret %q is not a string for plugin %q", secretKey, pluginName)
		}

		if uri, ok := strings.CutPrefix(strVal, config.KeychainPrefix); ok {
			parts := strings.SplitN(uri, "/", 2)
			if len(parts) != 2 {
				return "", errors.Newf("invalid keychain URI for secret %q: expected keychain://service/account", secretKey)
			}
			secret, err := keyring.Get(parts[0], parts[1])
			if err != nil {
				return "", errors.Wrapf(err, "failed to resolve keychain secret %q (%s/%s)", secretKey, parts[0], parts[1])
			}
			return secret, nil
		}

		return strVal, nil
	}

	m.tokenStore = newTokenStore()
	m.secretProxy = NewHostSecretProxy(m.tokenStore, resolver)

	// Build the context proxy once with the canonical tokenStore.
	pCtx := PluginContext{}
	if m.pluginCtx != nil {
		pCtx = *m.pluginCtx
		m.pluginCtx = nil // consumed; prevent stale references
	}
	m.contextProxy = NewHostContextProxy(m.tokenStore, pCtx, m.logger)

	srv := grpc.NewServer()
	apiv1.RegisterUIServiceServer(srv, m.hostUI)
	apiv1.RegisterSecretServiceServer(srv, m.secretProxy)
	apiv1.RegisterContextServiceServer(srv, m.contextProxy)
	m.hostServer = srv

	go func() {
		if err := srv.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			if m.logger != nil {
				m.logger.Error("host gRPC server failed", "error", err)
			} else {
				fmt.Fprintf(os.Stderr, "host gRPC server failed: %v\n", err)
			}
		}
	}()

	// 3. Configure executor with host endpoint.
	// This works despite the interface being passed by value because the
	// implementation is expected to be a mutable pointer type.
	executor.SetHostEndpoint(hostPath)

	return m, nil
}

// GetAssistantClient returns a gRPC client for the specified assistant plugin.
// If the plugin is not running, it will be started.
func (m *Manager) GetAssistantClient(ctx context.Context, name string) (client apiv1.AssistantServiceClient, err error) {
	p, err := m.getOrStartPlugin(ctx, name)
	if err != nil {
		return nil, err
	}

	p.AcquireSession()
	defer func() {
		if err != nil {
			p.ReleaseSession()
		}
	}()

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.process == nil {
		return nil, errors.NewPluginError(name, "GetAssistantClient", "plugin process is no longer running")
	}

	// Verify the plugin has the assistant capability
	hasAssistant := false
	for _, cap := range p.Capabilities {
		if cap.Name == AssistantCapability {
			hasAssistant = true
			break
		}
	}

	if !hasAssistant {
		return nil, errors.NewPluginError(name, "GetAssistantClient", "plugin does not support assistant capability")
	}

	if p.AssistantClient == nil {
		if p.conn == nil {
			return nil, errors.NewPluginError(name, "GetAssistantClient", "plugin connection not established")
		}
		p.AssistantClient = apiv1.NewAssistantServiceClient(p.conn)
	}

	return p.AssistantClient, nil
}

// GetCommandClient returns a gRPC client for the specified command plugin.
// If the plugin is not running, it will be started.
func (m *Manager) GetCommandClient(ctx context.Context, name string) (client apiv1.CommandServiceClient, err error) {
	p, err := m.getOrStartPlugin(ctx, name)
	if err != nil {
		return nil, err
	}

	p.AcquireSession()
	defer func() {
		if err != nil {
			p.ReleaseSession()
		}
	}()

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.process == nil {
		return nil, errors.NewPluginError(name, "GetCommandClient", "plugin process is no longer running")
	}

	// Verify the plugin has the command capability
	hasCommand := false
	for _, cap := range p.Capabilities {
		if cap.Name == CommandCapability {
			hasCommand = true
			break
		}
	}

	if !hasCommand {
		return nil, errors.NewPluginError(name, "GetCommandClient", "plugin does not support command capability")
	}

	if p.CommandClient == nil {
		if p.conn == nil {
			return nil, errors.NewPluginError(name, "GetCommandClient", "plugin connection not established")
		}
		p.CommandClient = apiv1.NewCommandServiceClient(p.conn)
	}

	return p.CommandClient, nil
}

// GetNodeClient returns a gRPC client for the specified node execution plugin.
// If the plugin is not running, it will be started.
func (m *Manager) GetNodeClient(ctx context.Context, name string) (client apiv1.NodeExecutionServiceClient, err error) {
	p, err := m.getOrStartPlugin(ctx, name)
	if err != nil {
		return nil, err
	}

	p.AcquireSession()
	defer func() {
		if err != nil {
			p.ReleaseSession()
		}
	}()

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.process == nil {
		return nil, errors.NewPluginError(name, "GetNodeClient", "plugin process is no longer running")
	}

	// Verify the plugin has the node capability
	hasNode := false
	for _, cap := range p.Capabilities {
		if cap.Name == NodeCapability {
			hasNode = true
			break
		}
	}

	if !hasNode {
		return nil, errors.NewPluginError(name, "GetNodeClient", "plugin does not support node capability")
	}

	if p.NodeClient == nil {
		if p.conn == nil {
			return nil, errors.NewPluginError(name, "GetNodeClient", "plugin connection not established")
		}
		p.NodeClient = apiv1.NewNodeExecutionServiceClient(p.conn)
	}

	return p.NodeClient, nil
}

// GetVCSClient returns a gRPC client for the specified VCS plugin.
// If the plugin is not running, it will be started.
func (m *Manager) GetVCSClient(ctx context.Context, name string) (client apiv1.VCSServiceClient, err error) {
	p, err := m.getOrStartPlugin(ctx, name)
	if err != nil {
		return nil, err
	}

	p.AcquireSession()
	defer func() {
		if err != nil {
			p.ReleaseSession()
		}
	}()

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.process == nil {
		return nil, errors.NewPluginError(name, "GetVCSClient", "plugin process is no longer running")
	}

	// Verify the plugin has the vcs capability
	hasVCS := false
	for _, cap := range p.Capabilities {
		if cap.Name == VCSCapability {
			hasVCS = true
			break
		}
	}

	if !hasVCS {
		return nil, errors.NewPluginError(name, "GetVCSClient", "plugin does not support vcs capability")
	}

	if p.VCSClient == nil {
		if p.conn == nil {
			return nil, errors.NewPluginError(name, "GetVCSClient", "plugin connection not established")
		}
		p.VCSClient = apiv1.NewVCSServiceClient(p.conn)
	}

	return p.VCSClient, nil
}

// GetTicketClient returns a gRPC client for the specified ticketing plugin.
// If the plugin is not running, it will be started.
func (m *Manager) GetTicketClient(ctx context.Context, name string) (client apiv1.TicketServiceClient, err error) {
	p, err := m.getOrStartPlugin(ctx, name)
	if err != nil {
		return nil, err
	}

	p.AcquireSession()
	defer func() {
		if err != nil {
			p.ReleaseSession()
		}
	}()

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.process == nil {
		return nil, errors.NewPluginError(name, "GetTicketClient", "plugin process is no longer running")
	}

	// Verify the plugin has the ticket capability
	hasTicket := false
	for _, cap := range p.Capabilities {
		if cap.Name == TicketCapability {
			hasTicket = true
			break
		}
	}

	if !hasTicket {
		return nil, errors.NewPluginError(name, "GetTicketClient", "plugin does not support ticket capability")
	}

	if p.TicketClient == nil {
		if p.conn == nil {
			return nil, errors.NewPluginError(name, "GetTicketClient", "plugin connection not established")
		}
		p.TicketClient = apiv1.NewTicketServiceClient(p.conn)
	}

	return p.TicketClient, nil
}

// GetKnowledgeClient returns a gRPC client for the specified knowledge plugin.
// If the plugin is not running, it will be started.
func (m *Manager) GetKnowledgeClient(ctx context.Context, name string) (client apiv1.KnowledgeServiceClient, err error) {
	p, err := m.getOrStartPlugin(ctx, name)
	if err != nil {
		return nil, err
	}

	p.AcquireSession()
	defer func() {
		if err != nil {
			p.ReleaseSession()
		}
	}()

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.process == nil {
		return nil, errors.NewPluginError(name, "GetKnowledgeClient", "plugin process is no longer running")
	}

	// Verify the plugin has the knowledge capability
	hasKnowledge := false
	for _, cap := range p.Capabilities {
		if cap.Name == KnowledgeCapability {
			hasKnowledge = true
			break
		}
	}

	if !hasKnowledge {
		return nil, errors.NewPluginError(name, "GetKnowledgeClient", "plugin does not support knowledge capability")
	}

	if p.KnowledgeClient == nil {
		if p.conn == nil {
			return nil, errors.NewPluginError(name, "GetKnowledgeClient", "plugin connection not established")
		}
		p.KnowledgeClient = apiv1.NewKnowledgeServiceClient(p.conn)
	}

	return p.KnowledgeClient, nil
}

func (m *Manager) getOrStartPlugin(ctx context.Context, name string) (*Plugin, error) {
	m.mu.Lock()
	p, ok := m.plugins[name]
	m.mu.Unlock()

	if ok {
		p.mu.Lock()
		running := p.process != nil
		p.mu.Unlock()
		if running {
			return p, nil
		}
	}

	// Not running or not found, try to discover it
	result, err := m.scanner.Scan()
	if err != nil {
		return nil, errors.Wrap(err, "failed to scan plugins")
	}

	var target *Plugin
	for _, found := range result.Plugins {
		if found.Name == name {
			target = found
			break
		}
	}

	if target == nil {
		return nil, ErrPluginNotFound
	}

	// Generate a unique secret token for this plugin instance
	u, err := uuid.NewRandom()
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate secret token")
	}
	target.secretToken = u.String()
	m.tokenStore.Register(target.secretToken, name)

	// Fetch plugin configuration if provider is available
	configJSON := []byte("{}")
	if m.configProvider != nil {
		data, err := m.configProvider(name)
		if err != nil {
			if m.logger != nil {
				m.logger.Debug("failed to get config for plugin", "plugin", name, "error", err)
			}
		} else if len(data) > 0 {
			configJSON = data

			// Unmarshal once; extract EnvAllowList and seed the resolver's
			// secret cache to avoid a redundant parse on first GetSecret.
			var cfg map[string]any
			if err := json.Unmarshal(data, &cfg); err == nil {
				if val, ok := cfg["env_allow_list"]; ok {
					target.EnvAllowList = toStringSlice(val)
				}
				if sec, ok := cfg["secrets"].(map[string]any); ok {
					m.secretCache.Store(name, sec)
				}
			}
		}
	}

	// Start the plugin
	if err := m.executor.Start(ctx, target); err != nil {
		m.tokenStore.Unregister(target.secretToken)
		return nil, errors.Wrapf(err, "failed to start plugin %q", name)
	}

	// Prepare the base client and handshake
	if err := m.executor.PrepareClient(target); err != nil {
		m.tokenStore.Unregister(target.secretToken)
		_ = m.executor.Stop(target)
		return nil, errors.Wrapf(err, "failed to prepare client for plugin %q", name)
	}

	// Perform handshake with host version and API contract version
	if err := m.executor.Handshake(ctx, target, m.rigVersion, APIVersion, configJSON); err != nil {
		m.tokenStore.Unregister(target.secretToken)
		_ = m.executor.Stop(target)
		return nil, errors.Wrapf(err, "handshake failed for plugin %q", name)
	}

	// Validate compatibility with host Rig version after handshake
	// (which might have updated the plugin's metadata/version).
	ValidateCompatibility(target, m.rigVersion)
	if target.Status == StatusIncompatible || target.Status == StatusError {
		m.tokenStore.Unregister(target.secretToken)
		_ = m.executor.Stop(target)
		if target.Error != nil {
			return nil, errors.Wrapf(target.Error, "plugin %q is incompatible", name)
		}
		return nil, errors.NewPluginError(name, "Compatibility", "plugin is incompatible with this version of rig")
	}

	m.mu.Lock()
	target.lastUsed = time.Now()
	m.plugins[name] = target
	m.mu.Unlock()
	return target, nil
}

// StopAll stops all managed plugins and the host UI server.
func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, p := range m.plugins {
		m.tokenStore.UnregisterPlugin(p.Name)
		_ = m.executor.Stop(p)
	}
	m.plugins = make(map[string]*Plugin)

	if m.hostServer != nil {
		m.hostServer.GracefulStop()
		m.hostServer = nil
	}

	if m.hostUI != nil {
		if s, ok := m.hostUI.(interface{ Stop() }); ok {
			s.Stop()
		}
		m.hostUI = nil
	}

	if m.hostL != nil {
		_ = m.hostL.Close()
		m.hostL = nil
	}

	if m.hostDir != "" {
		_ = os.RemoveAll(m.hostDir)
		m.hostDir = ""
		m.hostPath = ""
	}

	// Reset host endpoint in executor to avoid stale environment variables
	m.executor.SetHostEndpoint("")
}

// ReleasePlugin signals that a session with the plugin has finished.
func (m *Manager) ReleasePlugin(name string) {
	m.mu.Lock()
	p, ok := m.plugins[name]
	m.mu.Unlock()

	if ok {
		p.ReleaseSession()
	}
}

// StopPluginIfIdle stops a specific plugin by name if it is not busy and has been
// idle for longer than the provided timeout. If idleTimeout is 0, it is stopped
// as long as it is not busy.
func (m *Manager) StopPluginIfIdle(name string, idleTimeout time.Duration) error {
	m.mu.Lock()
	p, ok := m.plugins[name]
	if !ok {
		m.mu.Unlock()
		return nil
	}

	p.mu.Lock()
	busy := p.activeSessions > 0
	lastUsed := p.lastUsed
	p.mu.Unlock()

	if busy || (idleTimeout > 0 && time.Since(lastUsed) <= idleTimeout) {
		m.mu.Unlock()
		return nil
	}

	// Atomically unregister the token and remove from the map while
	// still holding m.mu, so no concurrent caller can observe a
	// half-detached plugin.
	m.tokenStore.UnregisterPlugin(p.Name)
	delete(m.plugins, name)
	m.mu.Unlock()

	// Stop the fully detached plugin outside the lock to avoid
	// holding m.mu during a potentially slow process teardown.
	return m.executor.Stop(p)
}

// ListPlugins returns a list of all currently managed plugins.
func (m *Manager) ListPlugins() []*Plugin {
	m.mu.Lock()
	defer m.mu.Unlock()

	plugins := make([]*Plugin, 0, len(m.plugins))
	for _, p := range m.plugins {
		p.mu.Lock()
		// Return a deep-enough copy to prevent external mutation of internal state.
		// We manually copy fields to avoid copying the mutex and runtime handles.

		// Copy Args slice
		var args []string
		if p.Args != nil {
			args = make([]string, len(p.Args))
			copy(args, p.Args)
		}

		// Copy Capabilities slice and objects
		var caps []*apiv1.Capability
		if p.Capabilities != nil {
			caps = make([]*apiv1.Capability, len(p.Capabilities))
			for i, c := range p.Capabilities {
				caps[i] = &apiv1.Capability{
					Name:    c.Name,
					Version: c.Version,
				}
			}
		}

		// Copy Manifest and its Commands slice
		var manifest *Manifest
		if p.Manifest != nil {
			mCopy := *p.Manifest
			if p.Manifest.Commands != nil {
				cmds := make([]CommandDescriptor, len(p.Manifest.Commands))
				copy(cmds, p.Manifest.Commands)
				mCopy.Commands = cmds
			}
			manifest = &mCopy
		}

		var envAllowList []string
		if p.EnvAllowList != nil {
			envAllowList = make([]string, len(p.EnvAllowList))
			copy(envAllowList, p.EnvAllowList)
		}

		pCopy := &Plugin{
			Name:         p.Name,
			Version:      p.Version,
			APIVersion:   p.APIVersion,
			Path:         p.Path,
			Args:         args,
			EnvAllowList: envAllowList,
			Source:       p.Source,
			Status:       p.Status,
			Description:  p.Description,
			Manifest:     manifest,
			Error:        p.Error, // Error is an interface, effectively immutable
			DiscoveryAt:  p.DiscoveryAt,
			lastUsed:     p.lastUsed,
			Capabilities: caps,
		}
		p.mu.Unlock()
		plugins = append(plugins, pCopy)
	}
	return plugins
}

func toStringSlice(i any) []string {
	if i == nil {
		return nil
	}
	if s, ok := i.([]string); ok {
		return s
	}
	if slice, ok := i.([]any); ok {
		res := make([]string, 0, len(slice))
		for _, v := range slice {
			if s, ok := v.(string); ok {
				res = append(res, s)
			}
		}
		return res
	}
	return nil
}
