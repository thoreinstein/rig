package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

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

	// Explicitly reserve built-in commands that might be added lazily by Cobra
	reserved := []string{"help", "h", "completion"}
	for _, r := range reserved {
		if _, exists := collisionMap[r]; !exists {
			collisionMap[r] = "built-in"
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
	// Identify which args are host persistent flags and extract them.
	// This ensures we only parse host-owned flags and avoid misinterpreting
	// plugin short options (e.g. -cfoo).
	pluginArgs, hostArgs := filterHostFlags(args)

	// Re-parse host persistent flags from extracted hostArgs.
	fs := rootCmd.PersistentFlags()
	fs.ParseErrorsAllowlist.UnknownFlags = true
	if err := fs.Parse(hostArgs); err != nil {
		return errors.Wrap(err, "failed to parse host flags")
	}

	// Re-initialize configuration if host flags were parsed.
	// This ensures --config or --verbose provided after the subcommand are respected.
	initConfig()

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
		Args:    pluginArgs,
	})
	if err != nil {
		return errors.Wrapf(err, "failed to execute command %q on plugin %q", commandName, pluginName)
	}

	var gotDone bool
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
			gotDone = true
			if resp.ExitCode != 0 {
				return fmt.Errorf("plugin command %q exited with code %d", commandName, resp.ExitCode)
			}
			break
		}
	}

	if !gotDone {
		return fmt.Errorf("plugin command %q terminated prematurely (no done message received)", commandName)
	}

	return nil
}

// filterHostFlags separates arguments into plugin-owned and host-owned slices.
// It respects the '--' separator, stopping all extraction once it's encountered.
func filterHostFlags(args []string) ([]string, []string) {
	fs := rootCmd.PersistentFlags()
	var pluginArgs []string
	var hostArgs []string
	stopFiltering := false

	for i := 0; i < len(args); i++ {
		arg := args[i]

		if stopFiltering {
			pluginArgs = append(pluginArgs, arg)
			continue
		}

		if arg == "--" {
			stopFiltering = true
			pluginArgs = append(pluginArgs, arg)
			continue
		}

		if !strings.HasPrefix(arg, "-") || arg == "-" {
			pluginArgs = append(pluginArgs, arg)
			continue
		}

		// Handle long flags (--flag) and shorthand (-f)
		isLong := strings.HasPrefix(arg, "--")
		name := strings.TrimLeft(arg, "-")
		if strings.Contains(name, "=") {
			name = strings.Split(name, "=")[0]
		}

		var f *pflag.Flag
		if isLong {
			f = fs.Lookup(name)
		} else if len(name) > 0 {
			// Check if the first character is a valid shorthand
			f = fs.ShorthandLookup(name[:1])
			if f != nil && f.Value.Type() == "bool" && len(name) > 1 {
				// If first char is a boolean shorthand, it's only a host flag if
				// the next char is also a valid host shorthand (grouping).
				// Otherwise, it's likely a plugin-specific short option (e.g. -vfoo).
				if fs.ShorthandLookup(name[1:2]) == nil {
					f = nil
				}
			}
		}

		if f != nil {
			// It's a host flag.
			hostArgs = append(hostArgs, arg)

			// Handle value for non-boolean flags
			if f.Value.Type() != "bool" {
				if isLong {
					// --config file OR --config=file
					if !strings.Contains(arg, "=") && i+1 < len(args) {
						hostArgs = append(hostArgs, args[i+1])
						i++
					}
				} else {
					// -Cfile OR -C file
					if len(name) == 1 && i+1 < len(args) {
						// -C file
						hostArgs = append(hostArgs, args[i+1])
						i++
					}
					// -Cfile is already part of hostArgs via 'arg'
				}
			}
			continue
		}

		// Unknown flag, belongs to the plugin
		pluginArgs = append(pluginArgs, arg)
	}

	return pluginArgs, hostArgs
}
