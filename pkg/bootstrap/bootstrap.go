package bootstrap

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
	"thoreinstein.com/rig/pkg/config"
	"thoreinstein.com/rig/pkg/plugin"
)

var (
	lastLoadedConfig  string
	lastLoadedVerbose bool
	loadedConfig      *config.Config
)

// PreParseGlobalFlags manually scans os.Args for --config and --verbose flags
// before the main Cobra execution. This is a bootstrap step for configuration.
// It stops scanning as soon as it hits a non-flag argument or the "--" marker.
func PreParseGlobalFlags(args []string) (string, bool) {
	var cfgFile string
	var verbose bool

	for i := 1; i < len(args); i++ {
		arg := args[i]

		// Stop parsing at the standard end-of-options marker
		if arg == "--" {
			break
		}

		// Stop parsing at the first non-flag argument (the subcommand)
		if !strings.HasPrefix(arg, "-") {
			break
		}

		switch {
		case arg == "--config" || arg == "-C":
			if i+1 < len(args) {
				cfgFile = args[i+1]
				i++
			}
		case strings.HasPrefix(arg, "--config="):
			cfgFile = strings.TrimPrefix(arg, "--config=")
		case strings.HasPrefix(arg, "-C="):
			cfgFile = strings.TrimPrefix(arg, "-C=")
		case strings.HasPrefix(arg, "-C") && len(arg) > 2:
			cfgFile = arg[2:]
		case arg == "--verbose" || arg == "-v":
			verbose = true
		}
	}

	return cfgFile, verbose
}

// InitConfig reads in config file and ENV variables if set.
// It returns the loaded config and the actual verbosity state.
func InitConfig(cfgFile string, verbose bool) (*config.Config, bool, error) {
	// Skip if already loaded with same parameters (unless in test)
	if os.Getenv("GO_TEST") != "true" && loadedConfig != nil && cfgFile == lastLoadedConfig && verbose == lastLoadedVerbose {
		return loadedConfig, verbose, nil
	}

	// Reset Viper state to avoid carrying over stale settings from previous loads.
	viper.Reset()

	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, verbose, errors.Wrap(err, "failed to get home directory")
		}
		viper.AddConfigPath(filepath.Join(home, ".config", "rig"))
		viper.SetConfigType("toml")
		viper.SetConfigName("config")
	}

	viper.SetEnvPrefix("RIG")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil && verbose {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}

	// Load repository-local config (.rig.toml) if present
	LoadRepoLocalConfig(verbose)

	cfg, err := config.Load()
	if err != nil {
		return nil, verbose, err
	}

	// Check for security warnings
	warnings := config.CheckSecurityWarnings(cfg)
	for _, w := range warnings {
		fmt.Fprintf(os.Stderr, "Warning: %s\n", w.Message)
	}

	// Update state
	lastLoadedConfig = cfgFile
	lastLoadedVerbose = verbose
	loadedConfig = cfg

	return cfg, verbose, nil
}

// RegisterPluginCommandsFromConfig scans for plugins and dynamically adds their commands to the root command.
func RegisterPluginCommandsFromConfig(rootCmd *cobra.Command, cfg *config.Config, rigVersion string, verbose bool, runPluginCmd func(ctx context.Context, pluginName, commandName string, args []string) error) {
	// 1. Initialize plugin scanner
	var scanner *plugin.Scanner
	var err error

	if gitRoot, gitErr := FindGitRoot(); gitErr == nil && gitRoot != "" {
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
	collisionMap := make(map[string]string)
	for _, c := range rootCmd.Commands() {
		collisionMap[c.Name()] = "built-in"
		for _, alias := range c.Aliases {
			collisionMap[alias] = "built-in"
		}
	}

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

		plugin.ValidateCompatibility(p, rigVersion)
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

			pName := pluginName
			cDesc := cmdDesc

			cobraCmd := &cobra.Command{
				Use:                cDesc.Name,
				Short:              cDesc.Short,
				Long:               cDesc.Long,
				Aliases:            filteredAliases,
				DisableFlagParsing: true,
				RunE: func(cmd *cobra.Command, args []string) error {
					return runPluginCmd(cmd.Context(), pName, cDesc.Name, args)
				},
			}

			rootCmd.AddCommand(cobraCmd)

			collisionMap[cDesc.Name] = pluginName
			for _, alias := range filteredAliases {
				collisionMap[alias] = pluginName
			}
		}
	}
}

