package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version information set via ldflags at build time.
var (
	// Version is the current version of rig, set via ldflags at build time.
	// Exported for use by the update command.
	Version = "dev"
	commit  = "none"
	date    = "unknown"
)

// GetVersion returns the current version string.
func GetVersion() string {
	return Version
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Long:  `Display the version, commit hash, and build date of the rig CLI.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("rig version %s\n", Version)
		fmt.Printf("  commit: %s\n", commit)
		fmt.Printf("  built:  %s\n", date)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
