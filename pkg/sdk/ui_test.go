package sdk

import (
	"context"
	"net"
	"path/filepath"
	"testing"

	"google.golang.org/grpc"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
)

type mockUIServer struct {
	apiv1.UnimplementedUIServiceServer
	promptFunc func(*apiv1.PromptRequest) (*apiv1.PromptResponse, error)
}

func (m *mockUIServer) Prompt(ctx context.Context, req *apiv1.PromptRequest) (*apiv1.PromptResponse, error) {
	if m.promptFunc != nil {
		return m.promptFunc(req)
	}
	return &apiv1.PromptResponse{Value: "default"}, nil
}

func (m *mockUIServer) Confirm(ctx context.Context, req *apiv1.ConfirmRequest) (*apiv1.ConfirmResponse, error) {
	return &apiv1.ConfirmResponse{Confirmed: true}, nil
}

func (m *mockUIServer) Select(ctx context.Context, req *apiv1.SelectRequest) (*apiv1.SelectResponse, error) {
	return &apiv1.SelectResponse{SelectedIndices: []uint32{0}}, nil
}

func (m *mockUIServer) UpdateProgress(ctx context.Context, req *apiv1.UpdateProgressRequest) (*apiv1.UpdateProgressResponse, error) {
	return &apiv1.UpdateProgressResponse{}, nil
}

func TestUI_Prompt(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "host.sock")

	srv := grpc.NewServer()
	mock := &mockUIServer{
		promptFunc: func(req *apiv1.PromptRequest) (*apiv1.PromptResponse, error) {
			if req.Label == "name?" {
				return &apiv1.PromptResponse{Value: "Bob"}, nil
			}
			return &apiv1.PromptResponse{Value: "unknown"}, nil
		},
	}
	apiv1.RegisterUIServiceServer(srv, mock)

	lis, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	go func() {
		_ = srv.Serve(lis)
	}()
	defer srv.Stop()

	ui := NewUI(WithHostEndpoint(socketPath))
	defer ui.Close()

	val, err := ui.Prompt(t.Context(), "name?")
	if err != nil {
		t.Fatalf("Prompt failed: %v", err)
	}
	if val != "Bob" {
		t.Errorf("got prompt value %q, want %q", val, "Bob")
	}
}

func TestUI_LazyConnect(t *testing.T) {
	ui := NewUI(WithHostEndpoint("/non/existent/path"))
	// Should not fail yet
	_, err := ui.Prompt(t.Context(), "test")
	if err == nil {
		t.Error("expected error when prompting with non-existent path")
	}
}

func TestUI_EnvironmentResolution(t *testing.T) {
	t.Setenv("RIG_HOST_ENDPOINT", "/tmp/env-host.sock")

	ui := NewUI()
	if ui.endpoint != "/tmp/env-host.sock" {
		t.Errorf("got endpoint %q, want %q", ui.endpoint, "/tmp/env-host.sock")
	}
}
