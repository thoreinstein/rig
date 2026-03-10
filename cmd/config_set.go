package cmd

import (
	"fmt"

	"github.com/cockroachdb/errors"
	"github.com/spf13/cobra"

	"thoreinstein.com/rig/pkg/config"
)

var (
	setKeychain bool
)

// configSetCmd represents the set subcommand
var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Update a configuration value",
	Long: `Update a configuration value in the user's config file.

By default, the value is stored in plaintext in ~/.config/rig/config.toml.
Use --keychain to store the value in the system keychain and save a reference URI.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]
		value := args[1]

		if !setKeychain && config.IsSensitiveKey(key) {
			fmt.Fprintf(cmd.OutOrStdout(), "Warning: Storing sensitive key %q in plaintext. Consider using --keychain.\n", key)
		}

		finalValue := value
		isNewEntry := false
		var oldSecret string
		if setKeychain {
			// Read existing value for rollback. Only treat ErrNotFound as "new entry";
			// other errors (locked keychain, timeout) leave isNewEntry false and
			// oldSecret empty, so rollback is best-effort.
			existing, probeErr := config.GetKeychainSecret("rig", key)
			if config.IsKeychainNotFound(probeErr) {
				isNewEntry = true
			} else if probeErr == nil {
				oldSecret = existing
			}

			// Use 'rig' as service and the key as account
			uri, err := config.StoreKeychainSecret("rig", key, value)
			if err != nil {
				return errors.Wrap(err, "failed to store secret in keychain")
			}
			finalValue = uri
		}

		if err := config.StoreConfigValue(key, finalValue); err != nil {
			// Roll back keychain entry if config update fails.
			if setKeychain {
				if isNewEntry {
					if rollbackErr := config.DeleteKeychainSecret("rig", key); rollbackErr != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to clean up keychain entry for %q during rollback: %v\n", key, rollbackErr)
					}
				} else if oldSecret != "" {
					if _, rollbackErr := config.StoreKeychainSecret("rig", key, oldSecret); rollbackErr != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to restore keychain entry for %q during rollback: %v\n", key, rollbackErr)
					}
				}
			}
			return errors.Wrap(err, "failed to update configuration")
		}

		if setKeychain {
			fmt.Fprintf(cmd.OutOrStdout(), "Stored secret for %q in system keychain.\n", key)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Updated %q in configuration.\n", key)
		return nil
	},
}

func init() {
	configCmd.AddCommand(configSetCmd)
	configSetCmd.Flags().BoolVar(&setKeychain, "keychain", false, "Store the value in the system keychain")
}
