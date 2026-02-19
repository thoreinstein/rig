package ai

import (
	"context"
	"io"
	"log/slog"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
)

// PluginAssistantProvider implements the Provider interface by communicating
// with a Rig plugin over gRPC.
type PluginAssistantProvider struct {
	name   string
	client apiv1.AssistantServiceClient
	logger *slog.Logger
}

// NewPluginAssistantProvider creates a new AI provider backed by a plugin.
func NewPluginAssistantProvider(name string, client apiv1.AssistantServiceClient, logger *slog.Logger) *PluginAssistantProvider {
	return &PluginAssistantProvider{
		name:   name,
		client: client,
		logger: logger,
	}
}

// IsAvailable always returns true for a connected plugin.
func (p *PluginAssistantProvider) IsAvailable() bool {
	return p.client != nil
}

// Chat performs a single-turn chat completion via gRPC.
func (p *PluginAssistantProvider) Chat(ctx context.Context, messages []Message) (*Response, error) {
	req := &apiv1.ChatRequest{
		Messages: p.toProtoMessages(messages),
	}

	resp, err := p.client.Chat(ctx, req)
	if err != nil {
		return nil, err
	}

	return &Response{
		Content:      resp.Content,
		StopReason:   resp.StopReason,
		InputTokens:  int(resp.InputTokens),
		OutputTokens: int(resp.OutputTokens),
	}, nil
}

// StreamChat performs a streaming chat completion via gRPC.
func (p *PluginAssistantProvider) StreamChat(ctx context.Context, messages []Message) (<-chan StreamChunk, error) {
	req := &apiv1.StreamChatRequest{
		Messages: p.toProtoMessages(messages),
	}

	stream, err := p.client.StreamChat(ctx, req)
	if err != nil {
		return nil, err
	}

	out := make(chan StreamChunk)
	go func() {
		defer close(out)
		for {
			chunk, err := stream.Recv()
			if err == io.EOF {
				return
			}
			if err != nil {
				select {
				case out <- StreamChunk{Error: err}:
				case <-ctx.Done():
				}
				return
			}

			select {
			case out <- StreamChunk{
				Content: chunk.Content,
				Done:    chunk.Done,
			}:
				if chunk.Done {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return out, nil
}

// Name returns the provider name (usually the plugin's ID or configured name).
func (p *PluginAssistantProvider) Name() string {
	return p.name
}

func (p *PluginAssistantProvider) toProtoMessages(messages []Message) []*apiv1.Message {
	protoMsgs := make([]*apiv1.Message, len(messages))
	for i, m := range messages {
		protoMsgs[i] = &apiv1.Message{
			Role:    m.Role,
			Content: m.Content,
		}
	}
	return protoMsgs
}
