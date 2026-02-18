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

	// Update plugin metadata from handshake response
	// Priority: New fields (3, 4, 5) then legacy fields (1, 2)
	if resp.ApiVersion != "" {
		p.ApiVersion = resp.ApiVersion
	}
	if resp.PluginVersion != "" {
		p.Version = resp.PluginVersion
	}
	if resp.PluginId != "" {
		p.Name = resp.PluginId
	}

	// Handle capabilities transition
	if len(resp.Capabilities) > 0 {
		p.Capabilities = resp.Capabilities
	} else if len(resp.CapabilitiesDeprecated) > 0 {
		// Translate old string capabilities to new structured ones
		p.Capabilities = make([]*apiv1.Capability, len(resp.CapabilitiesDeprecated))
		for i, name := range resp.CapabilitiesDeprecated {
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
