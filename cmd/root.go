package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"thoreinstein.com/rig/pkg/bootstrap"
	"thoreinstein.com/rig/pkg/config"
)

var cfgFile string
var verbose bool
var appConfig *config.Config

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
	cfgFile, verbose = bootstrap.PreParseGlobalFlags(os.Args)

	// 2. Initialize configuration (bootstrap)
	if err := initConfig(); err != nil {
		cobra.CheckErr(err)
	}

	// 3. Register dynamic commands from plugins
	registerPluginCommands()

	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(func() {
		_ = initConfig()
	})

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "C", "", "config file (default is $HOME/.config/rig/config.toml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	// Remove the example toggle flag
	// rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() error {
	var err error
	appConfig, verbose, err = bootstrap.InitConfig(cfgFile, verbose)
	return err
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
	bootstrap.Reset()
	viper.Reset()
}
