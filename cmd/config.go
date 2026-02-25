package cmd

import (
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/cockroachdb/errors"
	"github.com/spf13/cobra"
)

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage and inspect Rig configuration",
}

// inspectCmd represents the inspect subcommand
var inspectCmd = &cobra.Command{
	Use:   "inspect",
	Short: "Show the final resolved configuration with source attribution",
	RunE: func(cmd *cobra.Command, args []string) error {
		if appLoader == nil {
			return errors.New("configuration loader not initialized")
		}

		// Reload to ensure we have the latest and sources are populated
		cfg, err := appLoader.Load()
		if err != nil {
			return err
		}

		sources := appLoader.Sources()

		// Get all keys and sort them for stable output
		var keys []string
		for k := range sources {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "KEY\tVALUE\tSOURCE")
		fmt.Fprintln(w, "---\t-----\t------")

		for _, k := range keys {
			entry := sources[k]
			sourceStr := sources.Get(k)

			// We mask potentially sensitive values (tokens, secrets)
			val := fmt.Sprintf("%v", entry.Value)
			if isSensitiveKey(k) {
				val = "********"
			}

			fmt.Fprintf(w, "%s\t%s\t%s\n", k, val, sourceStr)
		}
		w.Flush()

		_ = cfg // cfg is just used to ensure successful load
		return nil
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(inspectCmd)
}

func isSensitiveKey(key string) bool {
	sensitive := []string{"token", "secret", "key", "password", "api_key"}
	lowerKey := strings.ToLower(key)
	for _, s := range sensitive {
		if strings.Contains(lowerKey, s) {
			return true
		}
	}
	return false
}
