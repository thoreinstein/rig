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
