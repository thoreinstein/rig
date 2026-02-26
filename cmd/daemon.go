package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"syscall"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/spf13/cobra"

	"thoreinstein.com/rig/pkg/daemon"
	"thoreinstein.com/rig/pkg/plugin"
	"thoreinstein.com/rig/pkg/project"
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
			var gitRoot string
			if ctx, err := project.CachedDiscover(""); err == nil {
				gitRoot = ctx.Markers[project.MarkerGit]
			}
			var scanner *plugin.Scanner
			var err error
			if gitRoot != "" {
				scanner, err = plugin.NewScannerWithProjectRoot(gitRoot)
			} else {
				scanner, err = plugin.NewScanner()
			}
			if err != nil {
				return err
			}

			executor := plugin.NewExecutor("")

			// Load configuration to get daemon settings
			if err := initConfig(); err != nil {
				return err
			}
			cfg := appConfig

			// Get rig version from build-time info
			rigVersion := GetVersion()

			// 2. Setup UI Proxy and Manager
			uiProxy := daemon.NewDaemonUIProxy()
			manager, err := plugin.NewManager(executor, scanner, rigVersion, appConfig.PluginConfig, slog.Default(), plugin.WithUIServer(uiProxy))
			if err != nil {
				return errors.Wrap(err, "failed to initialize plugin manager")
			}

			// Parse timeouts from config
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

			// 3. Start daemon server via Serve
			fmt.Printf("Starting Rig daemon (version %s)...\n", rigVersion)
			fmt.Printf("Socket: %s\n", daemon.SocketPath())

			// Serve handles context, signals, and server loop
			return daemon.Serve(cmd.Context(), manager, uiProxy, rigVersion, slog.Default(), pluginIdle, daemonIdle)
		},
	}
}

func newDaemonStopCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop the Rig background daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !daemon.IsRunning() {
				fmt.Println("Daemon is not running.")
				return nil
			}

			// Try graceful shutdown via gRPC first
			ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Second)
			defer cancel()

			client, err := daemon.NewClient(ctx)
			if err == nil {
				defer client.Close()
				fmt.Println("Shutting down daemon via gRPC...")
				if shutdownErr := client.Shutdown(ctx, force); shutdownErr == nil {
					fmt.Println("Daemon stop requested.")
					return nil
				} else {
					fmt.Printf("Graceful shutdown failed: %v\n", shutdownErr)
				}
			} else {
				fmt.Printf("Could not connect to daemon for graceful shutdown: %v\n", err)
			}

			// Fallback: Signal the process directly
			pid, err := daemon.ReadPIDFile()
			if err != nil {
				return errors.Wrap(err, "failed to read PID file for fallback shutdown")
			}

			process, err := os.FindProcess(pid)
			if err != nil {
				return errors.Wrapf(err, "failed to find daemon process %d", pid)
			}

			// Verify identity before signaling if not forced
			if !daemon.CheckIdentity(pid) && !force {
				return errors.Newf("daemon socket is unreachable and PID %d could not be verified as Rig. Use --force to signal anyway", pid)
			}

			fmt.Printf("Signaling PID %d with SIGTERM (fallback)...\n", pid)
			if err := process.Signal(syscall.SIGTERM); err != nil {
				return errors.Wrap(err, "failed to signal daemon process")
			}

			fmt.Println("Daemon process signaled.")
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force stop by signaling PID if daemon is unresponsive")
	return cmd
}

func newDaemonStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Check the status of the Rig background daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !daemon.IsRunning() {
				fmt.Println("Daemon is not running.")
				return nil
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Second)
			defer cancel()

			client, err := daemon.NewClient(ctx)
			if err != nil {
				fmt.Printf("Daemon is running (PID file exists) but connection failed: %v\n", err)
				fmt.Printf("Socket: %s\n", daemon.SocketPath())
				return nil
			}
			defer client.Close()

			resp, err := client.Status(ctx)
			if err != nil {
				return errors.Wrap(err, "failed to get daemon status")
			}

			fmt.Printf("Daemon is running.\n")
			fmt.Printf("Version: %s\n", resp.DaemonVersion)
			fmt.Printf("PID:     %d\n", resp.Pid)
			fmt.Printf("Uptime:  %ds\n", resp.UptimeSeconds)
			fmt.Printf("Socket:  %s\n", daemon.SocketPath())
			return nil
		},
	}
}
