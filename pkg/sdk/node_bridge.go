package sdk

import (
	"context"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
)

// nodeBridge wraps a NodeHandler to implement the NodeExecutionServiceServer gRPC interface.
type nodeBridge struct {
	apiv1.UnimplementedNodeExecutionServiceServer
	handler NodeHandler
}

// ExecuteNode delegates the execution to the underlying NodeHandler.
func (b *nodeBridge) ExecuteNode(ctx context.Context, req *apiv1.ExecuteNodeRequest) (*apiv1.ExecuteNodeResponse, error) {
	return b.handler.ExecuteNode(ctx, req)
}
