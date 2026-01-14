package config

import (
	"os"
	"path/filepath"

	"github.com/cockroachdb/errors"
	"github.com/spf13/viper"
)

// Config represents the application configuration
// Repository information is derived from git, not configuration
type Config struct {
	Notes    NotesConfig    `mapstructure:"notes"`
	Git      GitConfig      `mapstructure:"git"`
	Clone    CloneConfig    `mapstructure:"clone"`
	History  HistoryConfig  `mapstructure:"history"`
	Jira     JiraConfig     `mapstructure:"jira"`
	Tmux     TmuxConfig     `mapstructure:"tmux"`
	GitHub   GitHubConfig   `mapstructure:"github"`
	AI       AIConfig       `mapstructure:"ai"`
	Workflow WorkflowConfig `mapstructure:"workflow"`
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

// CloneConfig holds clone command configuration
type CloneConfig struct {
	BasePath string `mapstructure:"base_path"` // Base directory for clones (default: ~/src)
}

// HistoryConfig holds command history configuration
type HistoryConfig struct {
	DatabasePath   string   `mapstructure:"database_path"`
	IgnorePatterns []string `mapstructure:"ignore_patterns"`
}

// JiraConfig holds JIRA integration configuration
type JiraConfig struct {
	Enabled      bool              `mapstructure:"enabled"`
	Mode         string            `mapstructure:"mode"`          // "api" or "acli"
	BaseURL      string            `mapstructure:"base_url"`      // e.g., "https://your-domain.atlassian.net"
	Email        string            `mapstructure:"email"`         // User email for Basic Auth
	Token        string            `mapstructure:"token"`         // API token (JIRA_TOKEN env var takes precedence)
	CliCommand   string            `mapstructure:"cli_command"`   // For acli mode
	CustomFields map[string]string `mapstructure:"custom_fields"` // Map of field name to customfield_ID
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

// GitHubConfig holds GitHub integration configuration
type GitHubConfig struct {
	AuthMethod          string   `mapstructure:"auth_method"`          // "token", "oauth", "gh_cli"
	ClientID            string   `mapstructure:"client_id"`            // OAuth app client ID (for device flow)
	Token               string   `mapstructure:"token"`                // For token auth (RIG_GITHUB_TOKEN env var takes precedence)
	DefaultReviewers    []string `mapstructure:"default_reviewers"`    // Default PR reviewers
	DefaultMergeMethod  string   `mapstructure:"default_merge_method"` // "merge", "squash", "rebase"
	DeleteBranchOnMerge bool     `mapstructure:"delete_branch_on_merge"`
}

// AIConfig holds AI provider configuration
type AIConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	Provider string `mapstructure:"provider"` // "anthropic", "groq", "ollama"
	Model    string `mapstructure:"model"`    // e.g., "claude-3-5-sonnet-20241022"
	APIKey   string `mapstructure:"api_key"`  // Provider API key (env var takes precedence)
}

// WorkflowConfig holds PR workflow automation configuration
type WorkflowConfig struct {
	TransitionJira       bool `mapstructure:"transition_jira"`        // Auto-transition Jira on merge
	KillSession          bool `mapstructure:"kill_session"`           // Kill tmux session on merge
	QueueWorktreeCleanup bool `mapstructure:"queue_worktree_cleanup"` // Queue worktree for cleanup
}

// Load loads the configuration from file and environment variables
func Load() (*Config, error) {
	config := &Config{}

	// Set defaults
	setDefaults()

	// Unmarshal the config
	if err := viper.Unmarshal(config); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal config")
	}

	// Expand paths
	if err := expandPaths(config); err != nil {
		return nil, errors.Wrap(err, "failed to expand paths")
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
	viper.SetDefault("notes.template_dir", filepath.Join(homeDir, ".config", "rig", "templates"))

	// Git defaults (empty means auto-detect)
	viper.SetDefault("git.base_branch", "")

	// Clone defaults (empty means ~/src)
	viper.SetDefault("clone.base_path", "")

	// History defaults
	viper.SetDefault("history.database_path", filepath.Join(homeDir, ".histdb", "zsh-history.db"))
	viper.SetDefault("history.ignore_patterns", []string{"ls", "cd", "pwd", "clear"})

	// JIRA defaults
	viper.SetDefault("jira.enabled", true)
	viper.SetDefault("jira.mode", "api")
	viper.SetDefault("jira.base_url", "")
	viper.SetDefault("jira.email", "")
	viper.SetDefault("jira.token", "")
	viper.SetDefault("jira.cli_command", "acli")
	viper.SetDefault("jira.custom_fields", map[string]string{})

	// Tmux defaults
	viper.SetDefault("tmux.session_prefix", "")
	viper.SetDefault("tmux.windows", []TmuxWindow{
		{Name: "note", Command: "nvim {note_path}"},
		{Name: "code", Command: "nvim", WorkingDir: "{worktree_path}"},
		{Name: "term", WorkingDir: "{worktree_path}"},
	})

	// GitHub defaults
	viper.SetDefault("github.auth_method", "gh_cli") // Prefer gh CLI auth
	viper.SetDefault("github.client_id", "")         // OAuth app client ID for device flow
	viper.SetDefault("github.token", "")
	viper.SetDefault("github.default_reviewers", []string{})
	viper.SetDefault("github.default_merge_method", "squash")
	viper.SetDefault("github.delete_branch_on_merge", true)

	// AI defaults
	viper.SetDefault("ai.enabled", true)
	viper.SetDefault("ai.provider", "anthropic")
	viper.SetDefault("ai.model", "claude-sonnet-4-20250514")
	viper.SetDefault("ai.api_key", "")

	// Workflow defaults
	viper.SetDefault("workflow.transition_jira", true)
	viper.SetDefault("workflow.kill_session", true)
	viper.SetDefault("workflow.queue_worktree_cleanup", true)
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

	config.Clone.BasePath, err = expandPath(config.Clone.BasePath)
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
