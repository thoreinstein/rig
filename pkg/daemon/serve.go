package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
	"thoreinstein.com/rig/pkg/plugin"
)

// Serve initializes and runs the Rig background daemon.
// It handles PID management, UDS listener setup, and the gRPC server loop.
func Serve(ctx context.Context, mgr *plugin.Manager, uiProxy *DaemonUIProxy, rigVersion string, logger *slog.Logger, pluginIdle, daemonIdle time.Duration) error {
	server := NewDaemonServer(mgr, uiProxy, rigVersion, logger)
	lifecycle := NewLifecycle(mgr, server, pluginIdle, daemonIdle, logger)

	// 1. Setup UDS listener
	if err := EnsureDir(); err != nil {
		return err
	}
	path := SocketPath()
	_ = os.Remove(path) // Ensure clean start

	lis, err := net.Listen("unix", path)
	if err != nil {
		return err
	}
	defer lis.Close()

	if err := os.Chmod(path, 0o600); err != nil {
		return err
	}

	// 2. Start gRPC server
	s := grpc.NewServer()
	apiv1.RegisterDaemonServiceServer(s, server)

	if err := WritePIDFile(); err != nil {
		return err
	}
	defer func() { _ = RemovePIDFile() }()

	// 3. Handle signals and lifecycle shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		select {
		case <-ctx.Done():
			if logger != nil {
				logger.Info("Context canceled, shutting down...")
			}
		case <-sigCh:
			fmt.Println("\nShutting down daemon...")
		case <-server.ShutdownCh():
			fmt.Println("\nShutdown requested via RPC, exiting...")
		case <-lifecycle.ShutdownCh():
			fmt.Println("\nDaemon idle timeout reached, shutting down...")
		}
		mgr.StopAll()
		s.GracefulStop()
	}()

	// 4. Start lifecycle monitor (ONLY ONCE)
	go lifecycle.Run(ctx)

	fmt.Printf("Daemon started on %s (PID %d)\n", path, os.Getpid())
	return s.Serve(lis)
}
