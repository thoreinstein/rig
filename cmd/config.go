package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"thoreinstein.com/sre/pkg/config"
)

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage SRE CLI configuration",
	Long: `Display and manage the SRE CLI configuration.

This command shows the current configuration values and can help with 
initial setup by creating a default configuration file.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runConfigCommand(cmd, args)
	},
}

var (
	configInit bool
	configShow bool
)

func init() {
	rootCmd.AddCommand(configCmd)
	
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
		return fmt.Errorf("failed to get home directory: %w", err)
	}
	
	configDir := filepath.Join(homeDir, ".config", "sre")
	configFile := filepath.Join(configDir, "config.yaml")
	
	// Check if config file already exists
	if _, err := os.Stat(configFile); err == nil {
		fmt.Printf("Configuration file already exists at: %s\n", configFile)
		return nil
	}
	
	// Create config directory
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	
	// Default configuration content
	defaultConfig := `# SRE CLI Configuration
vault:
  path: "~/Documents/Second Brain"
  templates_dir: "templates"
  areas_dir: "Areas/Ping Identity"
  daily_dir: "Daily"

repository:
  owner: "test"
  name: "test"
  base_path: "~/src"
  base_branch: "main"

history:
  database_path: "~/.histdb/zsh-history.db"
  ignore_patterns: ["ls", "cd", "pwd", "clear"]

jira:
  enabled: true
  cli_command: "acli"

tmux:
  session_prefix: ""
  windows:
    - name: "note"
      command: "nvim {note_path}"
    - name: "code"
      command: "nvim"
      working_dir: "{worktree_path}"
    - name: "term"
      working_dir: "{worktree_path}"
`
	
	// Write the default configuration
	if err := os.WriteFile(configFile, []byte(defaultConfig), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}
	
	fmt.Printf("Default configuration created at: %s\n", configFile)
	fmt.Println("Edit this file to customize your SRE CLI settings.")
	
	return nil
}

func showConfig() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}
	
	fmt.Println("Current SRE CLI Configuration:")
	fmt.Println("==============================")
	
	fmt.Printf("Vault Path:          %s\n", cfg.Vault.Path)
	fmt.Printf("Repository:          %s/%s\n", cfg.Repository.Owner, cfg.Repository.Name)
	fmt.Printf("Repository Path:     %s\n", cfg.GetRepositoryPath())
	fmt.Printf("Base Branch:         %s\n", cfg.Repository.BaseBranch)
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