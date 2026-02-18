package plugin

import (
	"context"

	"google.golang.org/grpc"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
)

// MockPluginServiceClient is a mock implementation of apiv1.PluginServiceClient.
type MockPluginServiceClient struct {
	HandshakeFunc func(ctx context.Context, in *apiv1.HandshakeRequest, opts ...grpc.CallOption) (*apiv1.HandshakeResponse, error)
	InteractFunc  func(ctx context.Context, opts ...grpc.CallOption) (grpc.BidiStreamingClient[apiv1.InteractRequest, apiv1.InteractResponse], error)
}

func (m *MockPluginServiceClient) Handshake(ctx context.Context, in *apiv1.HandshakeRequest, opts ...grpc.CallOption) (*apiv1.HandshakeResponse, error) {
	if m.HandshakeFunc != nil {
		return m.HandshakeFunc(ctx, in, opts...)
	}
	return &apiv1.HandshakeResponse{}, nil
}

func (m *MockPluginServiceClient) Interact(ctx context.Context, opts ...grpc.CallOption) (grpc.BidiStreamingClient[apiv1.InteractRequest, apiv1.InteractResponse], error) {
	if m.InteractFunc != nil {
		return m.InteractFunc(ctx, opts...)
	}
	return nil, nil
}