// RunPluginCommand executes a command provided by a plugin.
// It handles flag parsing, configuration initialization, and gRPC communication.
func RunPluginCommand(ctx context.Context, hostFlags *pflag.FlagSet, pluginName, commandName string, args []string, rigVersion string, stdout, stderr io.Writer, cfgFile *string, verbose *bool) error {
	// 1. Identify which args are host persistent flags and extract them.
	pluginArgs, hostArgs := FilterHostFlags(hostFlags, args)

	// 2. Re-parse host persistent flags from extracted hostArgs.
	// Preserve global flagset state
	origUnknownFlags := hostFlags.ParseErrorsAllowlist.UnknownFlags
	hostFlags.ParseErrorsAllowlist.UnknownFlags = true
	defer func() { hostFlags.ParseErrorsAllowlist.UnknownFlags = origUnknownFlags }()

	if err := hostFlags.Parse(hostArgs); err != nil {
		return errors.Wrap(err, "failed to parse host flags")
	}

	// 3. Re-initialize configuration if host flags were parsed.
	cfg, verb, err := InitConfig(*cfgFile, *verbose)
	if err != nil {
		return errors.Wrap(err, "failed to load configuration")
	}
	*verbose = verb // Update host verbosity

	// 4. Initialize plugin components
	var scanner *plugin.Scanner
	if gitRoot, gitErr := FindGitRoot(); gitErr == nil && gitRoot != "" {
		scanner, err = plugin.NewScannerWithProjectRoot(gitRoot)
	} else {
		scanner, err = plugin.NewScanner()
	}
	if err != nil {
		return errors.Wrap(err, "failed to initialize plugin scanner")
	}

	executor := plugin.NewExecutor("")

	// Create manager with the config provider from the loaded config
	manager, err := plugin.NewManager(executor, scanner, rigVersion, cfg.PluginConfig, slog.Default())
	if err != nil {
		return errors.Wrap(err, "failed to initialize plugin manager")
	}
	defer manager.StopAll()

	// 5. Get command client and start plugin
	client, err := manager.GetCommandClient(ctx, pluginName)
	if err != nil {
		return errors.Wrapf(err, "failed to get command client for plugin %q", pluginName)
	}

	// 6. Execute the command and stream output
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
			_, _ = stdout.Write(resp.Stdout)
		}
		if len(resp.Stderr) > 0 {
			_, _ = stderr.Write(resp.Stderr)
		}

		if resp.Done {
			gotDone = true
			if resp.ExitCode != 0 {
				return errors.Errorf("plugin command %q exited with code %d", commandName, resp.ExitCode)
			}
			break
		}
	}

	if !gotDone {
		return errors.Errorf("plugin command %q terminated prematurely (no done message received)", commandName)
	}

	return nil
}

// ResolvePluginConfig fetches the JSON configuration for a specific plugin using the provided provider.
func ResolvePluginConfig(provider plugin.ConfigProvider, pluginName string, logger *slog.Logger) []byte {
	configJSON := []byte("{}")
	if provider != nil {
		data, err := provider(pluginName)
		if err != nil {
			if logger != nil {
				logger.Debug("failed to get config for plugin", "plugin", pluginName, "error", err)
			}
		} else if len(data) > 0 {
			configJSON = data
		}
	}
	return configJSON
}

// FilterHostFlags separates arguments into plugin-owned and host-owned slices.
// It respects the '--' separator, stopping all extraction once it's encountered.
func FilterHostFlags(fs *pflag.FlagSet, args []string) ([]string, []string) {
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
					// -Cfile OR -C=file OR -C file
					if !strings.Contains(arg, "=") && len(name) == 1 && i+1 < len(args) {
						// -C file
						hostArgs = append(hostArgs, args[i+1])
						i++
					}
					// -Cfile and -C=file are already part of hostArgs via 'arg'
				}
			}
			continue
		}

		// Unknown flag, belongs to the plugin
		pluginArgs = append(pluginArgs, arg)
	}

	return pluginArgs, hostArgs
}

// LoadRepoLocalConfig loads .rig.toml from current directory or git root.
func LoadRepoLocalConfig(verbose bool) {
	var localConfigPaths []string

	if gitRoot, err := FindGitRoot(); err == nil && gitRoot != "" {
		localConfigPaths = append(localConfigPaths, filepath.Join(gitRoot, ".rig.toml"))
		cwd, _ := os.Getwd()
		if cwd != gitRoot {
			localConfigPaths = append(localConfigPaths, ".rig.toml")
		}
	} else {
		localConfigPaths = append(localConfigPaths, ".rig.toml")
	}

	for _, configPath := range localConfigPaths {
		if _, err := os.Stat(configPath); err == nil {
			localViper := viper.New()
			localViper.SetConfigFile(configPath)

			if err := localViper.ReadInConfig(); err != nil {
				if verbose {
					fmt.Fprintf(os.Stderr, "Warning: could not read local config %s: %v\n", configPath, err)
				}
				continue
			}

			if verbose {
				fmt.Fprintf(os.Stderr, "Using repository config: %s\n", configPath)
			}

			if err := viper.MergeConfigMap(localViper.AllSettings()); err != nil {
				if verbose {
					fmt.Fprintf(os.Stderr, "Warning: could not merge local config: %v\n", err)
				}
			}
		}
	}
}

// FindGitRoot finds the root of the current git repository
func FindGitRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	dir := cwd
	for {
		gitPath := filepath.Join(dir, ".git")
		if info, err := os.Stat(gitPath); err == nil {
			if info.IsDir() || info.Mode().IsRegular() {
				return dir, nil
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", nil
		}
		dir = parent
	}
}

// Reset clears the cached configuration state.
func Reset() {
	lastLoadedConfig = ""
	lastLoadedVerbose = false
	loadedConfig = nil
}
