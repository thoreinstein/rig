package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"text/tabwriter"

	"github.com/cockroachdb/errors"
	"github.com/spf13/cobra"

	"thoreinstein.com/rig/pkg/config"
	"thoreinstein.com/rig/pkg/project"
)

// trustStore is shared across trust subcommands, initialized via PersistentPreRunE.
var trustStore *config.TrustStore

// trustCmd represents the trust command
var trustCmd = &cobra.Command{
	Use:   "trust [path]",
	Short: "Manage project trust for configuration overrides",
	Long: `Manage the trust store for project root directories.

Trusted projects are allowed to override configuration settings via their
local .rig.toml files. Immutable keys (like credentials) are always protected
regardless of trust.

If no path is provided, the command applies to the current project root.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		ts, err := config.NewTrustStore()
		if err != nil {
			return err
		}
		trustStore = ts
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return runTrustStatus()
		}
		return runTrustAdd(args)
	},
}

// trustAddCmd represents the 'rig trust add' subcommand
var trustAddCmd = &cobra.Command{
	Use:   "add [path]",
	Short: "Trust a project root",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTrustAdd(args)
	},
}

// trustRemoveCmd represents the 'rig trust remove' subcommand
var trustRemoveCmd = &cobra.Command{
	Use:   "remove [path]",
	Short: "Revoke trust for a project root",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTrustRemove(args)
	},
}

// trustListCmd represents the 'rig trust list' subcommand
var trustListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all trusted project roots",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTrustList()
	},
}

func init() {
	rootCmd.AddCommand(trustCmd)
	trustCmd.AddCommand(trustAddCmd)
	trustCmd.AddCommand(trustRemoveCmd)
	trustCmd.AddCommand(trustListCmd)
}

func runTrustStatus() error {
	ctx, err := project.CachedDiscover("")
	if err != nil {
		if project.IsNoProjectContext(err) {
			return errors.New("no rig project found in current directory")
		}
		return err
	}

	if trustStore.IsTrusted(ctx.RootPath) {
		fmt.Printf("Project at %s: TRUSTED\n", ctx.RootPath)
	} else {
		fmt.Printf("Project at %s: NOT TRUSTED\n", ctx.RootPath)
		fmt.Println("Run 'rig trust add' to trust this project.")
	}

	return nil
}

func runTrustAdd(args []string) error {
	path, err := resolveProjectPath(args)
	if err != nil {
		return err
	}

	if err := trustStore.Add(path); err != nil {
		return err
	}

	fmt.Printf("Trusted: %s\n", path)
	return nil
}

func runTrustRemove(args []string) error {
	path, err := resolveProjectPath(args)
	if err != nil {
		return err
	}

	if err := trustStore.Remove(path); err != nil {
		return err
	}

	fmt.Printf("Removed trust: %s\n", path)
	return nil
}

func runTrustList() error {
	list := trustStore.List()
	if len(list) == 0 {
		fmt.Println("No trusted projects found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PROJECT ROOT\tTRUSTED AT")

	// Sort keys for stable output
	paths := make([]string, 0, len(list))
	for p := range list {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	for _, p := range paths {
		entry := list[p]
		fmt.Fprintf(w, "%s\t%s\n", p, entry.TrustedAt.Format("2006-01-02 15:04:05"))
	}
	return w.Flush()
}

func resolveProjectPath(args []string) (string, error) {
	if len(args) > 0 {
		target := args[0]
		// Expand environment variables
		target = os.ExpandEnv(target)

		// Make absolute if relative
		if !filepath.IsAbs(target) {
			abs, err := filepath.Abs(target)
			if err != nil {
				return "", err
			}
			target = abs
		}

		// If we can't discover a project root at target, validate the path exists
		ctx, err := project.Discover(target)
		if err != nil {
			if _, statErr := os.Stat(target); statErr != nil {
				return "", errors.Newf("path %q does not exist", target)
			}
			return target, nil
		}
		return ctx.RootPath, nil
	}

	ctx, err := project.CachedDiscover("")
	if err != nil {
		return "", errors.New("no rig project found in current directory. Please provide a path.")
	}
	return ctx.RootPath, nil
}
