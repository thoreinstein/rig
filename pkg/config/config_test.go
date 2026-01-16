package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func TestExpandPath(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectTilde bool
	}{
		{
			name:        "empty path",
			input:       "",
			expectTilde: false,
		},
		{
			name:        "absolute path without tilde",
			input:       "/usr/local/bin",
			expectTilde: false,
		},
		{
			name:        "path with tilde",
			input:       "~/Documents",
			expectTilde: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := expandPath(tt.input)
			if err != nil {
				t.Fatalf("expandPath() error: %v", err)
			}

			if tt.expectTilde {
				if result[0] == '~' {
					t.Errorf("expandPath(%q) still contains ~: %q", tt.input, result)
				}
			} else if tt.input != "" {
				if result != tt.input {
					t.Errorf("expandPath(%q) = %q, want %q", tt.input, result, tt.input)
				}
			}
		})
	}
}

func TestLoad_WithDefaults(t *testing.T) {
	// Reset viper to clean state
	viper.Reset()

	// Don't read any config file - just use defaults
	config, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// Verify defaults are set
	if config.Jira.Enabled != true {
		t.Error("Expected jira.enabled to default to true")
	}
	if config.Jira.CliCommand != "acli" {
		t.Errorf("Expected jira.cli_command to default to 'acli', got %q", config.Jira.CliCommand)
	}
	if config.Git.BaseBranch != "" {
		t.Errorf("Expected git.base_branch to default to empty (auto-detect), got %q", config.Git.BaseBranch)
	}
	if config.Notes.DailyDir != "daily" {
		t.Errorf("Expected notes.daily_dir to default to 'daily', got %q", config.Notes.DailyDir)
	}

	// Verify default tmux windows are properly loaded (regression test for type mismatch bug)
	if len(config.Tmux.Windows) != 3 {
		t.Errorf("Expected 3 default tmux windows, got %d", len(config.Tmux.Windows))
	}
	if len(config.Tmux.Windows) >= 1 && config.Tmux.Windows[0].Name != "note" {
		t.Errorf("Expected first window name to be 'note', got %q", config.Tmux.Windows[0].Name)
	}
	if len(config.Tmux.Windows) >= 2 && config.Tmux.Windows[1].Name != "code" {
		t.Errorf("Expected second window name to be 'code', got %q", config.Tmux.Windows[1].Name)
	}
	if len(config.Tmux.Windows) >= 3 && config.Tmux.Windows[2].Name != "term" {
		t.Errorf("Expected third window name to be 'term', got %q", config.Tmux.Windows[2].Name)
	}
}

func TestLoad_WithConfigFile(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()

	configContent := `
notes:
  path: "/test/notes"
  daily_dir: "custom-daily"
  template_dir: "/test/templates"

git:
  base_branch: "develop"

jira:
  enabled: false
  cli_command: "custom-jira"

tmux:
  session_prefix: "test-"
`

	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Reset viper and configure it to read our test file
	viper.Reset()
	viper.SetConfigFile(configPath)
	if err := viper.ReadInConfig(); err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	config, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// Verify custom values
	if config.Notes.Path != "/test/notes" {
		t.Errorf("Notes.Path = %q, want %q", config.Notes.Path, "/test/notes")
	}
	if config.Notes.DailyDir != "custom-daily" {
		t.Errorf("Notes.DailyDir = %q, want %q", config.Notes.DailyDir, "custom-daily")
	}
	if config.Notes.TemplateDir != "/test/templates" {
		t.Errorf("Notes.TemplateDir = %q, want %q", config.Notes.TemplateDir, "/test/templates")
	}
	if config.Git.BaseBranch != "develop" {
		t.Errorf("Git.BaseBranch = %q, want %q", config.Git.BaseBranch, "develop")
	}
	if config.Jira.Enabled != false {
		t.Error("Jira.Enabled should be false")
	}
	if config.Jira.CliCommand != "custom-jira" {
		t.Errorf("Jira.CliCommand = %q, want %q", config.Jira.CliCommand, "custom-jira")
	}
	if config.Tmux.SessionPrefix != "test-" {
		t.Errorf("Tmux.SessionPrefix = %q, want %q", config.Tmux.SessionPrefix, "test-")
	}
}

func TestExpandPaths(t *testing.T) {
	homeDir, _ := os.UserHomeDir()

	config := &Config{
		Notes: NotesConfig{
			Path:        "~/notes",
			TemplateDir: "~/.config/sre/templates",
		},
		History: HistoryConfig{
			DatabasePath: "~/.histdb/history.db",
		},
	}

	err := expandPaths(config)
	if err != nil {
		t.Fatalf("expandPaths() error: %v", err)
	}

	expectedNotesPath := filepath.Join(homeDir, "notes")
	if config.Notes.Path != expectedNotesPath {
		t.Errorf("Notes.Path = %q, want %q", config.Notes.Path, expectedNotesPath)
	}

	expectedTemplatePath := filepath.Join(homeDir, ".config/sre/templates")
	if config.Notes.TemplateDir != expectedTemplatePath {
		t.Errorf("Notes.TemplateDir = %q, want %q", config.Notes.TemplateDir, expectedTemplatePath)
	}

	expectedHistoryPath := filepath.Join(homeDir, ".histdb/history.db")
	if config.History.DatabasePath != expectedHistoryPath {
		t.Errorf("History.DatabasePath = %q, want %q", config.History.DatabasePath, expectedHistoryPath)
	}
}

