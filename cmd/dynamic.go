package cmd

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/cockroachdb/errors"
	"github.com/spf13/cobra"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
	"thoreinstein.com/rig/pkg/plugin"
)

// registerPluginCommands scans for plugins and dynamically adds their commands to the root command.
func registerPluginCommands() {
	// 1. Initialize plugin scanner
	var scanner *plugin.Scanner
	var err error

	if gitRoot, gitErr := findGitRoot(); gitErr == nil && gitRoot != "" {
		scanner, err = plugin.NewScannerWithProjectRoot(gitRoot)
	} else {
		scanner, err = plugin.NewScanner()
	}

	if err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "Warning: failed to initialize plugin scanner: %v\n", err)
		}
		return
	}

	// 2. Scan for plugins
	result, err := scanner.Scan()
	if err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "Warning: plugin scan failed: %v\n", err)
		}
		return
	}

	// 3. Register commands
	// Track both names and aliases to prevent collisions
	collisionMap := make(map[string]string) // name/alias -> plugin/built-in name
	for _, c := range rootCmd.Commands() {
		collisionMap[c.Name()] = "built-in"
		for _, alias := range c.Aliases {
			collisionMap[alias] = "built-in"
		}
	}

	for _, p := range result.Plugins {
		if p.Manifest == nil || len(p.Manifest.Commands) == 0 {
			continue
		}

		// Validate compatibility before registering commands.
		// If incompatible, we skip the commands to avoid exposing unusable functionality.
		plugin.ValidateCompatibility(p, GetVersion())
		if p.Status != plugin.StatusCompatible {
			if verbose {
				fmt.Fprintf(os.Stderr, "Warning: skipping commands for plugin %q: %v\n", p.Name, p.Error)
			}
			continue
		}

		pluginName := p.Name
		for _, cmdDesc := range p.Manifest.Commands {
			if owner, exists := collisionMap[cmdDesc.Name]; exists {
				if verbose {
					fmt.Fprintf(os.Stderr, "Warning: skipping plugin command %q from %q: already exists (owned by %s)\n", cmdDesc.Name, pluginName, owner)
				}
				continue
			}

			// Check aliases for collisions and filter them
			var filteredAliases []string
			for _, alias := range cmdDesc.Aliases {
				if owner, exists := collisionMap[alias]; exists {
					if verbose {
						fmt.Fprintf(os.Stderr, "Warning: skipping alias %q for plugin command %q (%q): already exists (owned by %s)\n", alias, cmdDesc.Name, pluginName, owner)
					}
					continue
				}
				filteredAliases = append(filteredAliases, alias)
			}

			// Capture loop variables for the closure
			pName := pluginName
			cDesc := cmdDesc

			cobraCmd := &cobra.Command{
				Use:                cDesc.Name,
				Short:              cDesc.Short,
				Long:               cDesc.Long,
				Aliases:            filteredAliases,
				DisableFlagParsing: true, // Let the plugin handle its own flags
				RunE: func(cmd *cobra.Command, args []string) error {
					return runPluginCommand(cmd.Context(), pName, cDesc.Name, args)
				},
			}

			rootCmd.AddCommand(cobraCmd)

			// Add name and filtered aliases to collision map
			collisionMap[cDesc.Name] = pluginName
			for _, alias := range filteredAliases {
				collisionMap[alias] = pluginName
			}
		}
	}
}

// runPluginCommand starts the plugin and executes the specified command.
func runPluginCommand(ctx context.Context, pluginName, commandName string, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return errors.Wrap(err, "failed to load configuration")
	}

	// 1. Initialize plugin components
	var scanner *plugin.Scanner
	if gitRoot, gitErr := findGitRoot(); gitErr == nil && gitRoot != "" {
		scanner, err = plugin.NewScannerWithProjectRoot(gitRoot)
	} else {
		scanner, err = plugin.NewScanner()
	}
	if err != nil {
		return errors.Wrap(err, "failed to initialize plugin scanner")
	}

	executor := plugin.NewExecutor("")

	// Create manager with the config provider from the loaded appConfig
	manager, err := plugin.NewManager(executor, scanner, GetVersion(), cfg.PluginConfig)
	if err != nil {
		return errors.Wrap(err, "failed to initialize plugin manager")
	}
	defer manager.StopAll()

	// 2. Get command client and start plugin
	client, err := manager.GetCommandClient(ctx, pluginName)
	if err != nil {
		return errors.Wrapf(err, "failed to get command client for plugin %q", pluginName)
	}

	// 3. Execute the command and stream output
	stream, err := client.Execute(ctx, &apiv1.ExecuteRequest{
		Command: commandName,
		Args:    args,
	})
	if err != nil {
		return errors.Wrapf(err, "failed to execute command %q on plugin %q", commandName, pluginName)
	}

	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return errors.Wrap(err, "plugin command stream error")
		}

		if len(resp.Stdout) > 0 {
			os.Stdout.Write(resp.Stdout)
		}
		if len(resp.Stderr) > 0 {
			os.Stderr.Write(resp.Stderr)
		}

		if resp.Done {
			if resp.ExitCode != 0 {
				return fmt.Errorf("plugin command %q exited with code %d", commandName, resp.ExitCode)
			}
			break
		}
	}

	return nil
}
