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
func (e *Executor) Handshake(ctx context.Context, p *Plugin, apiVersion string) error {
	p.mu.Lock()
	client := p.client
	p.mu.Unlock()

	if client == nil {
		return errors.NewPluginError(p.Name, "Handshake", "plugin client not initialized; call PrepareClient first")
	}

	resp, err := client.Handshake(ctx, &apiv1.HandshakeRequest{
		ApiVersion: apiVersion,
	})
	if err != nil {
		return errors.NewPluginError(p.Name, "Handshake", "failed to verify plugin compatibility").WithCause(err)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Update plugin metadata from handshake response
	if resp.ApiVersion != "" {
		p.ApiVersion = resp.ApiVersion
	}
	if resp.PluginId != "" {
		p.Name = resp.PluginId
	}
	p.Capabilities = resp.Capabilities

	return nil
}
