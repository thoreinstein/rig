package orchestration

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
	"thoreinstein.com/rig/pkg/errors"
	"thoreinstein.com/rig/pkg/plugin"
)

// NodeBridge is the interface for executing a workflow node in an isolated context.
type NodeBridge interface {
	ExecuteNode(
		ctx context.Context,
		node *Node,
		caps *NodeCapabilities,
		pluginConfig json.RawMessage,
		inputs map[string]json.RawMessage,
		secrets map[string]string,
	) (json.RawMessage, error)
}

// PluginNodeBridge implements NodeBridge using the Rig plugin system.
type PluginNodeBridge struct {
	pluginMgr *plugin.Manager
	logger    *slog.Logger
}

// NewPluginNodeBridge creates a new PluginNodeBridge.
func NewPluginNodeBridge(pluginMgr *plugin.Manager, logger *slog.Logger) *PluginNodeBridge {
	if logger == nil {
		logger = slog.Default()
	}
	return &PluginNodeBridge{
		pluginMgr: pluginMgr,
		logger:    logger,
	}
}

// ExecuteNode executes the node via a plugin, setting up the ResourceService callback.
func (b *PluginNodeBridge) ExecuteNode(
	ctx context.Context,
	node *Node,
	caps *NodeCapabilities,
	pluginConfig json.RawMessage,
	inputs map[string]json.RawMessage,
	secrets map[string]string,
) (json.RawMessage, error) {
	// 1. Get the Node client for the specific plugin type
	client, err := b.pluginMgr.GetNodeClient(ctx, node.Type)
	if err != nil {
		if errors.IsPluginError(err) || errors.Is(err, plugin.ErrPluginNotFound) {
			return nil, errors.Wrapf(err, "failed to get plugin client for node type %q", node.Type)
		}
		return nil, errors.Wrap(err, "failed to get plugin client")
	}
	defer b.pluginMgr.ReleasePlugin(node.Type)

	// 2. Setup isolated UDS for the ResourceService for this specific node execution
	socketDir, err := os.MkdirTemp("", "rig-r-")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create resource socket dir")
	}
	defer os.RemoveAll(socketDir)
	if err := os.Chmod(socketDir, 0o700); err != nil {
		return nil, errors.Wrap(err, "failed to secure resource socket dir")
	}

	u, _ := uuid.NewRandom()
	socketPath := filepath.Join(socketDir, fmt.Sprintf("res-%s.sock", u.String()[:8]))

	lis, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to listen on resource socket")
	}
	defer lis.Close()
	if err := os.Chmod(socketPath, 0o600); err != nil {
		return nil, errors.Wrap(err, "failed to secure resource socket")
	}

	// 3. Start ResourceService gRPC server
	srv := grpc.NewServer()
	resServer := newResourceServer(node.ID, caps)
	apiv1.RegisterResourceServiceServer(srv, resServer)

	errChan := make(chan error, 1)
	go func() {
		if err := srv.Serve(lis); err != nil && err != grpc.ErrServerStopped {
			b.logger.Error("resource server failed", "node", node.ID, "error", err)
			errChan <- err
		}
		close(errChan)
	}()
	defer srv.GracefulStop()

	// Check for immediate server failure
	select {
	case err := <-errChan:
		if err != nil {
			return nil, fmt.Errorf("resource server failed to start: %w", err)
		}
	default:
		// Server started successfully, continue
	}

	// 4. Prepare inputs
	reqInputs := make(map[string][]byte)
	for k, v := range inputs {
		reqInputs[k] = []byte(v)
	}

	req := &apiv1.ExecuteNodeRequest{
		NodeId:     node.ID,
		NodeType:   node.Type,
		ConfigJson: []byte(pluginConfig),
		Inputs:     reqInputs,
		Secrets:    secrets,
		Workspace:  caps.Workspace,
	}

	// 5. Execute via plugin client
	// We pass the resource socket path via gRPC metadata so the plugin knows where to call back.
	// We'll just pass it in context metadata since that's standard for gRPC headers.
	outCtx := metadata.AppendToOutgoingContext(ctx, "rig-resource-endpoint", socketPath)

	resp, err := client.ExecuteNode(outCtx, req)
	if err != nil {
		return nil, errors.Wrapf(err, "node execution failed for %q", node.ID)
	}

	if resp.ErrorMessage != "" {
		return nil, errors.Newf("node error: %s", resp.ErrorMessage)
	}

	return json.RawMessage(resp.Output), nil
}
