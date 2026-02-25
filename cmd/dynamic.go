package cmd

import (
	"context"
	"fmt"
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

// tryDaemonExecution attempts to execute a plugin command via the daemon.
// Returns (true, nil) if the daemon handled the command successfully,
// (true, err) if the daemon handled it but returned an error,
// or (false, nil) if execution should fall back to direct mode.
func tryDaemonExecution(ctx context.Context, pluginName, commandName string, args []string) (handled bool, err error) {
	rigPath, err := os.Executable()
	if err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "daemon: could not resolve executable path, falling back to direct execution: %v\n", err)
		}
		return false, nil
	}

	client, err := daemonEnsureRunning(ctx, rigPath)
	if err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "daemon: could not connect, falling back to direct execution: %v\n", err)
		}
		return false, nil
	}
	defer client.Close()

	// Sanitize arguments by filtering out host flags
	pluginArgs, hostArgs := bootstrap.FilterHostFlags(rootCmd.PersistentFlags(), args)

	// Re-initialize configuration if host flags were parsed.
	if len(hostArgs) > 0 {
		if err := rootCmd.PersistentFlags().Parse(hostArgs); err != nil {
			return true, errors.Wrap(err, "failed to parse host flags")
		}
		if err := initConfig(); err != nil {
			return true, errors.Wrap(err, "failed to re-initialize config after parsing host flags")
		}
	}

	configJSON := bootstrap.ResolvePluginConfig(appConfig.PluginConfig, pluginName, nil)
	uiHandler := ui.NewUIServer()
	defer uiHandler.Stop()

	err = client.ExecuteCommand(ctx, &apiv1.CommandRequest{
		PluginName:  pluginName,
		CommandName: commandName,
		Args:        pluginArgs,
		Flags:       bootstrap.ParsePluginFlags(pluginArgs),
		ConfigJson:  configJSON,
		RigVersion:  GetVersion(),
	}, uiHandler, os.Stdout, os.Stderr)

	if err != nil {
		var dErr *errors.DaemonError
		if errors.As(err, &dErr) && dErr.Fallback {
			if verbose {
				fmt.Fprintf(os.Stderr, "daemon: plugin %q not found in daemon scope, falling back to direct execution\n", pluginName)
			}
			return false, nil
		}
		return true, err
	}
	return true, nil
}

// runPluginCommand starts the plugin and executes the specified command.
func runPluginCommand(ctx context.Context, pluginName, commandName string, args []string) error {
	// Attempt execution via daemon first
	if appConfig != nil && appConfig.Daemon.Enabled {
		handled, err := tryDaemonExecution(ctx, pluginName, commandName, args)
		if handled {
			return err
		}
	}

	// Direct execution fallback
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
