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
	"JIRA_TOKEN":  true,
	"JIRA_EMAIL":  true,
	"BEADS_TOKEN": true,
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
		return nil, status.Errorf(codes.PermissionDenied, "access to secret key %q is not allowed", req.Key)
	}

	// Resolve from environment.
	// Since Rig host already hydrated its config, these might be present in the environment
	// if they were passed as RIG_AI_* or if the loader injected them (though loader usually doesn't inject back to env).
	// NOTE: For full keychain support, we might need to pass the hydrated config to this proxy.
	val, ok := os.LookupEnv(req.Key)
	if !ok {
		return nil, status.Errorf(codes.NotFound, "secret key %q not found", req.Key)
	}

	return &apiv1.GetSecretResponse{Value: val}, nil
}
