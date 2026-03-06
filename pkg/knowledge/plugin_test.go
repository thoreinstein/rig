package knowledge

import (
	"context"
	"testing"
	"time"

	"github.com/cockroachdb/errors"
	"google.golang.org/grpc"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
)

// mockPluginManager tracks calls to GetKnowledgeClient and ReleasePlugin.
type mockPluginManager struct {
	client       apiv1.KnowledgeServiceClient
	err          error
	releaseCalls int
	lastReleased string
}

func (m *mockPluginManager) GetKnowledgeClient(ctx context.Context, name string) (apiv1.KnowledgeServiceClient, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.client, nil
}

func (m *mockPluginManager) ReleasePlugin(name string) {
	m.releaseCalls++
	m.lastReleased = name
}

// mockKnowledgeClient implements apiv1.KnowledgeServiceClient.
type mockKnowledgeClient struct {
	createTicketNoteFn func(ctx context.Context, in *apiv1.CreateTicketNoteRequest, opts ...grpc.CallOption) (*apiv1.CreateTicketNoteResponse, error)
	updateDailyNoteFn  func(ctx context.Context, in *apiv1.UpdateDailyNoteRequest, opts ...grpc.CallOption) (*apiv1.UpdateDailyNoteResponse, error)
	getNotePathFn      func(ctx context.Context, in *apiv1.GetNotePathRequest, opts ...grpc.CallOption) (*apiv1.GetNotePathResponse, error)
	getDailyNotePathFn func(ctx context.Context, in *apiv1.GetDailyNotePathRequest, opts ...grpc.CallOption) (*apiv1.GetDailyNotePathResponse, error)
}

// Ensure it implements the client interface.
var _ apiv1.KnowledgeServiceClient = (*mockKnowledgeClient)(nil)

func (m *mockKnowledgeClient) CreateTicketNote(ctx context.Context, in *apiv1.CreateTicketNoteRequest, opts ...grpc.CallOption) (*apiv1.CreateTicketNoteResponse, error) {
	if m.createTicketNoteFn != nil {
		return m.createTicketNoteFn(ctx, in, opts...)
	}
	return nil, nil
}

func (m *mockKnowledgeClient) UpdateDailyNote(ctx context.Context, in *apiv1.UpdateDailyNoteRequest, opts ...grpc.CallOption) (*apiv1.UpdateDailyNoteResponse, error) {
	if m.updateDailyNoteFn != nil {
		return m.updateDailyNoteFn(ctx, in, opts...)
	}
	return nil, nil
}

func (m *mockKnowledgeClient) GetNotePath(ctx context.Context, in *apiv1.GetNotePathRequest, opts ...grpc.CallOption) (*apiv1.GetNotePathResponse, error) {
	if m.getNotePathFn != nil {
		return m.getNotePathFn(ctx, in, opts...)
	}
	return nil, nil
}

func (m *mockKnowledgeClient) GetDailyNotePath(ctx context.Context, in *apiv1.GetDailyNotePathRequest, opts ...grpc.CallOption) (*apiv1.GetDailyNotePathResponse, error) {
	if m.getDailyNotePathFn != nil {
		return m.getDailyNotePathFn(ctx, in, opts...)
	}
	return nil, nil
}

func assertReleaseCalled(t *testing.T, m *mockPluginManager, expected int) {
	t.Helper()
	if m.releaseCalls != expected {
		t.Errorf("expected ReleasePlugin to be called %d times, got %d", expected, m.releaseCalls)
	}
}

func assertTimeoutApplied(t *testing.T, ctx context.Context, expectedTimeout time.Duration) {
	t.Helper()
	deadline, ok := ctx.Deadline()
	if !ok {
		t.Error("expected context to have a deadline")
		return
	}

	remaining := time.Until(deadline)
	// Allow for a generous window (25s to 31s)
	if remaining < expectedTimeout-5*time.Second || remaining > expectedTimeout+1*time.Second {
		t.Errorf("expected deadline around %v from now, remaining: %v", expectedTimeout, remaining)
	}
}

