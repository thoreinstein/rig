package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"syscall"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/spf13/cobra"

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

			if err := initConfig(); err != nil {
				return err
			}
			cfg := appConfig

			uiProxy := daemon.NewDaemonUIProxy()
			mgr, err := plugin.NewManager(executor, scanner, GetVersion(), cfg.PluginConfig, slog.Default(), plugin.WithUIServer(uiProxy))
			if err != nil {
				return err
			}

			pluginIdle, err := time.ParseDuration(cfg.Daemon.PluginIdleTimeout)
			if err != nil {
				slog.Error("Invalid plugin idle timeout", "value", cfg.Daemon.PluginIdleTimeout, "error", err)
				pluginIdle = 5 * time.Minute
			}
			daemonIdle, err := time.ParseDuration(cfg.Daemon.DaemonIdleTimeout)
			if err != nil {
				slog.Error("Invalid daemon idle timeout", "value", cfg.Daemon.DaemonIdleTimeout, "error", err)
				daemonIdle = 15 * time.Minute
			}

			// 2. Delegate to pkg/daemon.Serve
			return daemon.Serve(cmd.Context(), mgr, uiProxy, GetVersion(), slog.Default(), pluginIdle, daemonIdle)
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
				return errors.Wrapf(err, "failed to read PID file")
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
				return errors.Wrapf(err, "failed to find daemon process %d", pid)
			}

			if !force {
				return errors.Newf("daemon socket is unreachable and PID %d could not be verified as Rig. Use --force to signal anyway", pid)
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
