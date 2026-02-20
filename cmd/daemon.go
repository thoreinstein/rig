package cmd

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

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

			server := daemon.NewDaemonServer(mgr, uiProxy, GetVersion(), slog.Default())

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

			// 4. Handle signals for graceful shutdown
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

			go func() {
				<-sigCh
				fmt.Println("\nShutting down daemon...")
				mgr.StopAll()
				s.GracefulStop()
			}()

			fmt.Printf("Daemon started on %s (PID %d)\n", path, os.Getpid())
			return s.Serve(lis)
		},
	}
}

func newDaemonStopCmd() *cobra.Command {
	return &cobra.Command{
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

			process, err := os.FindProcess(pid)
			if err != nil {
				return fmt.Errorf("failed to find daemon process: %w", err)
			}

			return process.Signal(syscall.SIGTERM)
		},
	}
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