func TestPluginProvider_Lifecycle(t *testing.T) {
	t.Run("CreateTicketNote success", func(t *testing.T) {
		mockClient := &mockKnowledgeClient{
			createTicketNoteFn: func(ctx context.Context, in *apiv1.CreateTicketNoteRequest, opts ...grpc.CallOption) (*apiv1.CreateTicketNoteResponse, error) {
				assertTimeoutApplied(t, ctx, rpcTimeout)
				if in.Metadata.TicketId != "RIG-1" {
					t.Errorf("expected ticket ID RIG-1, got %s", in.Metadata.TicketId)
				}
				return &apiv1.CreateTicketNoteResponse{
					Path:    "/notes/RIG-1.md",
					Created: true,
				}, nil
			},
		}
		mockMgr := &mockPluginManager{client: mockClient}
		provider := NewPluginProvider(mockMgr, "test-plugin")

		result, err := provider.CreateTicketNote(t.Context(), &NoteData{Ticket: "RIG-1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Path != "/notes/RIG-1.md" || !result.Created {
			t.Errorf("unexpected result: %+v", result)
		}
		assertReleaseCalled(t, mockMgr, 1)
		if mockMgr.lastReleased != "test-plugin" {
			t.Errorf("expected released plugin 'test-plugin', got %q", mockMgr.lastReleased)
		}
	})

	t.Run("UpdateDailyNote success", func(t *testing.T) {
		mockClient := &mockKnowledgeClient{
			updateDailyNoteFn: func(ctx context.Context, in *apiv1.UpdateDailyNoteRequest, opts ...grpc.CallOption) (*apiv1.UpdateDailyNoteResponse, error) {
				assertTimeoutApplied(t, ctx, rpcTimeout)
				return &apiv1.UpdateDailyNoteResponse{Success: true}, nil
			},
		}
		mockMgr := &mockPluginManager{client: mockClient}
		provider := NewPluginProvider(mockMgr, "test-plugin")

		err := provider.UpdateDailyNote(t.Context(), "RIG-1", "feature")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertReleaseCalled(t, mockMgr, 1)
	})

	t.Run("GetNotePath success", func(t *testing.T) {
		mockClient := &mockKnowledgeClient{
			getNotePathFn: func(ctx context.Context, in *apiv1.GetNotePathRequest, opts ...grpc.CallOption) (*apiv1.GetNotePathResponse, error) {
				assertTimeoutApplied(t, ctx, rpcTimeout)
				return &apiv1.GetNotePathResponse{Path: "/notes/RIG-1.md"}, nil
			},
		}
		mockMgr := &mockPluginManager{client: mockClient}
		provider := NewPluginProvider(mockMgr, "test-plugin")

		path, err := provider.GetNotePath(t.Context(), "feature", "RIG-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if path != "/notes/RIG-1.md" {
			t.Errorf("expected path /notes/RIG-1.md, got %q", path)
		}
		assertReleaseCalled(t, mockMgr, 1)
	})

	t.Run("GetDailyNotePath success", func(t *testing.T) {
		mockClient := &mockKnowledgeClient{
			getDailyNotePathFn: func(ctx context.Context, in *apiv1.GetDailyNotePathRequest, opts ...grpc.CallOption) (*apiv1.GetDailyNotePathResponse, error) {
				assertTimeoutApplied(t, ctx, rpcTimeout)
				return &apiv1.GetDailyNotePathResponse{Path: "/notes/daily.md"}, nil
			},
		}
		mockMgr := &mockPluginManager{client: mockClient}
		provider := NewPluginProvider(mockMgr, "test-plugin")

		path, err := provider.GetDailyNotePath(t.Context())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if path != "/notes/daily.md" {
			t.Errorf("expected path /notes/daily.md, got %q", path)
		}
		assertReleaseCalled(t, mockMgr, 1)
	})
}

func TestPluginProvider_Robustness(t *testing.T) {
	t.Run("Acquisition failure coverage", func(t *testing.T) {
		expectedErr := errors.New("failed to acquire plugin")
		mockMgr := &mockPluginManager{err: expectedErr}
		provider := NewPluginProvider(mockMgr, "test-plugin")

		if _, err := provider.CreateTicketNote(t.Context(), &NoteData{}); err == nil {
			t.Error("expected error for CreateTicketNote acquisition failure")
		}
		if err := provider.UpdateDailyNote(t.Context(), "T", "t"); err == nil {
			t.Error("expected error for UpdateDailyNote acquisition failure")
		}
		if _, err := provider.GetNotePath(t.Context(), "T", "t"); err == nil {
			t.Error("expected error for GetNotePath acquisition failure")
		}
		if _, err := provider.GetDailyNotePath(t.Context()); err == nil {
			t.Error("expected error for GetDailyNotePath acquisition failure")
		}
		// Release should NOT be called if client acquisition fails.
		assertReleaseCalled(t, mockMgr, 0)
	})

	t.Run("RPC failure coverage", func(t *testing.T) {
		rpcErr := errors.New("gRPC error")
		mockClient := &mockKnowledgeClient{
			createTicketNoteFn: func(ctx context.Context, in *apiv1.CreateTicketNoteRequest, opts ...grpc.CallOption) (*apiv1.CreateTicketNoteResponse, error) {
				return nil, rpcErr
			},
			updateDailyNoteFn: func(ctx context.Context, in *apiv1.UpdateDailyNoteRequest, opts ...grpc.CallOption) (*apiv1.UpdateDailyNoteResponse, error) {
				return nil, rpcErr
			},
			getNotePathFn: func(ctx context.Context, in *apiv1.GetNotePathRequest, opts ...grpc.CallOption) (*apiv1.GetNotePathResponse, error) {
				return nil, rpcErr
			},
			getDailyNotePathFn: func(ctx context.Context, in *apiv1.GetDailyNotePathRequest, opts ...grpc.CallOption) (*apiv1.GetDailyNotePathResponse, error) {
				return nil, rpcErr
			},
		}
		mockMgr := &mockPluginManager{client: mockClient}
		provider := NewPluginProvider(mockMgr, "test-plugin")

		if _, err := provider.CreateTicketNote(t.Context(), &NoteData{}); err == nil {
			t.Error("expected error for CreateTicketNote")
		}
		if err := provider.UpdateDailyNote(t.Context(), "T", "t"); err == nil {
			t.Error("expected error for UpdateDailyNote")
		}
		if _, err := provider.GetNotePath(t.Context(), "T", "t"); err == nil {
			t.Error("expected error for GetNotePath")
		}
		if _, err := provider.GetDailyNotePath(t.Context()); err == nil {
			t.Error("expected error for GetDailyNotePath")
		}
		assertReleaseCalled(t, mockMgr, 4)
	})

	t.Run("Nil response error coverage", func(t *testing.T) {
		mockClient := &mockKnowledgeClient{} // returns (nil, nil) by default
		mockMgr := &mockPluginManager{client: mockClient}
		provider := NewPluginProvider(mockMgr, "test-plugin")

		if _, err := provider.CreateTicketNote(t.Context(), &NoteData{}); err == nil {
			t.Error("expected error for CreateTicketNote nil response")
		}
		if err := provider.UpdateDailyNote(t.Context(), "T", "t"); err == nil {
			t.Error("expected error for UpdateDailyNote nil response")
		}
		if _, err := provider.GetNotePath(t.Context(), "T", "t"); err == nil {
			t.Error("expected error for GetNotePath nil response")
		}
		if _, err := provider.GetDailyNotePath(t.Context()); err == nil {
			t.Error("expected error for GetDailyNotePath nil response")
		}
		assertReleaseCalled(t, mockMgr, 4)
	})

	t.Run("UpdateDailyNote success=false", func(t *testing.T) {
		mockClient := &mockKnowledgeClient{
			updateDailyNoteFn: func(ctx context.Context, in *apiv1.UpdateDailyNoteRequest, opts ...grpc.CallOption) (*apiv1.UpdateDailyNoteResponse, error) {
				return &apiv1.UpdateDailyNoteResponse{Success: false}, nil
			},
		}
		mockMgr := &mockPluginManager{client: mockClient}
		provider := NewPluginProvider(mockMgr, "test-plugin")

		err := provider.UpdateDailyNote(t.Context(), "RIG-1", "feature")
		expectedMsg := `plugin "test-plugin" failed to update daily note for RIG-1`
		if err == nil || err.Error() != expectedMsg {
			t.Errorf("expected error %q, got %v", expectedMsg, err)
		}
		assertReleaseCalled(t, mockMgr, 1)
	})
}
