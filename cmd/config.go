package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/cockroachdb/errors"
	"github.com/spf13/cobra"

	"thoreinstein.com/rig/pkg/config"
)

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage and inspect Rig configuration",
	Long: `Display and manage the Rig configuration.

Without subcommands, shows the current configuration values.
Use 'rig config inspect' for detailed source attribution.`,
	RunE: runConfigShow,
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
		if _, err := appLoader.Load(); err != nil {
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
		fmt.Fprintln(w, "KEY\tVALUE\tSOURCE\tPROTECTION")
		fmt.Fprintln(w, "---\t-----\t------\t----------")

		violations := appLoader.Violations()

		for _, k := range keys {
			entry := sources[k]
			sourceStr := sources.Get(k)

			// We mask potentially sensitive values (tokens, secrets)
			val := fmt.Sprintf("%v", entry.Value)
			if isSensitiveKey(k) {
				val = "********"
			}

			protection := ""
			if config.IsImmutable(k) {
				protection = "immutable"
			} else {
				// Check if this key was part of an untrusted project violation
				for _, v := range violations {
					if v.Key == k && v.Reason == config.ViolationUntrustedProject {
						protection = "untrusted"
						break
					}
				}
			}

			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", k, val, sourceStr, protection)
		}
		w.Flush()
		return nil
	},
}

// configInitCmd creates a default configuration file
var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a default configuration file",
	RunE: func(cmd *cobra.Command, args []string) error {
		return createDefaultConfig()
	},
}

// configEditCmd opens the configuration file in $EDITOR
var configEditCmd = &cobra.Command{
	Use:   "edit",
	Short: "Open configuration file in $EDITOR",
	Long: `Open the Rig configuration file in your preferred editor.

Uses $EDITOR environment variable, falls back to $VISUAL, then common editors (vim, vi, nano).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return editConfig()
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(inspectCmd)
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configEditCmd)
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return errors.Wrap(err, "failed to load configuration")
	}

	fmt.Println("Current Rig Configuration:")
	fmt.Println("==============================")

	fmt.Printf("Notes Path:          %s\n", cfg.Notes.Path)
	fmt.Printf("Daily Notes Dir:     %s\n", cfg.Notes.DailyDir)
	fmt.Printf("Template Dir:        %s\n", cfg.Notes.TemplateDir)

	if cfg.Git.BaseBranch != "" {
		fmt.Printf("Git Base Branch:     %s (override)\n", cfg.Git.BaseBranch)
	} else {
		fmt.Printf("Git Base Branch:     (auto-detect)\n")
	}

	fmt.Printf("History Database:    %s\n", cfg.History.DatabasePath)
	fmt.Printf("JIRA Enabled:        %t\n", cfg.Jira.Enabled)

	if cfg.Jira.Enabled {
		fmt.Printf("JIRA CLI Command:    %s\n", cfg.Jira.CliCommand)
	}

	fmt.Printf("Tmux Windows:        %d configured\n", len(cfg.Tmux.Windows))
	for i, window := range cfg.Tmux.Windows {
		fmt.Printf("  %d. %s", i+1, window.Name)
		if window.Command != "" {
			fmt.Printf(" (%s)", window.Command)
		}
		fmt.Println()
	}

	return nil
}

func createDefaultConfig() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return errors.Wrap(err, "failed to get home directory")
	}

	configDir := filepath.Join(homeDir, ".config", "rig")
	configFile := filepath.Join(configDir, "config.toml")

	if _, err := os.Stat(configFile); err == nil {
		fmt.Printf("Configuration file already exists at: %s\n", configFile)
		return nil
	}

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return errors.Wrap(err, "failed to create config directory")
	}

	defaultConfig := `# Rig Configuration

[notes]
path = "~/Documents/Notes"
daily_dir = "daily"
template_dir = "~/.config/rig/templates"

[git]
# Optional: override auto-detected default branch
# base_branch = "main"

[history]
database_path = "~/.histdb/zsh-history.db"
ignore_patterns = ["ls", "cd", "pwd", "clear"]

[jira]
enabled = true
cli_command = "acli"

[tmux]
session_prefix = ""

[[tmux.windows]]
name = "note"
command = "nvim {note_path}"

[[tmux.windows]]
name = "code"
command = "nvim"
working_dir = "{worktree_path}"

[[tmux.windows]]
name = "term"
working_dir = "{worktree_path}"
`

	if err := os.WriteFile(configFile, []byte(defaultConfig), 0600); err != nil {
		return errors.Wrap(err, "failed to write config file")
	}

	fmt.Printf("Default configuration created at: %s\n", configFile)
	fmt.Println("Edit this file to customize your Rig settings.")

	return nil
}

func editConfig() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return errors.Wrap(err, "failed to get home directory")
	}

	configFile := filepath.Join(homeDir, ".config", "rig", "config.toml")

	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		fmt.Println("Config file does not exist, creating default...")
		if err := createDefaultConfig(); err != nil {
			return err
		}
	}

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		for _, e := range []string{"vim", "vi", "nano"} {
			if _, err := exec.LookPath(e); err == nil {
				editor = e
				break
			}
		}
	}
	if editor == "" {
		return errors.New("no editor found: set $EDITOR environment variable")
	}

	cmd := exec.Command(editor, configFile)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
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
