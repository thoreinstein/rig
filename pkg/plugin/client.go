package plugin

import (
	"context"

	"github.com/cockroachdb/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
)

// Dial establishes a gRPC connection to the plugin's UDS socket.
func (e *Executor) Dial(ctx context.Context, p *Plugin) error {
	if p.socketPath == "" {
		return errors.New("plugin socket path not set")
	}

	// Dial the Unix Domain Socket using grpc.NewClient (preferred over DialContext)
	conn, err := grpc.NewClient("unix://"+p.socketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return errors.Wrapf(err, "failed to create gRPC client for plugin %s at %s", p.Name, p.socketPath)
	}

	p.client = apiv1.NewPluginServiceClient(conn)
	return nil
}

// Handshake performs the initial handshake with the plugin to verify compatibility.
func (e *Executor) Handshake(ctx context.Context, p *Plugin, rigVersion string) error {
	if p.client == nil {
		return errors.New("plugin client not initialized; call Dial first")
	}

	resp, err := p.client.Handshake(ctx, &apiv1.HandshakeRequest{
		RigVersion: rigVersion,
	})
	if err != nil {
		return errors.Wrapf(err, "handshake failed for plugin %s", p.Name)
	}

	// Update plugin metadata if provided by the handshake
	if resp.PluginVersion != "" {
		p.Version = resp.PluginVersion
	}

	return nil
}
