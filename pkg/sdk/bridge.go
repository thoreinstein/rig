package sdk

import (
	"context"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
)

// pluginBridge adapts a PluginInfo implementation to the apiv1.PluginServiceServer interface.
type pluginBridge struct {
	apiv1.UnimplementedPluginServiceServer
	p PluginInfo
}

func newPluginBridge(p PluginInfo) *pluginBridge {
	return &pluginBridge{p: p}
}

func (b *pluginBridge) Handshake(ctx context.Context, req *apiv1.HandshakeRequest) (*apiv1.HandshakeResponse, error) {
	if c, ok := b.p.(Configurable); ok && len(req.ConfigJson) > 0 {
		if err := c.Configure(req.ConfigJson); err != nil {
			return nil, err
		}
	}

	info := b.p.Info()

	caps := make([]*apiv1.Capability, len(info.Capabilities))
	for i, c := range info.Capabilities {
		caps[i] = &apiv1.Capability{
			Name:    c.Name,
			Version: c.Version,
		}
	}

	cmds := make([]*apiv1.CommandDescriptorProto, len(info.Commands))
	for i, c := range info.Commands {
		cmds[i] = &apiv1.CommandDescriptorProto{
			Name:    c.Name,
			Short:   c.Short,
			Long:    c.Long,
			Aliases: c.Aliases,
		}
	}

	return &apiv1.HandshakeResponse{
		PluginId:     info.ID,
		ApiVersion:   info.APIVersion,
		PluginSemver: info.SemVer,
		Capabilities: caps,
		Commands:     cmds,
	}, nil
}

// assistantBridge adapts an AssistantHandler implementation to the apiv1.AssistantServiceServer interface.
type assistantBridge struct {
	apiv1.UnimplementedAssistantServiceServer
	h AssistantHandler
}

func newAssistantBridge(h AssistantHandler) *assistantBridge {
	return &assistantBridge{h: h}
}

func (b *assistantBridge) Chat(ctx context.Context, req *apiv1.ChatRequest) (*apiv1.ChatResponse, error) {
	resp, err := b.h.Chat(ctx, req)
	return resp, mapError(err)
}

func (b *assistantBridge) StreamChat(req *apiv1.StreamChatRequest, server apiv1.AssistantService_StreamChatServer) error {
	err := b.h.StreamChat(req, server)
	return mapError(err)
}

// commandBridge adapts a CommandHandler implementation to the apiv1.CommandServiceServer interface.
type commandBridge struct {
	apiv1.UnimplementedCommandServiceServer
	h CommandHandler
}

func newCommandBridge(h CommandHandler) *commandBridge {
	return &commandBridge{h: h}
}

func (b *commandBridge) Execute(req *apiv1.ExecuteRequest, server apiv1.CommandService_ExecuteServer) error {
	err := b.h.Execute(req, server)
	return mapError(err)
}

// vcsBridge adapts a VCSHandler implementation to the apiv1.VCSServiceServer interface.
type vcsBridge struct {
	apiv1.UnimplementedVCSServiceServer
	h VCSHandler
}

func newVCSBridge(h VCSHandler) *vcsBridge {
	return &vcsBridge{h: h}
}

func (b *vcsBridge) GetRepoRoot(ctx context.Context, req *apiv1.GetRepoRootRequest) (*apiv1.GetRepoRootResponse, error) {
	resp, err := b.h.GetRepoRoot(ctx, req)
	return resp, mapError(err)
}

func (b *vcsBridge) GetRepoName(ctx context.Context, req *apiv1.GetRepoNameRequest) (*apiv1.GetRepoNameResponse, error) {
	resp, err := b.h.GetRepoName(ctx, req)
	return resp, mapError(err)
}

func (b *vcsBridge) GetDefaultBranch(ctx context.Context, req *apiv1.GetDefaultBranchRequest) (*apiv1.GetDefaultBranchResponse, error) {
	resp, err := b.h.GetDefaultBranch(ctx, req)
	return resp, mapError(err)
}

func (b *vcsBridge) CreateWorktree(ctx context.Context, req *apiv1.CreateWorktreeRequest) (*apiv1.CreateWorktreeResponse, error) {
	resp, err := b.h.CreateWorktree(ctx, req)
	return resp, mapError(err)
}

func (b *vcsBridge) ListWorktrees(ctx context.Context, req *apiv1.ListWorktreesRequest) (*apiv1.ListWorktreesResponse, error) {
	resp, err := b.h.ListWorktrees(ctx, req)
	return resp, mapError(err)
}

func (b *vcsBridge) RemoveWorktree(ctx context.Context, req *apiv1.RemoveWorktreeRequest) (*apiv1.RemoveWorktreeResponse, error) {
	resp, err := b.h.RemoveWorktree(ctx, req)
	return resp, mapError(err)
}

func (b *vcsBridge) ForceRemoveWorktree(ctx context.Context, req *apiv1.ForceRemoveWorktreeRequest) (*apiv1.ForceRemoveWorktreeResponse, error) {
	resp, err := b.h.ForceRemoveWorktree(ctx, req)
	return resp, mapError(err)
}

func (b *vcsBridge) GetWorktreePath(ctx context.Context, req *apiv1.GetWorktreePathRequest) (*apiv1.GetWorktreePathResponse, error) {
	resp, err := b.h.GetWorktreePath(ctx, req)
	return resp, mapError(err)
}

func (b *vcsBridge) Clone(ctx context.Context, req *apiv1.CloneRequest) (*apiv1.CloneResponse, error) {
	resp, err := b.h.Clone(ctx, req)
	return resp, mapError(err)
}

func (b *vcsBridge) IsBranchMerged(ctx context.Context, req *apiv1.IsBranchMergedRequest) (*apiv1.IsBranchMergedResponse, error) {
	resp, err := b.h.IsBranchMerged(ctx, req)
	return resp, mapError(err)
}

// ticketBridge adapts a TicketHandler implementation to the apiv1.TicketServiceServer interface.
type ticketBridge struct {
	apiv1.UnimplementedTicketServiceServer
	h TicketHandler
}

func newTicketBridge(h TicketHandler) *ticketBridge {
	return &ticketBridge{h: h}
}

func (b *ticketBridge) GetTicketInfo(ctx context.Context, req *apiv1.GetTicketInfoRequest) (*apiv1.GetTicketInfoResponse, error) {
	resp, err := b.h.GetTicketInfo(ctx, req)
	return resp, mapError(err)
}

func (b *ticketBridge) UpdateTicketStatus(ctx context.Context, req *apiv1.UpdateTicketStatusRequest) (*apiv1.UpdateTicketStatusResponse, error) {
	resp, err := b.h.UpdateTicketStatus(ctx, req)
	return resp, mapError(err)
}

func (b *ticketBridge) ListTransitions(ctx context.Context, req *apiv1.ListTransitionsRequest) (*apiv1.ListTransitionsResponse, error) {
	resp, err := b.h.ListTransitions(ctx, req)
	return resp, mapError(err)
}
