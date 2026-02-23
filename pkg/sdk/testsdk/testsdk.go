package testsdk

import (
	"context"
	"net"
	"sync"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
	"thoreinstein.com/rig/pkg/sdk"
)

// MockHost provides a simulated Rig host for testing plugins.
type MockHost struct {
	lis *bufconn.Listener
	srv *grpc.Server
	UI  *MockUIServer
	t   testing.TB
}

// MockUIServer is a stubbable implementation of the UIService for testing.
type MockUIServer struct {
	apiv1.UnimplementedUIServiceServer
	mu sync.Mutex

	PromptFunc         func(*apiv1.PromptRequest) (*apiv1.PromptResponse, error)
	ConfirmFunc        func(*apiv1.ConfirmRequest) (*apiv1.ConfirmResponse, error)
	SelectFunc         func(*apiv1.SelectRequest) (*apiv1.SelectResponse, error)
	UpdateProgressFunc func(*apiv1.UpdateProgressRequest) (*apiv1.UpdateProgressResponse, error)

	PromptCalls         []*apiv1.PromptRequest
	ConfirmCalls        []*apiv1.ConfirmRequest
	SelectCalls         []*apiv1.SelectRequest
	UpdateProgressCalls []*apiv1.UpdateProgressRequest
}

func (m *MockUIServer) Prompt(ctx context.Context, req *apiv1.PromptRequest) (*apiv1.PromptResponse, error) {
	m.mu.Lock()
	m.PromptCalls = append(m.PromptCalls, req)
	m.mu.Unlock()

	if m.PromptFunc != nil {
		return m.PromptFunc(req)
	}
	return &apiv1.PromptResponse{Value: "mock-response"}, nil
}

func (m *MockUIServer) Confirm(ctx context.Context, req *apiv1.ConfirmRequest) (*apiv1.ConfirmResponse, error) {
	m.mu.Lock()
	m.ConfirmCalls = append(m.ConfirmCalls, req)
	m.mu.Unlock()

	if m.ConfirmFunc != nil {
		return m.ConfirmFunc(req)
	}
	return &apiv1.ConfirmResponse{Confirmed: true}, nil
}

func (m *MockUIServer) Select(ctx context.Context, req *apiv1.SelectRequest) (*apiv1.SelectResponse, error) {
	m.mu.Lock()
	m.SelectCalls = append(m.SelectCalls, req)
	m.mu.Unlock()

	if m.SelectFunc != nil {
		return m.SelectFunc(req)
	}
	return &apiv1.SelectResponse{SelectedIndices: []uint32{0}}, nil
}

func (m *MockUIServer) UpdateProgress(ctx context.Context, req *apiv1.UpdateProgressRequest) (*apiv1.UpdateProgressResponse, error) {
	m.mu.Lock()
	m.UpdateProgressCalls = append(m.UpdateProgressCalls, req)
	m.mu.Unlock()

	if m.UpdateProgressFunc != nil {
		return m.UpdateProgressFunc(req)
	}
	return &apiv1.UpdateProgressResponse{}, nil
}

// StartMockHost starts a mock Rig host on a bufconn listener.
func StartMockHost(t testing.TB) *MockHost {
	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer()
	ui := &MockUIServer{}
	apiv1.RegisterUIServiceServer(srv, ui)

	h := &MockHost{
		lis: lis,
		srv: srv,
		UI:  ui,
		t:   t,
	}

	go func() {
		if err := srv.Serve(lis); err != nil && err != grpc.ErrServerStopped {
			t.Errorf("Mock host Serve error: %v", err)
		}
	}()

	t.Cleanup(func() {
		h.Stop()
	})

	return h
}

// Stop stops the mock host.
func (h *MockHost) Stop() {
	h.srv.GracefulStop()
	h.lis.Close()
}

// DialOption returns a gRPC dial option for connecting to the mock host.
func (h *MockHost) DialOption() grpc.DialOption {
	return grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return h.lis.Dial()
	})
}

// NewTestUI creates an sdk.UI client connected to this mock host.
func (h *MockHost) NewTestUI() *sdk.UI {
	return sdk.NewUI(
		sdk.WithHostEndpoint("passthrough://bufnet"), // endpoint doesn't matter for bufconn but must be non-empty
		sdk.WithDialOptions(h.DialOption()),
	)
}

// ServePlugin starts a plugin server on a bufconn listener and returns a client connection to it.
func ServePlugin(t testing.TB, p sdk.PluginInfo) *grpc.ClientConn {
	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer()
	sdk.RegisterServices(srv, p)

	go func() {
		if err := srv.Serve(lis); err != nil && err != grpc.ErrServerStopped {
			t.Errorf("Plugin host Serve error: %v", err)
		}
	}()

	t.Cleanup(func() {
		srv.Stop()
		lis.Close()
	})

	conn, err := grpc.NewClient("passthrough://bufnet",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
	)
	if err != nil {
		t.Fatalf("failed to dial plugin: %v", err)
	}

	return conn
}
