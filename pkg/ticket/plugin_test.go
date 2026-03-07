package ticket

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/cockroachdb/errors"
	"google.golang.org/grpc"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
)

// mockPluginManager tracks calls to GetTicketClient and ReleasePlugin.
type mockPluginManager struct {
	client       apiv1.TicketServiceClient
	err          error
	releaseCalls int
	lastReleased string
}

func (m *mockPluginManager) GetTicketClient(ctx context.Context, name string) (apiv1.TicketServiceClient, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.client, nil
}

func (m *mockPluginManager) ReleasePlugin(name string) {
	m.releaseCalls++
	m.lastReleased = name
}

// mockTicketClient implements apiv1.TicketServiceClient.
type mockTicketClient struct {
	getTicketInfoFn      func(ctx context.Context, in *apiv1.GetTicketInfoRequest, opts ...grpc.CallOption) (*apiv1.GetTicketInfoResponse, error)
	updateTicketStatusFn func(ctx context.Context, in *apiv1.UpdateTicketStatusRequest, opts ...grpc.CallOption) (*apiv1.UpdateTicketStatusResponse, error)
	listTransitionsFn    func(ctx context.Context, in *apiv1.ListTransitionsRequest, opts ...grpc.CallOption) (*apiv1.ListTransitionsResponse, error)
}

// Ensure it implements the client interface.
var _ apiv1.TicketServiceClient = (*mockTicketClient)(nil)

func (m *mockTicketClient) GetTicketInfo(ctx context.Context, in *apiv1.GetTicketInfoRequest, opts ...grpc.CallOption) (*apiv1.GetTicketInfoResponse, error) {
	if m.getTicketInfoFn != nil {
		return m.getTicketInfoFn(ctx, in, opts...)
	}
	return nil, nil
}

func (m *mockTicketClient) UpdateTicketStatus(ctx context.Context, in *apiv1.UpdateTicketStatusRequest, opts ...grpc.CallOption) (*apiv1.UpdateTicketStatusResponse, error) {
	if m.updateTicketStatusFn != nil {
		return m.updateTicketStatusFn(ctx, in, opts...)
	}
	return nil, nil
}

func (m *mockTicketClient) ListTransitions(ctx context.Context, in *apiv1.ListTransitionsRequest, opts ...grpc.CallOption) (*apiv1.ListTransitionsResponse, error) {
	if m.listTransitionsFn != nil {
		return m.listTransitionsFn(ctx, in, opts...)
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

	const (
		// lowerSlack accounts for time elapsed between WithTimeout and this assertion.
		lowerSlack = 5 * time.Second
		// upperSlack accounts for minor clock drift or scheduling jitter.
		upperSlack = 1 * time.Second
	)

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Error("expected context to have a deadline")
		return
	}

	remaining := time.Until(deadline)
	if remaining < expectedTimeout-lowerSlack || remaining > expectedTimeout+upperSlack {
		t.Errorf("expected deadline around %v from now, remaining: %v", expectedTimeout, remaining)
	}
}

