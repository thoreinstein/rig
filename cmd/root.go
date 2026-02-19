package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"thoreinstein.com/rig/pkg/config"
)

var cfgFile string
var verbose bool
var appConfig *config.Config
var lastLoadedConfig string
var lastLoadedVerbose bool

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "rig",
	Short: "Rig - developer workflow automation",
	Long: `Rig is a Go-based CLI tool for developer workflow automation that integrates with
Git worktrees, Tmux sessions, Obsidian documentation, and command history tracking.

This tool provides an extensible, maintainable way to rig up your development
environment with better error handling and scriptability.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	// 1. Pre-parse global flags to initialize config early.
	// This is needed because registerPluginCommands depends on the loaded config.
	preParseGlobalFlags()

	// 2. Initialize configuration (bootstrap)
	initConfig()

	// 3. Register dynamic commands from plugins
	registerPluginCommands()

	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

// preParseGlobalFlags manually scans os.Args for --config and --verbose flags
// before the main Cobra execution. This is a bootstrap step for configuration.
// It stops scanning as soon as it hits a non-flag argument (the subcommand).
func preParseGlobalFlags() {
	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]

		// Stop parsing at the first non-flag argument
		if !strings.HasPrefix(arg, "-") {
			break
		}

		switch {
		case arg == "--config" || arg == "-C":
			if i+1 < len(os.Args) {
				cfgFile = os.Args[i+1]
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
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "C", "", "config file (default is $HOME/.config/rig/config.toml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	// Remove the example toggle flag
	// rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// initConfig reads in config file and ENV variables if set.
// Config precedence (highest to lowest):
// 1. Environment variables (RIG_*)
// 2. Repository-local config (.rig.toml in current dir or git root)
// 3. User config (~/.config/rig/config.toml)
// 4. Defaults
func initConfig() {
	// Skip if already loaded with same parameters (unless in test)
	if os.Getenv("GO_TEST") != "true" && appConfig != nil && cfgFile == lastLoadedConfig && verbose == lastLoadedVerbose {
		return
	}

	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".config/rig" (without extension).
		viper.AddConfigPath(home + "/.config/rig")
		viper.SetConfigType("toml")
		viper.SetConfigName("config")
	}

	viper.SetEnvPrefix("RIG")                              // Only bind RIG_* environment variables
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_")) // RIG_NOTES_PATH -> notes.path
	viper.AutomaticEnv()                                   // read in environment variables that match

	// If a config file is found, read it in.
	err := viper.ReadInConfig()
	if err == nil && verbose {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}

	// Load repository-local config (.rig.toml) if present
	// This merges on top of the user config, allowing per-repo overrides
	loadRepoLocalConfig()

	// Update state
	lastLoadedConfig = cfgFile
	lastLoadedVerbose = verbose

	// Check for security warnings (tokens in config file)
	cfg, err := config.Load()
	if err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "Warning: could not load config for security checks: %v\n", err)
		}
		return
	}
	appConfig = cfg

	warnings := config.CheckSecurityWarnings(appConfig)
	for _, w := range warnings {
		fmt.Fprintf(os.Stderr, "Warning: %s\n", w.Message)
	}
}

// loadConfig returns the already loaded configuration or loads it if it hasn't been yet.
// It always returns the latest configuration derived from viper.
func loadConfig() (*config.Config, error) {
	return config.Load()
}

// resetConfig clears the cached configuration.
// This is primarily used in tests to ensure each test starts with a fresh config.
func resetConfig() {
	appConfig = nil
	viper.Reset()
}

// loadRepoLocalConfig loads .rig.toml from current directory or git root.
// Values from the local config merge on top of the user config.
func loadRepoLocalConfig() {
	var localConfigPaths []string

	// Try to find git root first (parent config)
	if gitRoot, err := findGitRoot(); err == nil && gitRoot != "" {
		localConfigPaths = append(localConfigPaths, filepath.Join(gitRoot, ".rig.toml"))

		// If we are not in the root, also check current directory (child config)
		cwd, _ := os.Getwd()
		if cwd != gitRoot {
			localConfigPaths = append(localConfigPaths, ".rig.toml")
		}
	} else {
		// Fallback if no git root found
		localConfigPaths = append(localConfigPaths, ".rig.toml")
	}

	for _, configPath := range localConfigPaths {
		if _, err := os.Stat(configPath); err == nil {
			// Create a new viper instance to read the local config
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

			// Merge local config into main viper instance
			if err := viper.MergeConfigMap(localViper.AllSettings()); err != nil {
				if verbose {
					fmt.Fprintf(os.Stderr, "Warning: could not merge local config: %v\n", err)
				}
			}
		}
	}
}

// findGitRoot finds the root of the current git repository
func findGitRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Walk up the directory tree looking for .git
	dir := cwd
	for {
		gitPath := filepath.Join(dir, ".git")
		if info, err := os.Stat(gitPath); err == nil {
			// Check if it's a directory (regular repo) or file (worktree)
			if info.IsDir() || info.Mode().IsRegular() {
				return dir, nil
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root
			return "", nil
		}
		dir = parent
	}
}
