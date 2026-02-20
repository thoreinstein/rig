package cmd

import (
	"context"
	"os"

	"thoreinstein.com/rig/pkg/bootstrap"
)

// registerPluginCommands scans for plugins and dynamically adds their commands to the root command.
func registerPluginCommands() {
	bootstrap.RegisterPluginCommandsFromConfig(rootCmd, appConfig, GetVersion(), verbose, runPluginCommand)
}

// runPluginCommand starts the plugin and executes the specified command.
func runPluginCommand(ctx context.Context, pluginName, commandName string, args []string) error {
	// Delegate to the bootstrap package for heavy orchestration.
	// This ensures CLI logic remains decoupled from core plugin execution.
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
