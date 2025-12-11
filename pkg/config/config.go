package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config represents the application configuration
// Repository information is derived from git, not configuration
type Config struct {
	Notes   NotesConfig   `mapstructure:"notes"`
	Git     GitConfig     `mapstructure:"git"`
	History HistoryConfig `mapstructure:"history"`
	Jira    JiraConfig    `mapstructure:"jira"`
	Tmux    TmuxConfig    `mapstructure:"tmux"`
}

// NotesConfig holds markdown notes configuration
type NotesConfig struct {
	Path        string `mapstructure:"path"`         // Base directory for notes
	DailyDir    string `mapstructure:"daily_dir"`    // Subdirectory for daily notes
	TemplateDir string `mapstructure:"template_dir"` // Optional user template directory
}

// GitConfig holds optional git configuration overrides
type GitConfig struct {
	BaseBranch string `mapstructure:"base_branch"` // Optional override for default branch
}

// HistoryConfig holds command history configuration
type HistoryConfig struct {
	DatabasePath   string   `mapstructure:"database_path"`
	IgnorePatterns []string `mapstructure:"ignore_patterns"`
}

// JiraConfig holds JIRA integration configuration
type JiraConfig struct {
	Enabled    bool   `mapstructure:"enabled"`
	CliCommand string `mapstructure:"cli_command"`
}

// TmuxWindow represents a tmux window configuration
type TmuxWindow struct {
	Name       string `mapstructure:"name"`
	Command    string `mapstructure:"command"`
	WorkingDir string `mapstructure:"working_dir"`
}

// TmuxConfig holds Tmux session configuration
type TmuxConfig struct {
	SessionPrefix string       `mapstructure:"session_prefix"`
	Windows       []TmuxWindow `mapstructure:"windows"`
}

// Load loads the configuration from file and environment variables
func Load() (*Config, error) {
	config := &Config{}

	// Set defaults
	setDefaults()

	// Unmarshal the config
	if err := viper.Unmarshal(config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Expand paths
	if err := expandPaths(config); err != nil {
		return nil, fmt.Errorf("failed to expand paths: %w", err)
	}

	return config, nil
}

// setDefaults sets default configuration values
func setDefaults() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Fall back to current directory if home dir can't be determined
		homeDir = "."
	}

	// Notes defaults
	viper.SetDefault("notes.path", filepath.Join(homeDir, "Documents", "Notes"))
	viper.SetDefault("notes.daily_dir", "daily")
	viper.SetDefault("notes.template_dir", filepath.Join(homeDir, ".config", "sre", "templates"))

	// Git defaults (empty means auto-detect)
	viper.SetDefault("git.base_branch", "")

	// History defaults
	viper.SetDefault("history.database_path", filepath.Join(homeDir, ".histdb", "zsh-history.db"))
	viper.SetDefault("history.ignore_patterns", []string{"ls", "cd", "pwd", "clear"})

	// JIRA defaults
	viper.SetDefault("jira.enabled", true)
	viper.SetDefault("jira.cli_command", "acli")

	// Tmux defaults
	viper.SetDefault("tmux.session_prefix", "")
	viper.SetDefault("tmux.windows", []map[string]string{
		{"name": "note", "command": "nvim {note_path}"},
		{"name": "code", "command": "nvim", "working_dir": "{worktree_path}"},
		{"name": "term", "working_dir": "{worktree_path}"},
	})
}

// expandPaths expands ~ and environment variables in paths
func expandPaths(config *Config) error {
	var err error

	config.Notes.Path, err = expandPath(config.Notes.Path)
	if err != nil {
		return err
	}

	config.Notes.TemplateDir, err = expandPath(config.Notes.TemplateDir)
	if err != nil {
		return err
	}

	config.History.DatabasePath, err = expandPath(config.History.DatabasePath)
	if err != nil {
		return err
	}

	return nil
}

// expandPath expands ~ to home directory
func expandPath(path string) (string, error) {
	if len(path) == 0 || path[0] != '~' {
		return path, nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(homeDir, path[1:]), nil
}
