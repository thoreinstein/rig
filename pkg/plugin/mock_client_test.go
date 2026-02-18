package plugin

import (
	"context"

	"google.golang.org/grpc"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
)

// MockPluginServiceClient is a mock implementation of apiv1.PluginServiceClient.
type MockPluginServiceClient struct {
	HandshakeFunc func(ctx context.Context, in *apiv1.HandshakeRequest, opts ...grpc.CallOption) (*apiv1.HandshakeResponse, error)
}

func (m *MockPluginServiceClient) Handshake(ctx context.Context, in *apiv1.HandshakeRequest, opts ...grpc.CallOption) (*apiv1.HandshakeResponse, error) {
	if m.HandshakeFunc != nil {
		return m.HandshakeFunc(ctx, in, opts...)
	}
	return &apiv1.HandshakeResponse{}, nil
}
