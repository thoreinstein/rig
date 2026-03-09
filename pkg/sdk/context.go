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

// ContextInfo holds environment context returned by the host.
type ContextInfo struct {
	ProjectRoot  string
	WorktreeRoot string
	TicketID     string
	Metadata     map[string]any
}

// Context is a high-level client for interacting with the Rig host's ContextService.
type Context struct {
	mu       sync.Mutex
	endpoint string
	conn     *grpc.ClientConn
	client   apiv1.ContextServiceClient
	dialOpts []grpc.DialOption
}

// ContextOption is a functional option for configuring the Context client.
type ContextOption func(*Context)

// WithContextHostEndpoint overrides the host's UDS endpoint.
func WithContextHostEndpoint(endpoint string) ContextOption {
	return func(c *Context) {
		c.endpoint = endpoint
	}
}

// NewContext creates a new Context client.
// It reads the host's endpoint from the RIG_HOST_ENDPOINT environment variable by default.
func NewContext(opts ...ContextOption) *Context {
	c := &Context{
		endpoint: os.Getenv("RIG_HOST_ENDPOINT"),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Close closes the underlying gRPC connection.
func (c *Context) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		c.client = nil
		return err
	}
	return nil
}

// connect returns the gRPC client.
func (c *Context) connect() (apiv1.ContextServiceClient, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.client != nil {
		return c.client, nil
	}

	if c.endpoint == "" {
		return nil, ErrNoEndpoint
	}

	opts := append([]grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}, c.dialOpts...)

	endpoint := c.endpoint
	if !strings.Contains(endpoint, "://") && (strings.HasPrefix(endpoint, "/") || strings.HasPrefix(endpoint, ".")) {
		endpoint = "unix://" + endpoint
	}

	if !strings.HasPrefix(endpoint, "unix://") {
		return nil, errors.New("sdk: context service requires a unix:// endpoint for secure transport")
	}

	conn, err := grpc.NewClient(endpoint, opts...)
	if err != nil {
		return nil, err
	}

	c.conn = conn
	c.client = apiv1.NewContextServiceClient(conn)
	return c.client, nil
}

// GetContext retrieves the current environment context from the host.
func (c *Context) GetContext(ctx context.Context) (*ContextInfo, error) {
	client, err := c.connect()
	if err != nil {
		return nil, err
	}

	resp, err := client.GetContext(ctx, &apiv1.GetContextRequest{})
	if err != nil {
		return nil, mapError(err)
	}

	info := &ContextInfo{
		ProjectRoot:  resp.ProjectRoot,
		WorktreeRoot: resp.WorktreeRoot,
		TicketID:     resp.TicketId,
	}
	if resp.Metadata != nil {
		info.Metadata = resp.Metadata.AsMap()
	}
	return info, nil
}

// GetContext is a convenience helper that uses a default Context client.
// It creates a new connection per call, so use the Context type for multiple calls.
func GetContext(ctx context.Context) (*ContextInfo, error) {
	c := NewContext()
	defer c.Close()
	return c.GetContext(ctx)
}
