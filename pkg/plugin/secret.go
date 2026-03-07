package plugin

import (
	"context"
	"strings"
	"sync"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
	rigerrors "thoreinstein.com/rig/pkg/errors"
)

// ErrSecretNotFound indicates a secret key does not exist in the plugin's
// configuration. Callers can test for this with errors.Is to distinguish
// "not found" from host-side resolution failures (e.g. keychain errors).
var ErrSecretNotFound = rigerrors.New("secret not found")

// SecretResolver resolves a secret for a given plugin. The pluginName and
// secretKey are validated and passed separately to prevent dot-injection
// attacks where a crafted key could escape the plugin's secret scope.
type SecretResolver func(pluginName, secretKey string) (string, error)

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

	// Validate the requested key to prevent dot-injection, null bytes, and
	// path traversal. A well-formed secret key is a simple identifier.
	if req.Key == "" ||
		strings.ContainsAny(req.Key, ".\x00/\\") {
		return nil, status.Errorf(codes.InvalidArgument, "invalid secret key")
	}

	val, err := s.resolver(pluginName, req.Key)
	if err != nil {
		if rigerrors.Is(err, ErrSecretNotFound) {
			return nil, status.Errorf(codes.NotFound, "secret not available")
		}
		return nil, status.Errorf(codes.Internal, "secret resolution failed")
	}

	return &apiv1.GetSecretResponse{
		Secret: &apiv1.SecretValue{
			Value: val,
		},
	}, nil
}

// GetSecrets resolves multiple secret keys in a single request.
// Missing keys are omitted from the response map (partial-failure semantics).
func (s *HostSecretProxy) GetSecrets(ctx context.Context, req *apiv1.GetSecretsRequest) (*apiv1.GetSecretsResponse, error) {
	s.mu.RLock()
	pluginName, ok := s.tokens[req.Token]
	s.mu.RUnlock()

	if !ok || pluginName == "" {
		return nil, status.Errorf(codes.Unauthenticated, "invalid secret token")
	}

	secrets := make(map[string]*apiv1.SecretValue, len(req.Keys))
	for _, key := range req.Keys {
		if key == "" || strings.ContainsAny(key, ".\x00/\\") {
			continue // skip invalid keys silently per partial-failure semantics
		}

		val, err := s.resolver(pluginName, key)
		if err != nil {
			continue // omit missing/failed keys
		}
		secrets[key] = &apiv1.SecretValue{Value: val}
	}

	return &apiv1.GetSecretsResponse{Secrets: secrets}, nil
}

// RefreshToken rotates a plugin's session token.
func (s *HostSecretProxy) RefreshToken(_ context.Context, req *apiv1.RefreshTokenRequest) (*apiv1.RefreshTokenResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	pluginName, ok := s.tokens[req.CurrentToken]
	if !ok || pluginName == "" {
		return nil, status.Errorf(codes.Unauthenticated, "invalid secret token")
	}

	u, err := uuid.NewRandom()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to generate new token")
	}
	newToken := u.String()

	delete(s.tokens, req.CurrentToken)
	s.tokens[newToken] = pluginName

	return &apiv1.RefreshTokenResponse{
		NewToken: newToken,
	}, nil
}
