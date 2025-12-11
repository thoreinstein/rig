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
