package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string
var verbose bool

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "sre",
	Short: "SRE workflow automation CLI",
	Long: `A Go-based CLI tool for SRE workflow automation that integrates with
Git worktrees, Tmux sessions, Obsidian documentation, and command history tracking.

This tool replaces the complex bash script with an extensible, maintainable
Go application that provides better error handling and scriptability.`,
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

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.config/sre/config.toml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	// Remove the example toggle flag
	// rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".config/sre" (without extension).
		viper.AddConfigPath(home + "/.config/sre")
		viper.SetConfigType("toml")
		viper.SetConfigName("config")
	}

	viper.SetEnvPrefix("SRE")                              // Only bind SRE_* environment variables
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_")) // SRE_NOTES_PATH -> notes.path
	viper.AutomaticEnv()                                   // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil && verbose {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}
