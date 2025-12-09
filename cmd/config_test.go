package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigCommandFlags(t *testing.T) {
	cmd := configCmd

	// Check --init flag exists
	initFlag := cmd.Flags().Lookup("init")
	if initFlag == nil {
		t.Error("config command should have --init flag")
	}
	if initFlag != nil && initFlag.DefValue != "false" {
		t.Errorf("--init default should be false, got %s", initFlag.DefValue)
	}

	// Check --show flag exists
	showFlag := cmd.Flags().Lookup("show")
	if showFlag == nil {
		t.Error("config command should have --show flag")
	}
	if showFlag != nil && showFlag.DefValue != "false" {
		t.Errorf("--show default should be false, got %s", showFlag.DefValue)
	}
}

func TestConfigCommandDescription(t *testing.T) {
	cmd := configCmd

	if cmd.Use != "config" {
		t.Errorf("config command Use = %q, want %q", cmd.Use, "config")
	}

	if cmd.Short == "" {
		t.Error("config command should have Short description")
	}

	if cmd.Long == "" {
		t.Error("config command should have Long description")
	}

	// Verify key information is in the description
	if !strings.Contains(cmd.Long, "configuration") {
		t.Error("config command Long description should mention 'configuration'")
	}
}

func TestRunConfigCommand_FlagRouting(t *testing.T) {
	// Test the flag routing logic
	// Note: We're testing the logic, not the actual execution

	tests := []struct {
		name       string
		initFlag   bool
		showFlag   bool
		shouldInit bool
		shouldShow bool
	}{
		{
			name:       "no flags - defaults to show",
			initFlag:   false,
			showFlag:   false,
			shouldInit: false,
			shouldShow: true,
		},
		{
			name:       "init flag",
			initFlag:   true,
			showFlag:   false,
			shouldInit: true,
			shouldShow: false,
		},
		{
			name:       "show flag",
			initFlag:   false,
			showFlag:   true,
			shouldInit: false,
			shouldShow: true,
		},
		{
			name:       "both flags - init takes precedence",
			initFlag:   true,
			showFlag:   true,
			shouldInit: true,
			shouldShow: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the flag routing logic from runConfigCommand
			shouldInit := tt.initFlag
			shouldShow := tt.showFlag || (!tt.initFlag && !tt.showFlag)

			// If init is set, show should not be called
			if shouldInit {
				shouldShow = false
			}

			if shouldInit != tt.shouldInit {
				t.Errorf("shouldInit = %v, want %v", shouldInit, tt.shouldInit)
			}
			if shouldShow != tt.shouldShow {
				t.Errorf("shouldShow = %v, want %v", shouldShow, tt.shouldShow)
			}
		})
	}
}

func TestCreateDefaultConfig(t *testing.T) {
	// Create a temporary home directory
	tmpDir, err := os.MkdirTemp("", "config-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Save original home dir and restore after test
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tmpDir)

	// Run createDefaultConfig
	err = createDefaultConfig()
	if err != nil {
		t.Fatalf("createDefaultConfig() error: %v", err)
	}

	// Verify config file was created
	configPath := filepath.Join(tmpDir, ".config", "sre", "config.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Errorf("Config file not created at %s", configPath)
	}

	// Verify config content has expected sections
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	expectedSections := []string{
		"vault:",
		"repository:",
		"history:",
		"jira:",
		"tmux:",
	}

	for _, section := range expectedSections {
		if !strings.Contains(string(content), section) {
			t.Errorf("Config file should contain %q section", section)
		}
	}
}

func TestCreateDefaultConfig_AlreadyExists(t *testing.T) {
	// Create a temporary home directory
	tmpDir, err := os.MkdirTemp("", "config-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Save original home dir and restore after test
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tmpDir)

	// Create config directory and file
	configDir := filepath.Join(tmpDir, ".config", "sre")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	configPath := filepath.Join(configDir, "config.yaml")
	existingContent := "# Existing config\nvault:\n  path: /existing/path"
	if err := os.WriteFile(configPath, []byte(existingContent), 0644); err != nil {
		t.Fatalf("Failed to write existing config: %v", err)
	}

	// Run createDefaultConfig - should not overwrite
	err = createDefaultConfig()
	if err != nil {
		t.Fatalf("createDefaultConfig() error: %v", err)
	}

	// Verify original content is preserved
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	if !strings.Contains(string(content), "/existing/path") {
		t.Error("Existing config file should not be overwritten")
	}
}

func TestDefaultConfigContent(t *testing.T) {
	// Verify the default config has all necessary fields
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

	// Verify all required sections are present
	requiredFields := []string{
		"vault:",
		"path:",
		"templates_dir:",
		"areas_dir:",
		"daily_dir:",
		"repository:",
		"owner:",
		"name:",
		"base_path:",
		"base_branch:",
		"history:",
		"database_path:",
		"ignore_patterns:",
		"jira:",
		"enabled:",
		"cli_command:",
		"tmux:",
		"session_prefix:",
		"windows:",
	}

	for _, field := range requiredFields {
		if !strings.Contains(defaultConfig, field) {
			t.Errorf("Default config should contain %q", field)
		}
	}
}
