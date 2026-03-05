package sdk

import (
	"context"
	"os"
	"strings"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
)

// Secret is a high-level client for interacting with the Rig host's SecretService.
type Secret struct {
	mu       sync.Mutex
	endpoint string
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

// NewSecret creates a new Secret client.
// It reads the host's endpoint from the RIG_HOST_ENDPOINT environment variable by default.
func NewSecret(opts ...SecretOption) *Secret {
	s := &Secret{
		endpoint: os.Getenv("RIG_HOST_ENDPOINT"),
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

	resp, err := client.GetSecret(ctx, &apiv1.GetSecretRequest{Key: key})
	if err != nil {
		return "", mapError(err)
	}
	return resp.Value, nil
}

// GetSecret is a convenience helper that uses a default Secret client.
// It creates a new connection per call, so use the Secret type for multiple calls.
func GetSecret(ctx context.Context, key string) (string, error) {
	s := NewSecret()
	defer s.Close()
	return s.GetSecret(ctx, key)
}
