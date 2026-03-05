package plugin

import (
	"context"
	"os"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
)

// secretAllowList defines the set of keys that plugins are allowed to request.
var secretAllowList = map[string]bool{
	"JIRA_TOKEN":    true,
	"JIRA_EMAIL":    true,
	"JIRA_BASE_URL": true,
	"BEADS_TOKEN":   true,
}

// HostSecretProxy implements apiv1.SecretServiceServer.
type HostSecretProxy struct {
	apiv1.UnimplementedSecretServiceServer
}

// NewHostSecretProxy creates a new HostSecretProxy.
func NewHostSecretProxy() *HostSecretProxy {
	return &HostSecretProxy{}
}

// GetSecret resolves a secret key from the host's environment.
func (s *HostSecretProxy) GetSecret(ctx context.Context, req *apiv1.GetSecretRequest) (*apiv1.GetSecretResponse, error) {
	if !secretAllowList[req.Key] {
		return nil, status.Errorf(codes.PermissionDenied, "secret not available")
	}

	val, ok := os.LookupEnv(req.Key)
	if !ok {
		return nil, status.Errorf(codes.PermissionDenied, "secret not available")
	}

	return &apiv1.GetSecretResponse{Value: val}, nil
}