func TestPluginProvider_Lifecycle(t *testing.T) {
	t.Run("IsAvailable success", func(t *testing.T) {
		mockClient := &mockTicketClient{}
		mockMgr := &mockPluginManager{client: mockClient}
		provider := NewPluginProvider(mockMgr, "test-plugin")

		if !provider.IsAvailable(t.Context()) {
			t.Error("expected IsAvailable to be true")
		}
		assertReleaseCalled(t, mockMgr, 1)
		if mockMgr.lastReleased != "test-plugin" {
			t.Errorf("expected released plugin 'test-plugin', got %q", mockMgr.lastReleased)
		}
	})

	t.Run("GetTicketInfo success", func(t *testing.T) {
		mockClient := &mockTicketClient{
			getTicketInfoFn: func(ctx context.Context, in *apiv1.GetTicketInfoRequest, opts ...grpc.CallOption) (*apiv1.GetTicketInfoResponse, error) {
				assertTimeoutApplied(t, ctx, rpcLongTimeout)
				if in.TicketId != "RIG-1" {
					t.Errorf("expected ticket ID RIG-1, got %s", in.TicketId)
				}
				return &apiv1.GetTicketInfoResponse{
					Ticket: &apiv1.TicketInfo{
						Id:          "RIG-1",
						Title:       "Test Ticket",
						Type:        "Task",
						Status:      "Open",
						Priority:    "High",
						Description: "Description",
					},
				}, nil
			},
		}
		mockMgr := &mockPluginManager{client: mockClient}
		provider := NewPluginProvider(mockMgr, "test-plugin")

		info, err := provider.GetTicketInfo(t.Context(), "RIG-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if info.ID != "RIG-1" || info.Title != "Test Ticket" {
			t.Errorf("unexpected info: %+v", info)
		}
		assertReleaseCalled(t, mockMgr, 1)
	})

	t.Run("UpdateStatus success", func(t *testing.T) {
		mockClient := &mockTicketClient{
			updateTicketStatusFn: func(ctx context.Context, in *apiv1.UpdateTicketStatusRequest, opts ...grpc.CallOption) (*apiv1.UpdateTicketStatusResponse, error) {
				assertTimeoutApplied(t, ctx, rpcLongTimeout)
				if in.TicketId != "RIG-1" || in.Status != "Done" {
					t.Errorf("unexpected request: %+v", in)
				}
				return &apiv1.UpdateTicketStatusResponse{Success: true}, nil
			},
		}
		mockMgr := &mockPluginManager{client: mockClient}
		provider := NewPluginProvider(mockMgr, "test-plugin")

		err := provider.UpdateStatus(t.Context(), "RIG-1", "Done")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertReleaseCalled(t, mockMgr, 1)
	})
}

