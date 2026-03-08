package plugin

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
)

// PluginContext holds current environment metadata provided to plugins.
type PluginContext struct {
	ProjectRoot  string
	WorktreeRoot string
	TicketID     string
	Metadata     map[string]any
}

// HostContextProxy implements apiv1.ContextServiceServer.
type HostContextProxy struct {
	apiv1.UnimplementedContextServiceServer
	store   *tokenStore
	context PluginContext
}

// NewHostContextProxy creates a new HostContextProxy.
func NewHostContextProxy(store *tokenStore, ctx PluginContext) *HostContextProxy {
	return &HostContextProxy{
		store:   store,
		context: ctx,
	}
}

// GetContext returns the current environment context.
func (p *HostContextProxy) GetContext(ctx context.Context, req *apiv1.GetContextRequest) (*apiv1.GetContextResponse, error) {
	if p.store == nil {
		return nil, status.Errorf(codes.Internal, "token store not initialized")
	}
	pluginName, ok := p.store.Resolve(req.Token)
	if !ok || pluginName == "" {
		return nil, status.Errorf(codes.Unauthenticated, "invalid secret token")
	}

	metadata, err := structpb.NewStruct(p.context.Metadata)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to serialize metadata: %v", err)
	}

	return &apiv1.GetContextResponse{
		ProjectRoot:  p.context.ProjectRoot,
		WorktreeRoot: p.context.WorktreeRoot,
		TicketId:     p.context.TicketID,
		Metadata:     metadata,
	}, nil
}
