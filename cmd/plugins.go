package cmd

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/cockroachdb/errors"
	"github.com/spf13/cobra"

	"thoreinstein.com/rig/pkg/bootstrap"
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
	Long: `Scan plugin directories and list all discovered plugins.

System plugins are loaded from ~/.config/rig/plugins.
When inside a git repository, project plugins are also loaded from
<project-root>/.rig/plugins. Project plugins override system plugins
with the same name.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runPluginsListCommand()
	},
}

func init() {
	rootCmd.AddCommand(pluginsCmd)
	pluginsCmd.AddCommand(pluginsListCmd)
}

func runPluginsListCommand() error {
	var scanner *plugin.Scanner
	var err error

	if gitRoot, gitErr := bootstrap.FindGitRoot(); gitErr == nil && gitRoot != "" {
		scanner, err = plugin.NewScannerWithProjectRoot(gitRoot)
	} else {
		scanner, err = plugin.NewScanner()
	}
	if err != nil {
		return errors.Wrap(err, "failed to initialize plugin scanner")
	}

	result, err := scanner.Scan()
	if err != nil {
		return errors.Wrap(err, "failed to scan for plugins")
	}

	if len(result.Plugins) == 0 {
		fmt.Printf("No plugins found in %s\n", strings.Join(scanner.Paths, ", "))
		return nil
	}

	// Validate compatibility for all found plugins
	for i := range result.Plugins {
		plugin.ValidateCompatibility(result.Plugins[i], GetVersion())
	}

	fmt.Printf("Found %d plugin(s) in %s:\n\n", len(result.Plugins), strings.Join(scanner.Paths, ", "))

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAME\tVERSION\tSOURCE\tSTATUS\tPATH")

	for _, p := range result.Plugins {
		version := p.Version
		if version == "" {
			version = "unknown"
		}

		source := p.Source
		if source == "" {
			source = "system"
		}

		status := string(p.Status)
		if p.Status != plugin.StatusCompatible && p.Error != nil {
			status = fmt.Sprintf("%s (%v)", status, p.Error)
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", p.Name, version, source, status, p.Path)
	}
	w.Flush()

	return nil
}
