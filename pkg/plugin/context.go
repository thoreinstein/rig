package plugin

import (
	"context"
	"log/slog"

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
	store       *tokenStore
	logger      *slog.Logger
	pluginCtx   PluginContext
	metadata    *structpb.Struct
	metadataErr error
}

// NewHostContextProxy creates a new HostContextProxy.
// Metadata is serialized once at construction time; if the map contains
// values that structpb cannot represent, metadata will be nil and
// GetContext will return an Internal error.
func NewHostContextProxy(store *tokenStore, ctx PluginContext, logger *slog.Logger) *HostContextProxy {
	// Pre-build the protobuf struct once. Errors are deferred to GetContext.
	md, mdErr := structpb.NewStruct(ctx.Metadata)
	return &HostContextProxy{
		store:       store,
		logger:      logger,
		pluginCtx:   ctx,
		metadata:    md,
		metadataErr: mdErr,
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

	if p.metadataErr != nil {
		if p.logger != nil {
			p.logger.Error("failed to serialize plugin context metadata", "error", p.metadataErr)
		}
		return nil, status.Errorf(codes.Internal, "metadata unavailable")
	}

	return &apiv1.GetContextResponse{
		ProjectRoot:  p.pluginCtx.ProjectRoot,
		WorktreeRoot: p.pluginCtx.WorktreeRoot,
		TicketId:     p.pluginCtx.TicketID,
		Metadata:     p.metadata, // nil when Metadata map is nil (no-op)
	}, nil
}
