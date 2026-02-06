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
	Notes     NotesConfig     `mapstructure:"notes"`
	Git       GitConfig       `mapstructure:"git"`
	Clone     CloneConfig     `mapstructure:"clone"`
	History   HistoryConfig   `mapstructure:"history"`
	Jira      JiraConfig      `mapstructure:"jira"`
	Beads     BeadsConfig     `mapstructure:"beads"`
	Tmux      TmuxConfig      `mapstructure:"tmux"`
	GitHub    GitHubConfig    `mapstructure:"github"`
	AI        AIConfig        `mapstructure:"ai"`
	Workflow  WorkflowConfig  `mapstructure:"workflow"`
	Discovery DiscoveryConfig `mapstructure:"discovery"`
}

// NotesConfig holds markdown notes configuration
type NotesConfig struct {
	Path        string `mapstructure:"path"`         // Base directory for notes
	DailyDir    string `mapstructure:"daily_dir"`    // Subdirectory for daily notes
	TemplateDir string `mapstructure:"template_dir"` // Optional user template directory
}

// DiscoveryConfig holds project discovery configuration
type DiscoveryConfig struct {
	SearchPaths []string `mapstructure:"search_paths"` // Directories to scan for projects
	MaxDepth    int      `mapstructure:"max_depth"`    // Max depth to scan (default: 3)
	CachePath   string   `mapstructure:"cache_path"`   // Path to project cache file
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

// BeadsConfig holds beads issue tracking configuration
type BeadsConfig struct {
	Enabled    bool   `mapstructure:"enabled"`
	CliCommand string `mapstructure:"cli_command"` // Command to run (default: "bd")
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
	Model    string `mapstructure:"model"`    // e.g., "claude-sonnet-4-20250514"
	APIKey   string `mapstructure:"api_key"`  // Provider API key (env var takes precedence)
	Endpoint string `mapstructure:"endpoint"` // Custom endpoint URL (e.g., for Ollama: http://localhost:11434)

	// Per-provider default models (used when Model is empty)
	AnthropicModel string `mapstructure:"anthropic_model"` // Default: claude-sonnet-4-20250514
	GroqModel      string `mapstructure:"groq_model"`      // Default: llama-3.3-70b-versatile
	OllamaModel    string `mapstructure:"ollama_model"`    // Default: llama3.2
	OllamaEndpoint string `mapstructure:"ollama_endpoint"` // Default: http://localhost:11434
	GeminiModel    string `mapstructure:"gemini_model"`
	GeminiCommand  string `mapstructure:"gemini_command"` // Default: gemini
}

// WorkflowConfig holds PR workflow automation configuration
type WorkflowConfig struct {
	TransitionJira       bool `mapstructure:"transition_jira"`        // Auto-transition Jira on merge
	KillSession          bool `mapstructure:"kill_session"`           // Kill tmux session on merge
	QueueWorktreeCleanup bool `mapstructure:"queue_worktree_cleanup"` // Queue worktree for cleanup
}

// SecurityWarning represents a configuration security issue
type SecurityWarning struct {
	Field   string
	Message string
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

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, errors.Wrap(err, "config validation failed")
	}

	return config, nil
}

// CheckSecurityWarnings returns warnings for insecure configuration practices.
// Call this when loading config to warn users about tokens stored in config files.
func CheckSecurityWarnings(config *Config) []SecurityWarning {
	var warnings []SecurityWarning

	// Check for tokens in config file (should use environment variables instead)
	// Consider checking viper.InConfig("github.token") if possible to warn whenever
	// the secret exists in a physical file, regardless of environment overrides.
	if config.GitHub.Token != "" && os.Getenv("RIG_GITHUB_TOKEN") == "" {
		warnings = append(warnings, SecurityWarning{
			Field:   "github.token",
			Message: "GitHub token is set in config file. For security, use RIG_GITHUB_TOKEN environment variable or 'gh auth login' instead.",
		})
	}

	if config.Jira.Token != "" && os.Getenv("RIG_JIRA_TOKEN") == "" && os.Getenv("JIRA_TOKEN") == "" {
		warnings = append(warnings, SecurityWarning{
			Field:   "jira.token",
			Message: "Jira token is set in config file. For security, use RIG_JIRA_TOKEN or JIRA_TOKEN environment variable instead.",
		})
	}

	if config.AI.APIKey != "" && os.Getenv("RIG_AI_API_KEY") == "" &&
		os.Getenv("ANTHROPIC_API_KEY") == "" && os.Getenv("GROQ_API_KEY") == "" {
		warnings = append(warnings, SecurityWarning{
			Field:   "ai.api_key",
			Message: "AI API key is set in config file. For security, use environment variables (ANTHROPIC_API_KEY, GROQ_API_KEY, or RIG_AI_API_KEY) instead.",
		})
	}

	return warnings
}

// ValidMergeMethods is the list of supported GitHub merge methods.
var ValidMergeMethods = []string{"merge", "squash", "rebase"}

// ValidateMergeMethod validates that a merge method is supported.
// Returns the method if valid, or an error if not.
func ValidateMergeMethod(method string) error {
	if method == "" {
		return nil // Empty is allowed, will use default
	}
	for _, valid := range ValidMergeMethods {
		if method == valid {
			return nil
		}
	}
	return errors.Newf("invalid merge method %q: must be one of: merge, squash, rebase", method)
}

// Validate validates the configuration and returns any validation errors.
func (c *Config) Validate() error {
	if err := ValidateMergeMethod(c.GitHub.DefaultMergeMethod); err != nil {
		return errors.Wrap(err, "github.default_merge_method")
	}
	return nil
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

	// Beads defaults
	viper.SetDefault("beads.enabled", true)
	viper.SetDefault("beads.cli_command", "bd")

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
	viper.SetDefault("ai.model", "") // Empty means use per-provider default
	viper.SetDefault("ai.api_key", "")
	viper.SetDefault("ai.endpoint", "") // Empty means use provider default

	// Per-provider AI model defaults (configurable)
	viper.SetDefault("ai.anthropic_model", "claude-sonnet-4-20250514")
	viper.SetDefault("ai.groq_model", "llama-3.3-70b-versatile")
	viper.SetDefault("ai.ollama_model", "llama3.2")
	viper.SetDefault("ai.ollama_endpoint", "http://localhost:11434")
	viper.SetDefault("ai.gemini_model", "")
	viper.SetDefault("ai.gemini_command", "gemini")

	// Workflow defaults
	viper.SetDefault("workflow.transition_jira", true)
	viper.SetDefault("workflow.kill_session", true)
	viper.SetDefault("workflow.queue_worktree_cleanup", true)

	// Discovery defaults
	viper.SetDefault("discovery.search_paths", []string{filepath.Join(homeDir, "src")})
	viper.SetDefault("discovery.max_depth", 3)
	viper.SetDefault("discovery.cache_path", filepath.Join(homeDir, ".cache", "rig", "projects.json"))
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

	for i, path := range config.Discovery.SearchPaths {
		config.Discovery.SearchPaths[i], err = expandPath(path)
		if err != nil {
			return err
		}
	}

	config.Discovery.CachePath, err = expandPath(config.Discovery.CachePath)
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
