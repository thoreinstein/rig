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
	store     *tokenStore
	pluginCtx PluginContext
	metadata  *structpb.Struct
}

// NewHostContextProxy creates a new HostContextProxy.
// Metadata is serialized once at construction time; if the map contains
// values that structpb cannot represent, metadata will be nil and
// GetContext will return an Internal error.
func NewHostContextProxy(store *tokenStore, ctx PluginContext) *HostContextProxy {
	// Pre-build the protobuf struct once. Errors are deferred to GetContext.
	md, _ := structpb.NewStruct(ctx.Metadata)
	return &HostContextProxy{
		store:     store,
		pluginCtx: ctx,
		metadata:  md,
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

	if p.metadata == nil {
		return nil, status.Errorf(codes.Internal, "failed to serialize metadata")
	}

	return &apiv1.GetContextResponse{
		ProjectRoot:  p.pluginCtx.ProjectRoot,
		WorktreeRoot: p.pluginCtx.WorktreeRoot,
		TicketId:     p.pluginCtx.TicketID,
		Metadata:     p.metadata,
	}, nil
}
