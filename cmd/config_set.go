package cmd

import (
	"fmt"

	"github.com/cockroachdb/errors"
	"github.com/spf13/cobra"

	"thoreinstein.com/rig/pkg/config"
	rigerrors "thoreinstein.com/rig/pkg/errors"
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
		var rollback func() error
		var wasNew bool
		if setKeychain {
			uri, rb, isNew, err := config.UpdateKeychainSecret("rig", key, value)
			if err != nil {
				return err
			}
			finalValue = uri
			rollback = rb
			wasNew = isNew
		}

		if err := config.StoreConfigValue(key, finalValue); err != nil {
			if rollback != nil {
				if rollbackErr := rollback(); rollbackErr != nil {
					sbErr := rigerrors.NewSplitBrainError(key, "rig", key, err, rollbackErr, wasNew)
					fmt.Fprint(cmd.ErrOrStderr(), sbErr.RecoveryInstructions())
					return sbErr
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
