package cmd

import (
	"context"
	"os"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
	"thoreinstein.com/rig/pkg/bootstrap"
	"thoreinstein.com/rig/pkg/daemon"
	"thoreinstein.com/rig/pkg/errors"
	"thoreinstein.com/rig/pkg/ui"
)

// registerPluginCommands scans for plugins and dynamically adds their commands to the root command.
func registerPluginCommands() {
	bootstrap.RegisterPluginCommandsFromConfig(rootCmd, appConfig, GetVersion(), verbose, runPluginCommand)
}

// runPluginCommand starts the plugin and executes the specified command.
func runPluginCommand(ctx context.Context, pluginName, commandName string, args []string) error {
	// Phase 7: Attempt execution via Daemon first
	if appConfig != nil {
		rigPath, _ := os.Executable()
		client, err := daemon.EnsureRunning(ctx, rigPath)
		if err == nil {
			defer client.Close()

			// Sanitize arguments by filtering out host flags
			pluginArgs, hostArgs := bootstrap.FilterHostFlags(rootCmd.PersistentFlags(), args)

			// 3. Re-initialize configuration if host flags were parsed.
			if len(hostArgs) > 0 {
				if err := rootCmd.PersistentFlags().Parse(hostArgs); err != nil {
					return errors.Wrap(err, "failed to parse host flags")
				}
				initConfig()
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
			if err == nil || !errors.IsDaemonError(err) {
				return err
			}
			// If it's a DaemonError (transport/availability), fallback to direct execution
		}
	}
	// Delegate to the bootstrap package for heavy orchestration (Fallback).
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
