package knowledge_test

import (
	"context"
	"testing"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
	"thoreinstein.com/rig/pkg/sdk"
	"thoreinstein.com/rig/pkg/sdk/testsdk"
)

// mockKnowledgePlugin implements sdk.PluginInfo and sdk.KnowledgeHandler
type mockKnowledgePlugin struct {
}

func (p *mockKnowledgePlugin) Info() sdk.Info {
	return sdk.Info{
		ID:         "test-knowledge-plugin",
		APIVersion: "v1",
		Capabilities: []sdk.Capability{
			{Name: "knowledge", Version: "1.0.0"},
		},
	}
}

func (p *mockKnowledgePlugin) CreateTicketNote(ctx context.Context, req *apiv1.CreateTicketNoteRequest) (*apiv1.CreateTicketNoteResponse, error) {
	return &apiv1.CreateTicketNoteResponse{
		Path:    "/mock/path/" + req.Metadata.TicketId + ".md",
		Created: true,
	}, nil
}

func (p *mockKnowledgePlugin) UpdateDailyNote(ctx context.Context, req *apiv1.UpdateDailyNoteRequest) (*apiv1.UpdateDailyNoteResponse, error) {
	return &apiv1.UpdateDailyNoteResponse{Success: true}, nil
}

func (p *mockKnowledgePlugin) GetNotePath(ctx context.Context, req *apiv1.GetNotePathRequest) (*apiv1.GetNotePathResponse, error) {
	return &apiv1.GetNotePathResponse{Path: "/mock/path/" + req.TicketId + ".md"}, nil
}

func (p *mockKnowledgePlugin) GetDailyNotePath(ctx context.Context, req *apiv1.GetDailyNotePathRequest) (*apiv1.GetDailyNotePathResponse, error) {
	return &apiv1.GetDailyNotePathResponse{Path: "/mock/path/daily.md"}, nil
}

func TestKnowledgePluginIntegration(t *testing.T) {
	plugin := &mockKnowledgePlugin{}
	conn := testsdk.ServePlugin(t, plugin)

	// Verify the gRPC wiring by creating a client
	client := apiv1.NewKnowledgeServiceClient(conn)

	ctx := t.Context()

	t.Run("CreateTicketNote", func(t *testing.T) {
		resp, err := client.CreateTicketNote(ctx, &apiv1.CreateTicketNoteRequest{
			Metadata: &apiv1.NoteMetadata{TicketId: "PROJ-1"},
		})
		if err != nil {
			t.Fatalf("CreateTicketNote failed: %v", err)
		}
		if resp.Path != "/mock/path/PROJ-1.md" {
			t.Errorf("Unexpected path: %s", resp.Path)
		}
	})

	t.Run("GetDailyNotePath", func(t *testing.T) {
		resp, err := client.GetDailyNotePath(ctx, &apiv1.GetDailyNotePathRequest{})
		if err != nil {
			t.Fatalf("GetDailyNotePath failed: %v", err)
		}
		if resp.Path != "/mock/path/daily.md" {
			t.Errorf("Unexpected path: %s", resp.Path)
		}
	})
}
