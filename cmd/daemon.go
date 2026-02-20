package cmd

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
	"thoreinstein.com/rig/pkg/daemon"
	"thoreinstein.com/rig/pkg/plugin"
)

func init() {
	rootCmd.AddCommand(newDaemonCmd())
}

func newDaemonCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Manage the Rig background daemon",
	}

	cmd.AddCommand(newDaemonStartCmd())
	cmd.AddCommand(newDaemonStopCmd())
	cmd.AddCommand(newDaemonStatusCmd())

	return cmd
}

func newDaemonStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start the Rig background daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			if daemon.IsRunning() {
				fmt.Println("Daemon is already running.")
				return nil
			}

			// 1. Initialize components
			scanner, err := plugin.NewScanner()
			if err != nil {
				return err
			}

			executor := plugin.NewExecutor("")

			// For simplicity in Phase 7, we'll use a basic config provider.
			// Ideally, this should come from the loaded config.
			initConfig()
			cfg := appConfig

			uiProxy := daemon.NewDaemonUIProxy()
			mgr, err := plugin.NewManager(executor, scanner, GetVersion(), cfg.PluginConfig, slog.Default(), plugin.WithUIServer(uiProxy))
			if err != nil {
				return err
			}
			defer mgr.StopAll()

			server := daemon.NewDaemonServer(mgr, uiProxy, GetVersion(), slog.Default())

			pluginIdle, _ := time.ParseDuration(cfg.Daemon.PluginIdleTimeout)
			daemonIdle, _ := time.ParseDuration(cfg.Daemon.DaemonIdleTimeout)
			lifecycle := daemon.NewLifecycle(mgr, server, pluginIdle, daemonIdle, slog.Default())

			// 2. Setup UDS listener
			if err := daemon.EnsureDir(); err != nil {
				return err
			}
			path := daemon.SocketPath()
			_ = os.Remove(path) // Ensure clean start

			lis, err := net.Listen("unix", path)
			if err != nil {
				return err
			}
			defer lis.Close()

			if err := os.Chmod(path, 0o600); err != nil {
				return err
			}

			// 3. Start gRPC server
			s := grpc.NewServer()
			apiv1.RegisterDaemonServiceServer(s, server)

			if err := daemon.WritePIDFile(); err != nil {
				return err
			}
			defer func() { _ = daemon.RemovePIDFile() }()

			// 4. Start lifecycle monitor
			go lifecycle.Run(cmd.Context())

			// 5. Handle signals and lifecycle shutdown
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

			go func() {
				select {
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
			fmt.Printf("Daemon started on %s (PID %d)\n", path, os.Getpid())
			return s.Serve(lis)
		},
	}
}

func newDaemonStopCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop the Rig background daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			pid, err := daemon.ReadPIDFile()
			if err != nil {
				if os.IsNotExist(err) {
					fmt.Println("Daemon is not running.")
					return nil
				}
				return fmt.Errorf("failed to read PID file: %w", err)
			}

			// 1. Try to connect and shut down via gRPC
			client, err := daemon.NewClient(cmd.Context())
			if err == nil {
				defer client.Close()
				status, err := client.Status(cmd.Context())
				if err == nil && int(status.Pid) == pid {
					fmt.Println("Shutting down daemon via gRPC...")
					return client.Shutdown(cmd.Context(), force)
				}
			}

			// 2. If gRPC fails or PID mismatch, validate process before signaling
			process, err := os.FindProcess(pid)
			if err != nil {
				return fmt.Errorf("failed to find daemon process: %w", err)
			}

			if !force {
				return fmt.Errorf("daemon socket is unreachable and PID %d could not be verified as Rig. Use --force to signal anyway", pid)
			}

			fmt.Printf("Signaling PID %d with SIGTERM (force)...\n", pid)
			return process.Signal(syscall.SIGTERM)
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force stop by signaling PID if daemon is unresponsive")
	return cmd
}
func newDaemonStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the status of the Rig background daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !daemon.IsRunning() {
				fmt.Println("Daemon is not running.")
				return nil
			}

			client, err := daemon.NewClient(cmd.Context())
			if err != nil {
				return err
			}
			defer client.Close()

			// Status RPC implementation in Phase 5 was basic,
			// we can expand it here if needed.
			fmt.Println("Daemon is running.")
			return nil
		},
	}
}