func TestExpandPaths_AbsolutePaths(t *testing.T) {
	config := &Config{
		Notes: NotesConfig{
			Path:        "/absolute/notes",
			TemplateDir: "/absolute/templates",
		},
		History: HistoryConfig{
			DatabasePath: "/absolute/history.db",
		},
	}

	err := expandPaths(config)
	if err != nil {
		t.Fatalf("expandPaths() error: %v", err)
	}

	// Absolute paths should remain unchanged
	if config.Notes.Path != "/absolute/notes" {
		t.Errorf("Notes.Path = %q, want %q", config.Notes.Path, "/absolute/notes")
	}
	if config.Notes.TemplateDir != "/absolute/templates" {
		t.Errorf("Notes.TemplateDir = %q, want %q", config.Notes.TemplateDir, "/absolute/templates")
	}
}

func TestConfigStructure(t *testing.T) {
	// Verify config struct has expected fields
	config := &Config{
		Notes: NotesConfig{
			Path:        "/test/notes",
			DailyDir:    "daily",
			TemplateDir: "/test/templates",
		},
		Git: GitConfig{
			BaseBranch: "main",
		},
		History: HistoryConfig{
			DatabasePath:   "/test/history.db",
			IgnorePatterns: []string{"ls", "cd"},
		},
		Jira: JiraConfig{
			Enabled:    true,
			CliCommand: "acli",
		},
		Tmux: TmuxConfig{
			SessionPrefix: "sre-",
			Windows: []TmuxWindow{
				{Name: "note", Command: "nvim {note_path}"},
			},
		},
	}

	// Verify fields are accessible
	if config.Notes.Path != "/test/notes" {
		t.Error("Notes.Path not set correctly")
	}
	if config.Git.BaseBranch != "main" {
		t.Error("Git.BaseBranch not set correctly")
	}
	if len(config.History.IgnorePatterns) != 2 {
		t.Error("History.IgnorePatterns not set correctly")
	}
	if !config.Jira.Enabled {
		t.Error("Jira.Enabled not set correctly")
	}
	if len(config.Tmux.Windows) != 1 {
		t.Error("Tmux.Windows not set correctly")
	}
}

func TestCheckSecurityWarnings_NoWarnings(t *testing.T) {
	// Config with no tokens - should produce no warnings
	config := &Config{}

	warnings := CheckSecurityWarnings(config)
	if len(warnings) != 0 {
		t.Errorf("Expected no warnings for empty config, got %d: %+v", len(warnings), warnings)
	}
}

func TestCheckSecurityWarnings_GitHubToken(t *testing.T) {
	// Config with GitHub token in config file (not via env var)
	config := &Config{
		GitHub: GitHubConfig{
			Token: "ghp_secret_token",
		},
	}

	// Ensure env var is not set
	t.Setenv("RIG_GITHUB_TOKEN", "")

	warnings := CheckSecurityWarnings(config)
	if len(warnings) != 1 {
		t.Errorf("Expected 1 warning for GitHub token in config, got %d", len(warnings))
	}
	if len(warnings) > 0 && warnings[0].Field != "github.token" {
		t.Errorf("Expected warning field 'github.token', got %q", warnings[0].Field)
	}
}

func TestCheckSecurityWarnings_GitHubTokenEnvVar(t *testing.T) {
	// Config with GitHub token, but env var is also set (should be silent)
	config := &Config{
		GitHub: GitHubConfig{
			Token: "ghp_secret_token",
		},
	}

	t.Setenv("RIG_GITHUB_TOKEN", "env_token")

	warnings := CheckSecurityWarnings(config)
	if len(warnings) != 0 {
		t.Errorf("Expected no warnings when env var is set, got %d: %+v", len(warnings), warnings)
	}
}

func TestCheckSecurityWarnings_AllTokens(t *testing.T) {
	// Config with all tokens set in config file
	config := &Config{
		GitHub: GitHubConfig{
			Token: "ghp_secret",
		},
		Jira: JiraConfig{
			Token: "jira_secret",
		},
		AI: AIConfig{
			APIKey: "ai_secret",
		},
	}

	// Ensure all env vars are not set
	t.Setenv("RIG_GITHUB_TOKEN", "")
	t.Setenv("RIG_JIRA_TOKEN", "")
	t.Setenv("JIRA_TOKEN", "")
	t.Setenv("RIG_AI_API_KEY", "")
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("GROQ_API_KEY", "")

	warnings := CheckSecurityWarnings(config)
	if len(warnings) != 3 {
		t.Errorf("Expected 3 warnings for all tokens, got %d: %+v", len(warnings), warnings)
	}
}

func TestValidateMergeMethod_Valid(t *testing.T) {
	validMethods := []string{"merge", "squash", "rebase", ""}
	for _, method := range validMethods {
		t.Run(method, func(t *testing.T) {
			err := ValidateMergeMethod(method)
			if err != nil {
				t.Errorf("ValidateMergeMethod(%q) returned error: %v", method, err)
			}
		})
	}
}

func TestValidateMergeMethod_Invalid(t *testing.T) {
	invalidMethods := []string{"fast-forward", "invalid", "SQUASH", "Merge"}
	for _, method := range invalidMethods {
		t.Run(method, func(t *testing.T) {
			err := ValidateMergeMethod(method)
			if err == nil {
				t.Errorf("ValidateMergeMethod(%q) should have returned error", method)
			}
		})
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name:    "empty config",
			config:  &Config{},
			wantErr: false,
		},
		{
			name: "valid merge method",
			config: &Config{
				GitHub: GitHubConfig{
					DefaultMergeMethod: "squash",
				},
			},
			wantErr: false,
		},
		{
			name: "invalid merge method",
			config: &Config{
				GitHub: GitHubConfig{
					DefaultMergeMethod: "invalid",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
