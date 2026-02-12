package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/cockroachdb/errors"
	"github.com/spf13/cobra"

	"thoreinstein.com/rig/pkg/plugin"
)

// pluginsCmd represents the plugins command
var pluginsCmd = &cobra.Command{
	Use:   "plugins",
	Short: "Manage rig plugins",
	Long:  `Discover and manage third-party plugins for rig.`,
}

// pluginsListCmd represents the plugins list command
var pluginsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List discovered plugins",
	Long:  `Scan the plugin directory (~/.config/rig/plugins) and list all discovered plugins.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runPluginsListCommand()
	},
}

func init() {
	rootCmd.AddCommand(pluginsCmd)
	pluginsCmd.AddCommand(pluginsListCmd)
}

func runPluginsListCommand() error {
	scanner, err := plugin.NewScanner()
	if err != nil {
		return errors.Wrap(err, "failed to initialize plugin scanner")
	}
	result, err := scanner.Scan()
	if err != nil {
		return errors.Wrap(err, "failed to scan for plugins")
	}

	if len(result.Plugins) == 0 {
		fmt.Printf("No plugins found in %s\n", scanner.Path)
		return nil
	}

	// Validate compatibility for all found plugins
	for i := range result.Plugins {
		_ = plugin.ValidateCompatibility(&result.Plugins[i], GetVersion())
	}

	fmt.Printf("Found %d plugin(s) in %s:\n\n", len(result.Plugins), scanner.Path)

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAME\tVERSION\tSTATUS\tPATH")

	for _, p := range result.Plugins {
		version := p.Version
		if version == "" {
			version = "unknown"
		}

		status := string(p.Status)
		if p.Status != plugin.StatusCompatible && p.Error != nil {
			status = fmt.Sprintf("%s (%v)", status, p.Error)
		}
		
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", p.Name, version, status, p.Path)
	}
	w.Flush()

	return nil
}
