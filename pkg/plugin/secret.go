package plugin

import (
	"context"
	"sync"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
)

// SecretResolver is a function that resolves a configuration key to its value.
type SecretResolver func(key string) (string, error)

// HostSecretProxy implements apiv1.SecretServiceServer.
type HostSecretProxy struct {
	apiv1.UnimplementedSecretServiceServer
	mu       sync.RWMutex
	tokens   map[string]string // token -> plugin name
	resolver SecretResolver
}

// NewHostSecretProxy creates a new HostSecretProxy.
func NewHostSecretProxy(resolver SecretResolver) *HostSecretProxy {
	return &HostSecretProxy{
		tokens:   make(map[string]string),
		resolver: resolver,
	}
}

// RegisterPlugin registers a secret token for a specific plugin.
func (s *HostSecretProxy) RegisterPlugin(token, name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens[token] = name
}

// UnregisterPlugin removes a secret token.
func (s *HostSecretProxy) UnregisterPlugin(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.tokens, token)
}

// GetSecret resolves a secret key from the host's configuration.
func (s *HostSecretProxy) GetSecret(ctx context.Context, req *apiv1.GetSecretRequest) (*apiv1.GetSecretResponse, error) {
	s.mu.RLock()
	pluginName, ok := s.tokens[req.Token]
	s.mu.RUnlock()

	if !ok || pluginName == "" {
		return nil, status.Errorf(codes.Unauthenticated, "invalid secret token")
	}

	// Scoping Logic:
	// A plugin 'p' can only access secrets that are either:
	// 1. In a global 'secrets' section (not yet implemented, but could be).
	// 2. In its own specific config: 'plugins.p.secrets.K'.
	// We'll start with (2).

	// The request key 'K' is interpreted as a relative key within the plugin's secrets.
	// So if plugin 'jira' asks for 'token', we look up 'plugins.jira.secrets.token'.
	// This ensures plugins can't peek into each other's configuration.
	scopedKey := "plugins." + pluginName + ".secrets." + req.Key

	val, err := s.resolver(scopedKey)
	if err != nil {
		// Anti-enumeration: return the same error for missing or denied keys.
		return nil, status.Errorf(codes.NotFound, "secret not available")
	}

	return &apiv1.GetSecretResponse{Value: val}, nil
}
