package plugin

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
	"thoreinstein.com/rig/pkg/errors"
)

// PrepareClient sets up the gRPC client for the plugin.
// Note that it does not actually establish a network connection; gRPC connections
// are created lazily when the first RPC is invoked.
func (e *Executor) PrepareClient(p *Plugin) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.socketPath == "" {
		return errors.NewPluginError(p.Name, "Dial", "plugin socket path not set")
	}

	if p.conn != nil {
		return errors.NewPluginError(p.Name, "Dial", "plugin client already initialized; call Stop/cleanup first")
	}

	// Dial the Unix Domain Socket using grpc.NewClient (preferred over DialContext)
	conn, err := grpc.NewClient("unix://"+p.socketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return errors.NewPluginError(p.Name, "Dial", "failed to create gRPC client").WithCause(err)
	}

	p.conn = conn
	p.client = apiv1.NewPluginServiceClient(conn)
	return nil
}

// Handshake performs the initial handshake with the plugin to verify compatibility.
func (e *Executor) Handshake(ctx context.Context, p *Plugin, rigVersion, apiVersion string) error {
	p.mu.Lock()
	client := p.client
	p.mu.Unlock()

	if client == nil {
		return errors.NewPluginError(p.Name, "Handshake", "plugin client not initialized; call PrepareClient first")
	}

	resp, err := client.Handshake(ctx, &apiv1.HandshakeRequest{
		RigVersion: rigVersion,
		ApiVersion: apiVersion,
	})
	if err != nil {
		return errors.NewPluginError(p.Name, "Handshake", "failed to verify plugin compatibility").WithCause(err)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Update plugin metadata from handshake response.
	// Priority: New fields (3, 4, 5, 6) then legacy fields (1, 2).

	// APIVersion is the Plugin API contract version implemented by the plugin.
	if resp.ApiVersion != "" {
		p.APIVersion = resp.ApiVersion
	}

	// Source: Source plugin semantic version (binary version).
	// TODO: plugin semantic version is sourced from the deprecated resp.PluginVersion
	// until a non-deprecated response field (e.g., plugin_semver) or manifest-based
	// source is available.
	if resp.PluginSemver != "" {
		p.Version = resp.PluginSemver
	} else if resp.PluginVersion != "" { //nolint:staticcheck // intentional use of deprecated field for compatibility
		p.Version = resp.PluginVersion //nolint:staticcheck // intentional use of deprecated field for compatibility
	}

	// Intent: resp.PluginId is intentionally used as the plugin's display name (p.Name)
	// for backward compatibility with existing Rig naming patterns.
	if resp.PluginId != "" {
		p.Name = resp.PluginId
	}

	// Handle capabilities transition
	if len(resp.Capabilities) > 0 {
		p.Capabilities = resp.Capabilities
	} else if len(resp.CapabilitiesDeprecated) > 0 { //nolint:staticcheck // intentional use of deprecated field for compatibility
		// Translate old string capabilities to new structured ones
		p.Capabilities = make([]*apiv1.Capability, len(resp.CapabilitiesDeprecated)) //nolint:staticcheck // intentional use of deprecated field for compatibility
		for i, name := range resp.CapabilitiesDeprecated {                           //nolint:staticcheck // intentional use of deprecated field for compatibility
			p.Capabilities[i] = &apiv1.Capability{
				Name:    name,
				Version: "v0.0.0", // Default version for legacy capabilities
			}
		}
	} else {
		// Explicitly clear capabilities if neither field is populated.
		// This prevents stale state from previous handshakes.
		p.Capabilities = nil
	}

	return nil
}
