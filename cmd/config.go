package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/cockroachdb/errors"
	"github.com/spf13/cobra"

	"thoreinstein.com/rig/pkg/config"
)

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage Rig configuration",
	Long: `Display and manage the Rig configuration.

This command shows the current configuration values and can help with
initial setup by creating a default configuration file.`,
	RunE: runConfigCommand,
}

// configEditCmd represents the config edit subcommand
var configEditCmd = &cobra.Command{
	Use:   "edit",
	Short: "Open configuration file in $EDITOR",
	Long: `Open the Rig configuration file in your preferred editor.

Uses $EDITOR environment variable, falls back to $VISUAL, then common editors (vim, vi, nano).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return editConfig()
	},
}

var (
	configInit bool
	configShow bool
)

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configEditCmd)

	configCmd.Flags().BoolVar(&configInit, "init", false, "create default configuration file")
	configCmd.Flags().BoolVar(&configShow, "show", false, "show current configuration")
}

func runConfigCommand(cmd *cobra.Command, args []string) error {
	if configInit {
		return createDefaultConfig()
	}

	if configShow || (!configInit && !configShow) {
		return showConfig()
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

	// Check if config file already exists
	if _, err := os.Stat(configFile); err == nil {
		fmt.Printf("Configuration file already exists at: %s\n", configFile)
		return nil
	}

	// Create config directory
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return errors.Wrap(err, "failed to create config directory")
	}

	// Default configuration content
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

	// Write the default configuration with restricted permissions (owner read/write only)
	// Config files may contain sensitive paths and can execute arbitrary commands via tmux
	if err := os.WriteFile(configFile, []byte(defaultConfig), 0600); err != nil {
		return errors.Wrap(err, "failed to write config file")
	}

	fmt.Printf("Default configuration created at: %s\n", configFile)
	fmt.Println("Edit this file to customize your Rig settings.")

	return nil
}

func showConfig() error {
	cfg, err := config.Load()
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

func editConfig() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return errors.Wrap(err, "failed to get home directory")
	}

	configFile := filepath.Join(homeDir, ".config", "rig", "config.toml")

	// Create default config if it doesn't exist
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		fmt.Println("Config file does not exist, creating default...")
		if err := createDefaultConfig(); err != nil {
			return err
		}
	}

	// Get editor from environment, with fallbacks
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		// Check for common editors
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

	// Execute editor
	cmd := exec.Command(editor, configFile)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
