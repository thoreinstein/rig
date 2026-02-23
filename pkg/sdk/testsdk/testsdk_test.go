package testsdk

import (
	"context"
	"testing"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
	"thoreinstein.com/rig/pkg/sdk"
)

type mockPlugin struct {
	info sdk.Info
}

func (m *mockPlugin) Info() sdk.Info {
	return m.info
}

func TestMockHost(t *testing.T) {
	h := StartMockHost(t)
	h.UI.PromptFunc = func(req *apiv1.PromptRequest) (*apiv1.PromptResponse, error) {
		return &apiv1.PromptResponse{Value: "Bob"}, nil
	}

	ui := h.NewTestUI()
	defer ui.Close()

	val, err := ui.Prompt(t.Context(), "name?")
	if err != nil {
		t.Fatalf("Prompt failed: %v", err)
	}
	if val != "Bob" {
		t.Errorf("got prompt value %q, want %q", val, "Bob")
	}

	if len(h.UI.PromptCalls) != 1 || h.UI.PromptCalls[0].Label != "name?" {
		t.Errorf("got unexpected prompt calls: %v", h.UI.PromptCalls)
	}
}

func TestServePlugin(t *testing.T) {
	info := sdk.Info{ID: "test-plugin"}
	conn := ServePlugin(t, &mockPlugin{info: info})
	defer conn.Close()

	client := apiv1.NewPluginServiceClient(conn)
	resp, err := client.Handshake(t.Context(), &apiv1.HandshakeRequest{})
	if err != nil {
		t.Fatalf("Handshake failed: %v", err)
	}

	if resp.PluginId != info.ID {
		t.Errorf("got PluginId %q, want %q", resp.PluginId, info.ID)
	}
}

func TestServePlugin_Services(t *testing.T) {
	info := sdk.Info{ID: "assistant-plugin"}
	p := &mockAssistant{info: info}
	conn := ServePlugin(t, p)
	defer conn.Close()

	// Assistant service should be registered
	asst := apiv1.NewAssistantServiceClient(conn)
	_, err := asst.Chat(t.Context(), &apiv1.ChatRequest{})
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	// Command service should NOT be registered
	cmd := apiv1.NewCommandServiceClient(conn)
	stream, err := cmd.Execute(t.Context(), &apiv1.ExecuteRequest{})
	if err != nil {
		t.Fatalf("Execute stream initiation failed: %v", err)
	}
	_, err = stream.Recv()
	if err == nil {
		t.Error("expected Execute to fail with Unimplemented")
	}
}

type mockAssistant struct {
	info sdk.Info
}

func (m *mockAssistant) Info() sdk.Info {
	return m.info
}

func (m *mockAssistant) Chat(ctx context.Context, req *apiv1.ChatRequest) (*apiv1.ChatResponse, error) {
	return &apiv1.ChatResponse{Content: "hi"}, nil
}

func (m *mockAssistant) StreamChat(req *apiv1.StreamChatRequest, server apiv1.AssistantService_StreamChatServer) error {
	return nil
}
