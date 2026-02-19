package ai

import (
	"context"
	"io"
	"strings"
	"testing"

	"google.golang.org/grpc"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
)

type mockAssistantServer struct {
	apiv1.UnimplementedAssistantServiceServer
	chatFn       func(ctx context.Context, req *apiv1.ChatRequest) (*apiv1.ChatResponse, error)
	streamChatFn func(req *apiv1.StreamChatRequest, stream apiv1.AssistantService_StreamChatServer) error
}

func (m *mockAssistantServer) Chat(ctx context.Context, req *apiv1.ChatRequest) (*apiv1.ChatResponse, error) {
	return m.chatFn(ctx, req)
}

func (m *mockAssistantServer) StreamChat(req *apiv1.StreamChatRequest, stream apiv1.AssistantService_StreamChatServer) error {
	return m.streamChatFn(req, stream)
}

// mockAssistantClient implements AssistantServiceClient for testing
type mockAssistantClient struct {
	server *mockAssistantServer
}

func (m *mockAssistantClient) Chat(ctx context.Context, in *apiv1.ChatRequest, opts ...grpc.CallOption) (*apiv1.ChatResponse, error) {
	return m.server.Chat(ctx, in)
}

func (m *mockAssistantClient) StreamChat(ctx context.Context, in *apiv1.StreamChatRequest, opts ...grpc.CallOption) (apiv1.AssistantService_StreamChatClient, error) {
	return &mockStreamClient{ctx: ctx, req: in, server: m.server}, nil
}

type mockStreamClient struct {
	grpc.ClientStream
	ctx    context.Context
	req    *apiv1.StreamChatRequest
	server *mockAssistantServer

	// Internal state to simulate the stream
	chunks  chan *apiv1.StreamChatResponse
	err     error
	started bool
}

func (m *mockStreamClient) Recv() (*apiv1.StreamChatResponse, error) {
	if !m.started {
		m.started = true
		m.chunks = make(chan *apiv1.StreamChatResponse, 10)
		go func() {
			err := m.server.StreamChat(m.req, &mockStreamServer{chunks: m.chunks, ctx: m.ctx})
			if err != nil && err != io.EOF {
				m.err = err
			}
			close(m.chunks)
		}()
	}

	chunk, ok := <-m.chunks
	if !ok {
		if m.err != nil {
			return nil, m.err
		}
		return nil, io.EOF
	}
	return chunk, nil
}

type mockStreamServer struct {
	grpc.ServerStream
	chunks chan *apiv1.StreamChatResponse
	ctx    context.Context
}

func (m *mockStreamServer) Send(chunk *apiv1.StreamChatResponse) error {
	m.chunks <- chunk
	return nil
}

func (m *mockStreamServer) Context() context.Context {
	return m.ctx
}

func TestPluginAssistantProvider_Chat(t *testing.T) {
	mockServer := &mockAssistantServer{
		chatFn: func(ctx context.Context, req *apiv1.ChatRequest) (*apiv1.ChatResponse, error) {
			return &apiv1.ChatResponse{
				Content:      "Hello from plugin",
				StopReason:   "end_turn",
				InputTokens:  5,
				OutputTokens: 10,
			}, nil
		},
	}

	client := &mockAssistantClient{server: mockServer}
	provider := NewPluginAssistantProvider("test-plugin", client, nil)

	resp, err := provider.Chat(t.Context(), []Message{{Role: "user", Content: "hi"}})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}

	if resp.Content != "Hello from plugin" {
		t.Errorf("expected 'Hello from plugin', got %q", resp.Content)
	}
	if resp.InputTokens != 5 {
		t.Errorf("expected 5 tokens, got %d", resp.InputTokens)
	}
}

func TestPluginAssistantProvider_StreamChat(t *testing.T) {
	mockServer := &mockAssistantServer{
		streamChatFn: func(req *apiv1.StreamChatRequest, stream apiv1.AssistantService_StreamChatServer) error {
			_ = stream.Send(&apiv1.StreamChatResponse{Content: "Part 1"})
			_ = stream.Send(&apiv1.StreamChatResponse{Content: " Part 2", Done: true})
			return nil
		},
	}

	client := &mockAssistantClient{server: mockServer}
	provider := NewPluginAssistantProvider("test-plugin", client, nil)

	chunks, err := provider.StreamChat(t.Context(), []Message{{Role: "user", Content: "hi"}})
	if err != nil {
		t.Fatalf("StreamChat() error = %v", err)
	}

	var content strings.Builder
	var gotDone bool
	for chunk := range chunks {
		if chunk.Error != nil {
			t.Fatalf("chunk error = %v", chunk.Error)
		}
		content.WriteString(chunk.Content)
		if chunk.Done {
			gotDone = true
		}
	}

	if content.String() != "Part 1 Part 2" {
		t.Errorf("expected 'Part 1 Part 2', got %q", content.String())
	}
	if !gotDone {
		t.Error("expected done chunk")
	}
}