func TestPluginProvider_Robustness(t *testing.T) {
	t.Run("IsAvailable acquisition failure", func(t *testing.T) {
		mockMgr := &mockPluginManager{err: errors.New("failed to acquire plugin")}
		provider := NewPluginProvider(mockMgr, "test-plugin")

		if provider.IsAvailable(t.Context()) {
			t.Error("expected IsAvailable to be false on acquisition failure")
		}
		assertReleaseCalled(t, mockMgr, 0)
	})

	t.Run("GetTicketInfo acquisition failure", func(t *testing.T) {
		mockMgr := &mockPluginManager{err: errors.New("failed to acquire plugin")}
		provider := NewPluginProvider(mockMgr, "test-plugin")

		if _, err := provider.GetTicketInfo(t.Context(), "RIG-1"); err == nil {
			t.Error("expected error for GetTicketInfo acquisition failure")
		}
		assertReleaseCalled(t, mockMgr, 0)
	})

	t.Run("UpdateStatus acquisition failure", func(t *testing.T) {
		mockMgr := &mockPluginManager{err: errors.New("failed to acquire plugin")}
		provider := NewPluginProvider(mockMgr, "test-plugin")

		if err := provider.UpdateStatus(t.Context(), "RIG-1", "Done"); err == nil {
			t.Error("expected error for UpdateStatus acquisition failure")
		}
		assertReleaseCalled(t, mockMgr, 0)
	})

	t.Run("IsAvailable client nil", func(t *testing.T) {
		mockMgr := &mockPluginManager{client: nil}
		provider := NewPluginProvider(mockMgr, "test-plugin")

		if provider.IsAvailable(t.Context()) {
			t.Error("expected IsAvailable to be false when client is nil")
		}
		assertReleaseCalled(t, mockMgr, 0)
	})

	t.Run("RPC failure coverage", func(t *testing.T) {
		rpcErr := errors.New("gRPC error")
		mockClient := &mockTicketClient{
			getTicketInfoFn: func(ctx context.Context, in *apiv1.GetTicketInfoRequest, opts ...grpc.CallOption) (*apiv1.GetTicketInfoResponse, error) {
				return nil, rpcErr
			},
			updateTicketStatusFn: func(ctx context.Context, in *apiv1.UpdateTicketStatusRequest, opts ...grpc.CallOption) (*apiv1.UpdateTicketStatusResponse, error) {
				return nil, rpcErr
			},
		}
		mockMgr := &mockPluginManager{client: mockClient}
		provider := NewPluginProvider(mockMgr, "test-plugin")

		if _, err := provider.GetTicketInfo(t.Context(), "RIG-1"); err == nil {
			t.Error("expected error for GetTicketInfo RPC failure")
		}
		if err := provider.UpdateStatus(t.Context(), "RIG-1", "Done"); err == nil {
			t.Error("expected error for UpdateStatus RPC failure")
		}
		assertReleaseCalled(t, mockMgr, 2)
	})

	t.Run("GetTicketInfo nil response", func(t *testing.T) {
		mockClient := &mockTicketClient{} // returns (nil, nil) by default
		mockMgr := &mockPluginManager{client: mockClient}
		provider := NewPluginProvider(mockMgr, "test-plugin")

		_, err := provider.GetTicketInfo(t.Context(), "RIG-1")
		if err == nil {
			t.Fatal("expected error for GetTicketInfo nil response")
		}
		if !strings.Contains(err.Error(), "RIG-1") {
			t.Errorf("expected error to contain ticket ID, got: %v", err)
		}
		assertReleaseCalled(t, mockMgr, 1)
	})

	t.Run("UpdateStatus nil response", func(t *testing.T) {
		mockClient := &mockTicketClient{
			updateTicketStatusFn: func(ctx context.Context, in *apiv1.UpdateTicketStatusRequest, opts ...grpc.CallOption) (*apiv1.UpdateTicketStatusResponse, error) {
				return nil, nil
			},
		}
		mockMgr := &mockPluginManager{client: mockClient}
		provider := NewPluginProvider(mockMgr, "test-plugin")

		err := provider.UpdateStatus(t.Context(), "RIG-1", "Done")
		if err == nil {
			t.Fatal("expected error for UpdateStatus nil response")
		}
		if !strings.Contains(err.Error(), "RIG-1") {
			t.Errorf("expected error to contain ticket ID, got: %v", err)
		}
		assertReleaseCalled(t, mockMgr, 1)
	})

	t.Run("Nil ticket in response", func(t *testing.T) {
		mockClient := &mockTicketClient{
			getTicketInfoFn: func(ctx context.Context, in *apiv1.GetTicketInfoRequest, opts ...grpc.CallOption) (*apiv1.GetTicketInfoResponse, error) {
				return &apiv1.GetTicketInfoResponse{Ticket: nil}, nil
			},
		}
		mockMgr := &mockPluginManager{client: mockClient}
		provider := NewPluginProvider(mockMgr, "test-plugin")

		_, err := provider.GetTicketInfo(t.Context(), "RIG-1")
		if err == nil {
			t.Fatal("expected error for GetTicketInfo nil ticket in response")
		}
		if !strings.Contains(err.Error(), "RIG-1") {
			t.Errorf("expected error to contain ticket ID, got: %v", err)
		}
		assertReleaseCalled(t, mockMgr, 1)
	})

	t.Run("UpdateStatus success=false", func(t *testing.T) {
		mockClient := &mockTicketClient{
			updateTicketStatusFn: func(ctx context.Context, in *apiv1.UpdateTicketStatusRequest, opts ...grpc.CallOption) (*apiv1.UpdateTicketStatusResponse, error) {
				return &apiv1.UpdateTicketStatusResponse{Success: false}, nil
			},
		}
		mockMgr := &mockPluginManager{client: mockClient}
		provider := NewPluginProvider(mockMgr, "test-plugin")

		err := provider.UpdateStatus(t.Context(), "RIG-1", "Done")
		if err == nil {
			t.Fatal("expected error for UpdateStatus success=false")
		}
		if !strings.Contains(err.Error(), "RIG-1") {
			t.Errorf("expected error to contain ticket ID, got: %v", err)
		}
		if !strings.Contains(err.Error(), "Done") {
			t.Errorf("expected error to contain status, got: %v", err)
		}
		assertReleaseCalled(t, mockMgr, 1)
	})
}
