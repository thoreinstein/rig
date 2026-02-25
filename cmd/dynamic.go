package cmd

import (
	"context"
	"io"
	"os"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
	"thoreinstein.com/rig/pkg/bootstrap"
	"thoreinstein.com/rig/pkg/daemon"
	"thoreinstein.com/rig/pkg/errors"
	"thoreinstein.com/rig/pkg/ui"
)

// daemonExecutor defines the interface for the daemon client used by runPluginCommand.
// This allows mocking the daemon in tests.
type daemonExecutor interface {
	ExecuteCommand(ctx context.Context, req *apiv1.CommandRequest, ui daemon.UIHandler, stdout, stderr io.Writer) error
	Close() error
}

var daemonEnsureRunning = func(ctx context.Context, rigPath string) (daemonExecutor, error) {
	return daemon.EnsureRunning(ctx, rigPath)
}

// registerPluginCommands scans for plugins and dynamically adds their commands to the root command.
func registerPluginCommands() {
	bootstrap.RegisterPluginCommandsFromConfig(rootCmd, appConfig, GetVersion(), verbose, runPluginCommand)
}

// runPluginCommand starts the plugin and executes the specified command.
func runPluginCommand(ctx context.Context, pluginName, commandName string, args []string) error {
	// Phase 7: Attempt execution via Daemon first
	if appConfig != nil && appConfig.Daemon.Enabled {
		rigPath, err := os.Executable()
		if err == nil {
			client, err := daemonEnsureRunning(ctx, rigPath)
			if err == nil {
				defer client.Close()

				// Sanitize arguments by filtering out host flags
				pluginArgs, hostArgs := bootstrap.FilterHostFlags(rootCmd.PersistentFlags(), args)

				// 3. Re-initialize configuration if host flags were parsed.
				if len(hostArgs) > 0 {
					if err := rootCmd.PersistentFlags().Parse(hostArgs); err != nil {
						return errors.Wrap(err, "failed to parse host flags")
					}
					if err := initConfig(); err != nil {
						return errors.Wrap(err, "failed to re-initialize config after parsing host flags")
					}
				}

				configJSON := bootstrap.ResolvePluginConfig(appConfig.PluginConfig, pluginName, nil)
				uiHandler := ui.NewUIServer()
				defer uiHandler.Stop()

				err = client.ExecuteCommand(ctx, &apiv1.CommandRequest{
					PluginName:  pluginName,
					CommandName: commandName,
					Args:        pluginArgs,
					ConfigJson:  configJSON,
					RigVersion:  GetVersion(),
				}, uiHandler, os.Stdout, os.Stderr)

				if err != nil {
					var dErr *errors.DaemonError
					if errors.As(err, &dErr) {
						// Fallback if explicitly requested or if it's a connection/availability issue
						// (Connect errors return nil client, so handled by the 'err == nil' check above).
						if dErr.Fallback {
							goto fallback
						}
					}
					return err
				}
				return nil
			}
		}
	}

fallback: // Delegate to the bootstrap package for heavy orchestration (Fallback).
	return bootstrap.RunPluginCommand(
		ctx,
		rootCmd.PersistentFlags(),
		pluginName,
		commandName,
		args,
		GetVersion(),
		os.Stdout,
		os.Stderr,
		&cfgFile,
		&verbose,
	)
}
