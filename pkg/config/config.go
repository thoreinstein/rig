package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config represents the application configuration
type Config struct {
	Vault        VaultConfig                  `mapstructure:"vault"`
	Repository   RepositoryConfig             `mapstructure:"repository"`    // Legacy single-repo config
	Repositories map[string]RepositoryConfig  `mapstructure:"repositories"`  // Multi-repo config
	DefaultRepo  string                       `mapstructure:"default_repo"`  // Default repo name for multi-repo
	TicketTypes  map[string]TicketTypeConfig  `mapstructure:"ticket_types"`  // Maps ticket prefix to repo
	History      HistoryConfig                `mapstructure:"history"`
	Jira         JiraConfig                   `mapstructure:"jira"`
	Tmux         TmuxConfig                   `mapstructure:"tmux"`
}

// TicketTypeConfig maps a ticket type to a repository and vault subdirectory
type TicketTypeConfig struct {
	Repo        string `mapstructure:"repo"`         // References key in Repositories map
	VaultSubdir string `mapstructure:"vault_subdir"` // e.g., "Jira", "Incidents", "Hacks"
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

	// Expand legacy single repo path
	config.Repository.BasePath, err = expandPath(config.Repository.BasePath)
	if err != nil {
		return err
	}

	// Expand multi-repo paths
	for name, repo := range config.Repositories {
		repo.BasePath, err = expandPath(repo.BasePath)
		if err != nil {
			return err
		}
		config.Repositories[name] = repo
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
	subdir := c.GetVaultSubdir(ticketType)
	return filepath.Join(c.Vault.Path, c.Vault.AreasDir, subdir, ticketType, ticket+".md")
}

// GetVaultSubdir returns the vault subdirectory for a ticket type
func (c *Config) GetVaultSubdir(ticketType string) string {
	// Check ticket_types config first
	if typeConfig, ok := c.TicketTypes[ticketType]; ok && typeConfig.VaultSubdir != "" {
		return typeConfig.VaultSubdir
	}

	// Fall back to default mapping
	switch ticketType {
	case "incident":
		return "Incidents"
	case "hack":
		return "Hacks"
	default:
		return "Jira"
	}
}

// IsMultiRepo returns true if multi-repo configuration is being used
func (c *Config) IsMultiRepo() bool {
	return len(c.Repositories) > 0
}

// GetRepoForTicketType returns the repository config for a given ticket type
// Falls back to default repo or legacy single repo config
func (c *Config) GetRepoForTicketType(ticketType string) *RepositoryConfig {
	// If multi-repo is configured
	if c.IsMultiRepo() {
		// Check if ticket type has a specific repo mapping
		if typeConfig, ok := c.TicketTypes[ticketType]; ok && typeConfig.Repo != "" {
			if repo, ok := c.Repositories[typeConfig.Repo]; ok {
				return &repo
			}
		}

		// Fall back to default repo
		if c.DefaultRepo != "" {
			if repo, ok := c.Repositories[c.DefaultRepo]; ok {
				return &repo
			}
		}

		// Fall back to first repo in map
		for _, repo := range c.Repositories {
			repoCopy := repo
			return &repoCopy
		}
	}

	// Fall back to legacy single repo config
	return &c.Repository
}

// GetRepoByName returns a repository config by name
func (c *Config) GetRepoByName(name string) (*RepositoryConfig, error) {
	if c.IsMultiRepo() {
		if repo, ok := c.Repositories[name]; ok {
			return &repo, nil
		}
		return nil, fmt.Errorf("repository %q not found in configuration", name)
	}

	// If not multi-repo, return legacy config if name matches or is empty
	if name == "" || name == c.Repository.Name {
		return &c.Repository, nil
	}
	return nil, fmt.Errorf("repository %q not found in configuration", name)
}

// GetDefaultRepo returns the default repository config
func (c *Config) GetDefaultRepo() *RepositoryConfig {
	if c.IsMultiRepo() {
		if c.DefaultRepo != "" {
			if repo, ok := c.Repositories[c.DefaultRepo]; ok {
				return &repo
			}
		}
		// Return first repo if no default set
		for _, repo := range c.Repositories {
			repoCopy := repo
			return &repoCopy
		}
	}
	return &c.Repository
}

// GetAllRepos returns all configured repositories
func (c *Config) GetAllRepos() map[string]*RepositoryConfig {
	repos := make(map[string]*RepositoryConfig)

	if c.IsMultiRepo() {
		for name, repo := range c.Repositories {
			repoCopy := repo
			repos[name] = &repoCopy
		}
	} else {
		// Legacy single repo - use name as key, or "default" if empty
		key := c.Repository.Name
		if key == "" {
			key = "default"
		}
		repos[key] = &c.Repository
	}

	return repos
}

// GetRepositoryPathForRepo returns the full path to a specific repository
func (c *Config) GetRepositoryPathForRepo(repo *RepositoryConfig) string {
	return filepath.Join(repo.BasePath, repo.Owner, repo.Name)
}