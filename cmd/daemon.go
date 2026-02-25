package cmd

import (
	"context"
	"fmt"
	"log/slog"
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

			// Get rig version from root command
			rigVersion := rootCmd.Version
			if rigVersion == "" {
				rigVersion = "dev"
			}

			// 2. Setup UI Proxy and Manager
			// Plugin config is nil — daemon discovers plugins via scanner only.
			// Config-driven plugin settings will be restored when config loading
			// is wired through pkg/project in a future change.
			uiProxy := daemon.NewDaemonUIProxy()
			manager, err := plugin.NewManager(executor, scanner, rigVersion, nil, slog.Default(), plugin.WithUIServer(uiProxy))
			if err != nil {
				return errors.Wrap(err, "failed to initialize plugin manager")
			}

			// 3. Start daemon server via Serve
			fmt.Printf("Starting Rig daemon (version %s)...\n", rigVersion)
			fmt.Printf("Socket: %s\n", daemon.SocketPath())

			// Serve handles context, signals, and server loop
			return daemon.Serve(context.Background(), manager, uiProxy, rigVersion, slog.Default(), 5*time.Minute, 15*time.Minute)
		},
	}
}

func newDaemonStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the Rig background daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !daemon.IsRunning() {
				fmt.Println("Daemon is not running.")
				return nil
			}

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			client, err := daemon.NewClient(ctx)
			if err != nil {
				return errors.Wrap(err, "failed to connect to daemon")
			}
			defer client.Close()

			if err := client.Shutdown(ctx, false); err != nil {
				return errors.Wrap(err, "failed to shutdown daemon")
			}

			fmt.Println("Daemon stop requested.")
			return nil
		},
	}
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

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
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
