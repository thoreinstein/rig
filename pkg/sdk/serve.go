package sdk

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
)

type serverConfig struct {
	ctx          context.Context
	logger       *slog.Logger
	grpcOpts     []grpc.ServerOption
	endpoint     string
	stopOnSignal bool
}

func defaultConfig() *serverConfig {
	return &serverConfig{
		ctx:          context.Background(),
		logger:       slog.Default(),
		stopOnSignal: true,
	}
}

// Option is a functional option for configuring the SDK server.
type Option func(*serverConfig)

// WithContext sets the parent context for the server.
func WithContext(ctx context.Context) Option {
	return func(c *serverConfig) {
		c.ctx = ctx
	}
}

// WithLogger sets the logger used by the SDK.
func WithLogger(logger *slog.Logger) Option {
	return func(c *serverConfig) {
		c.logger = logger
	}
}

// WithGRPCServerOptions adds custom gRPC server options.
func WithGRPCServerOptions(opts ...grpc.ServerOption) Option {
	return func(c *serverConfig) {
		c.grpcOpts = append(c.grpcOpts, opts...)
	}
}

// WithEndpoint overrides the UDS endpoint.
// If not provided, the SDK reads from the RIG_PLUGIN_ENDPOINT environment variable.
func WithEndpoint(endpoint string) Option {
	return func(c *serverConfig) {
		c.endpoint = endpoint
	}
}

// Serve initializes and starts the Rig plugin gRPC server over UDS.
// It handles socket lifecycle, service registration, and graceful shutdown.
func Serve(p PluginInfo, opts ...Option) error {
	config := defaultConfig()
	for _, opt := range opts {
		opt(config)
	}

	endpoint := config.endpoint
	if endpoint == "" {
		endpoint = os.Getenv("RIG_PLUGIN_ENDPOINT")
	}

	if endpoint == "" {
		config.logger.Error("RIG_PLUGIN_ENDPOINT not set")
		return ErrNoEndpoint
	}

	// Handle graceful shutdown via context/signals
	ctx, cancel := context.WithCancel(config.ctx)
	defer cancel()

	if config.stopOnSignal {
		sigCtx, sigCancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
		defer sigCancel()
		ctx = sigCtx
	}

	// Remove stale socket
	if err := os.Remove(endpoint); err != nil && !os.IsNotExist(err) {
		return err
	}

	lis, err := net.Listen("unix", endpoint)
	if err != nil {
		return err
	}
	defer lis.Close()
	defer os.Remove(endpoint)

	srv := grpc.NewServer(config.grpcOpts...)
	RegisterServices(srv, p)

	// Graceful shutdown goroutine
	go func() {
		<-ctx.Done()
		config.logger.Info("shutting down plugin server...")
		srv.GracefulStop()
	}()

	config.logger.Info("plugin server started", "endpoint", endpoint)
	if err := srv.Serve(lis); err != nil && err != grpc.ErrServerStopped {
		return err
	}

	return nil
}

// RegisterServices registers all services implemented by the plugin to the gRPC registrar.
// This is exported for use by testsdk.
func RegisterServices(reg grpc.ServiceRegistrar, p PluginInfo) {
	apiv1.RegisterPluginServiceServer(reg, newPluginBridge(p))

	if h, ok := p.(AssistantHandler); ok {
		apiv1.RegisterAssistantServiceServer(reg, newAssistantBridge(h))
	}

	if h, ok := p.(CommandHandler); ok {
		apiv1.RegisterCommandServiceServer(reg, newCommandBridge(h))
	}

	if h, ok := p.(NodeHandler); ok {
		apiv1.RegisterNodeExecutionServiceServer(reg, &nodeBridge{handler: h})
	}

	if h, ok := p.(VCSHandler); ok {
		apiv1.RegisterVCSServiceServer(reg, newVCSBridge(h))
	}

	if h, ok := p.(TicketHandler); ok {
		apiv1.RegisterTicketServiceServer(reg, newTicketBridge(h))
	}
}
