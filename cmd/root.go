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
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.config/rig/config.toml)")
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
	if err := viper.ReadInConfig(); err == nil && verbose {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}

	// Load repository-local config (.rig.toml) if present
	// This merges on top of the user config, allowing per-repo overrides
	loadRepoLocalConfig()

	// Check for security warnings (tokens in config file)
	cfg, err := config.Load()
	if err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "Warning: could not load config for security checks: %v\n", err)
		}
		return
	}

	warnings := config.CheckSecurityWarnings(cfg)
	for _, w := range warnings {
		fmt.Fprintf(os.Stderr, "Warning: %s\n", w.Message)
	}
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
