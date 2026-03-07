package sdk

import (
	"context"
	"os"
	"strings"
	"sync"

	"github.com/cockroachdb/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
)

// Secret is a high-level client for interacting with the Rig host's SecretService.
type Secret struct {
	mu       sync.Mutex
	endpoint string
	token    string
	conn     *grpc.ClientConn
	client   apiv1.SecretServiceClient
	dialOpts []grpc.DialOption
}

// SecretOption is a functional option for configuring the Secret client.
type SecretOption func(*Secret)

// WithSecretHostEndpoint overrides the host's UDS endpoint.
func WithSecretHostEndpoint(endpoint string) SecretOption {
	return func(s *Secret) {
		s.endpoint = endpoint
	}
}

// WithSecretToken overrides the host's secret token.
func WithSecretToken(token string) SecretOption {
	return func(s *Secret) {
		s.token = token
	}
}

// NewSecret creates a new Secret client.
// It reads the host's endpoint from the RIG_HOST_ENDPOINT environment variable and
// the secret token from RIG_HOST_SECRET_TOKEN by default.
func NewSecret(opts ...SecretOption) *Secret {
	s := &Secret{
		endpoint: os.Getenv("RIG_HOST_ENDPOINT"),
		token:    os.Getenv("RIG_HOST_SECRET_TOKEN"),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Close closes the underlying gRPC connection.
func (s *Secret) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.conn != nil {
		err := s.conn.Close()
		s.conn = nil
		s.client = nil
		return err
	}
	return nil
}

func (s *Secret) connect() (apiv1.SecretServiceClient, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.client != nil {
		return s.client, nil
	}

	if s.endpoint == "" {
		return nil, ErrNoEndpoint
	}

	opts := append([]grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}, s.dialOpts...)

	endpoint := s.endpoint
	// If no scheme is provided, and it looks like a path, default to unix:// for UDS.
	if !strings.Contains(endpoint, "://") && (strings.HasPrefix(endpoint, "/") || strings.HasPrefix(endpoint, ".")) {
		endpoint = "unix://" + endpoint
	}

	// Reject non-UDS endpoints when using insecure credentials to prevent
	// transmitting secrets over the network in plaintext.
	if !strings.HasPrefix(endpoint, "unix://") {
		return nil, errors.New("sdk: secret service requires a unix:// endpoint for secure transport")
	}

	conn, err := grpc.NewClient(endpoint, opts...)
	if err != nil {
		return nil, err
	}

	s.conn = conn
	s.client = apiv1.NewSecretServiceClient(conn)
	return s.client, nil
}

// GetSecret retrieves a secret value by key from the host.
func (s *Secret) GetSecret(ctx context.Context, key string) (string, error) {
	client, err := s.connect()
	if err != nil {
		return "", err
	}

	resp, err := client.GetSecret(ctx, &apiv1.GetSecretRequest{
		Key:   key,
		Token: s.token,
	})
	if err != nil {
		return "", mapError(err)
	}
	if resp.Secret == nil {
		return "", errors.New("secret not available")
	}
	return resp.Secret.Value, nil
}

// GetSecrets retrieves multiple secret values by key from the host in a single request.
// Missing or inaccessible keys are omitted from the returned map.
func (s *Secret) GetSecrets(ctx context.Context, keys []string) (map[string]string, error) {
	client, err := s.connect()
	if err != nil {
		return nil, err
	}

	resp, err := client.GetSecrets(ctx, &apiv1.GetSecretsRequest{
		Keys:  keys,
		Token: s.token,
	})
	if err != nil {
		return nil, mapError(err)
	}

	result := make(map[string]string, len(resp.Secrets))
	for k, sv := range resp.Secrets {
		if sv != nil {
			result[k] = sv.Value
		}
	}
	return result, nil
}

// RefreshToken rotates the current session token and updates the client's
// internal token for subsequent requests.
func (s *Secret) RefreshToken(ctx context.Context) (string, error) {
	s.mu.Lock()
	currentToken := s.token
	s.mu.Unlock()

	client, err := s.connect()
	if err != nil {
		return "", err
	}

	resp, err := client.RefreshToken(ctx, &apiv1.RefreshTokenRequest{
		CurrentToken: currentToken,
	})
	if err != nil {
		return "", mapError(err)
	}

	s.mu.Lock()
	if s.token == currentToken {
		s.token = resp.NewToken
	}
	s.mu.Unlock()

	return resp.NewToken, nil
}

// GetSecret is a convenience helper that uses a default Secret client.
// It creates a new connection per call, so use the Secret type for multiple calls.
func GetSecret(ctx context.Context, key string) (string, error) {
	s := NewSecret()
	defer s.Close()
	return s.GetSecret(ctx, key)
}
