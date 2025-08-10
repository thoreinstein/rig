package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config represents the application configuration
type Config struct {
	Vault      VaultConfig      `mapstructure:"vault"`
	Repository RepositoryConfig `mapstructure:"repository"`
	History    HistoryConfig    `mapstructure:"history"`
	Jira       JiraConfig       `mapstructure:"jira"`
	Tmux       TmuxConfig       `mapstructure:"tmux"`
}

// VaultConfig holds Obsidian vault configuration
type VaultConfig struct {
	Path         string `mapstructure:"path"`
	TemplatesDir string `mapstructure:"templates_dir"`
	AreasDir     string `mapstructure:"areas_dir"`
	DailyDir     string `mapstructure:"daily_dir"`
}

// RepositoryConfig holds Git repository configuration
type RepositoryConfig struct {
	Owner      string `mapstructure:"owner"`
	Name       string `mapstructure:"name"`
	BasePath   string `mapstructure:"base_path"`
	BaseBranch string `mapstructure:"base_branch"`
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
	homeDir, _ := os.UserHomeDir()
	
	// Vault defaults
	viper.SetDefault("vault.path", filepath.Join(homeDir, "Documents", "Second Brain"))
	viper.SetDefault("vault.templates_dir", "templates")
	viper.SetDefault("vault.areas_dir", "Areas/Ping Identity")
	viper.SetDefault("vault.daily_dir", "Daily")
	
	// Repository defaults
	viper.SetDefault("repository.owner", "test")
	viper.SetDefault("repository.name", "test")
	viper.SetDefault("repository.base_path", filepath.Join(homeDir, "src"))
	viper.SetDefault("repository.base_branch", "main")
	
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
	
	config.Vault.Path, err = expandPath(config.Vault.Path)
	if err != nil {
		return err
	}
	
	config.Repository.BasePath, err = expandPath(config.Repository.BasePath)
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

// GetRepositoryPath returns the full path to the repository
func (c *Config) GetRepositoryPath() string {
	return filepath.Join(c.Repository.BasePath, c.Repository.Owner, c.Repository.Name)
}

// GetWorktreePath returns the path for a specific ticket's worktree
func (c *Config) GetWorktreePath(ticketType, ticket string) string {
	return filepath.Join(c.GetRepositoryPath(), ticketType, ticket)
}

// GetNotePath returns the path for a ticket's Obsidian note
func (c *Config) GetNotePath(ticketType, ticket string) string {
	var subdir string
	switch ticketType {
	case "incident":
		subdir = "Incidents"
	default:
		subdir = "Jira"
	}
	
	return filepath.Join(c.Vault.Path, c.Vault.AreasDir, subdir, ticketType, ticket+".md")
}