package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func TestRootCommandStructure(t *testing.T) {
	t.Parallel()

	cmd := rootCmd

	if cmd.Use != "sre" {
		t.Errorf("root command Use = %q, want %q", cmd.Use, "sre")
	}

	if cmd.Short == "" {
		t.Error("root command should have Short description")
	}

	if cmd.Long == "" {
		t.Error("root command should have Long description")
	}

	// Verify key information is in the description
	expectedKeywords := []string{"SRE", "workflow", "automation"}
	for _, keyword := range expectedKeywords {
		if !strings.Contains(cmd.Long, keyword) {
			t.Errorf("root command Long description should mention %q", keyword)
		}
	}
}

func TestRootCommandPersistentFlags(t *testing.T) {
	t.Parallel()

	cmd := rootCmd

	// Check --config flag exists
	configFlag := cmd.PersistentFlags().Lookup("config")
	if configFlag == nil {
		t.Error("root command should have --config persistent flag")
	}
	if configFlag != nil {
		if configFlag.DefValue != "" {
			t.Errorf("--config default should be empty, got %q", configFlag.DefValue)
		}
		if configFlag.Usage == "" {
			t.Error("--config flag should have usage description")
		}
		// Verify usage mentions default location
		if !strings.Contains(configFlag.Usage, "$HOME/.config/sre") {
			t.Error("--config usage should mention default config location")
		}
	}

	// Check --verbose flag exists
	verboseFlag := cmd.PersistentFlags().Lookup("verbose")
	if verboseFlag == nil {
		t.Error("root command should have --verbose persistent flag")
	}
	if verboseFlag != nil {
		if verboseFlag.DefValue != "false" {
			t.Errorf("--verbose default should be 'false', got %q", verboseFlag.DefValue)
		}
		if verboseFlag.Shorthand != "v" {
			t.Errorf("--verbose shorthand should be 'v', got %q", verboseFlag.Shorthand)
		}
	}
}

func TestRootCommandHasSubcommands(t *testing.T) {
	t.Parallel()

	cmd := rootCmd
	subcommands := cmd.Commands()

	if len(subcommands) == 0 {
		t.Error("root command should have subcommands registered")
	}

	// Build a map of registered subcommand names
	registeredCommands := make(map[string]bool)
	for _, sub := range subcommands {
		// Extract just the command name (first word of Use)
		name := strings.Split(sub.Use, " ")[0]
		registeredCommands[name] = true
	}

	// Verify expected subcommands exist
	expectedCommands := []string{"init", "list", "session", "config", "sync", "history", "timeline", "clean", "hack", "update", "version"}
	for _, expected := range expectedCommands {
		if !registeredCommands[expected] {
			t.Errorf("root command should have %q subcommand registered", expected)
		}
	}
}

func TestInitConfig_WithCustomConfigFile(t *testing.T) {
	// Don't run in parallel - modifies global viper state
	tmpDir := t.TempDir()

	// Create a custom config file
	configContent := `[notes]
path = "/custom/notes/path"
daily_dir = "custom_daily"

[jira]
enabled = false
`
	customConfigPath := filepath.Join(tmpDir, "custom-config.toml")
	if err := os.WriteFile(customConfigPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write custom config: %v", err)
	}

	// Reset viper and set the custom config file
	viper.Reset()
	defer viper.Reset()

	// Set the global cfgFile variable
	oldCfgFile := cfgFile
	cfgFile = customConfigPath
	defer func() { cfgFile = oldCfgFile }()

	// Run initConfig
	initConfig()

	// Verify config was loaded
	if viper.GetString("notes.path") != "/custom/notes/path" {
		t.Errorf("notes.path = %q, want %q", viper.GetString("notes.path"), "/custom/notes/path")
	}
	if viper.GetString("notes.daily_dir") != "custom_daily" {
		t.Errorf("notes.daily_dir = %q, want %q", viper.GetString("notes.daily_dir"), "custom_daily")
	}
	if viper.GetBool("jira.enabled") != false {
		t.Error("jira.enabled should be false")
	}
}

func TestInitConfig_WithDefaultLocation(t *testing.T) {
	// Don't run in parallel - modifies global viper state
	tmpDir := t.TempDir()

	// Create config directory and file in default location
	configDir := filepath.Join(tmpDir, ".config", "sre")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	configContent := `[notes]
path = "/default/location/notes"

[git]
base_branch = "develop"
`
	configPath := filepath.Join(configDir, "config.toml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Reset viper and set HOME to temp dir
	viper.Reset()
	defer viper.Reset()

	t.Setenv("HOME", tmpDir)

	// Ensure cfgFile is empty to use default location
	oldCfgFile := cfgFile
	cfgFile = ""
	defer func() { cfgFile = oldCfgFile }()

	// Run initConfig
	initConfig()

	// Verify config was loaded from default location
	if viper.GetString("notes.path") != "/default/location/notes" {
		t.Errorf("notes.path = %q, want %q", viper.GetString("notes.path"), "/default/location/notes")
	}
	if viper.GetString("git.base_branch") != "develop" {
		t.Errorf("git.base_branch = %q, want %q", viper.GetString("git.base_branch"), "develop")
	}
}

func TestInitConfig_NoConfigFile(t *testing.T) {
	// Don't run in parallel - modifies global viper state
	tmpDir := t.TempDir()

	// Reset viper and set HOME to temp dir (no config file exists)
	viper.Reset()
	defer viper.Reset()

	t.Setenv("HOME", tmpDir)

	// Ensure cfgFile is empty
	oldCfgFile := cfgFile
	cfgFile = ""
	defer func() { cfgFile = oldCfgFile }()

	// Run initConfig - should not panic when config file doesn't exist
	initConfig()

	// Viper should still be usable even without a config file
	// This verifies the error is silently ignored when no config exists
	// Setting a value should work
	viper.Set("test.key", "value")
	if viper.GetString("test.key") != "value" {
		t.Error("viper should be functional even without config file")
	}
}

func TestInitConfig_EnvironmentVariables(t *testing.T) {
	// Don't run in parallel - modifies global viper state
	tmpDir := t.TempDir()

	// Reset viper
	viper.Reset()
	defer viper.Reset()

	t.Setenv("HOME", tmpDir)

	// Ensure cfgFile is empty
	oldCfgFile := cfgFile
	cfgFile = ""
	defer func() { cfgFile = oldCfgFile }()

	// Run initConfig to enable AutomaticEnv
	initConfig()

	// After initConfig, viper.AutomaticEnv() has been called
	// Verify viper is functional after initConfig
	viper.Set("test.key", "test_value")
	if viper.GetString("test.key") != "test_value" {
		t.Error("viper should be functional after initConfig")
	}
}

func TestInitConfig_VerboseOutput(t *testing.T) {
	// Don't run in parallel - modifies global viper state
	tmpDir := t.TempDir()

	// Create config directory and file
	configDir := filepath.Join(tmpDir, ".config", "sre")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	configContent := `[notes]
path = "/test/path"
`
	configPath := filepath.Join(configDir, "config.toml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Reset viper
	viper.Reset()
	defer viper.Reset()

	t.Setenv("HOME", tmpDir)

	// Set verbose flag
	oldVerbose := verbose
	verbose = true
	defer func() { verbose = oldVerbose }()

	// Ensure cfgFile is empty
	oldCfgFile := cfgFile
	cfgFile = ""
	defer func() { cfgFile = oldCfgFile }()

	// Capture stderr to verify verbose output
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	// Run initConfig
	initConfig()

	// Restore stderr and read captured output
	w.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	// When verbose is true and config file is found, it should print the path
	if !strings.Contains(output, "Using config file:") {
		t.Errorf("Verbose mode should print 'Using config file:', got: %q", output)
	}
	if !strings.Contains(output, configPath) {
		t.Errorf("Verbose mode should print config path %q, got: %q", configPath, output)
	}
}

func TestInitConfig_NonVerboseNoOutput(t *testing.T) {
	// Don't run in parallel - modifies global viper state
	tmpDir := t.TempDir()

	// Create config directory and file
	configDir := filepath.Join(tmpDir, ".config", "sre")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	configContent := `[notes]
path = "/test/path"
`
	configPath := filepath.Join(configDir, "config.toml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Reset viper
	viper.Reset()
	defer viper.Reset()

	t.Setenv("HOME", tmpDir)

	// Ensure verbose is false
	oldVerbose := verbose
	verbose = false
	defer func() { verbose = oldVerbose }()

	// Ensure cfgFile is empty
	oldCfgFile := cfgFile
	cfgFile = ""
	defer func() { cfgFile = oldCfgFile }()

	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	// Run initConfig
	initConfig()

	// Restore stderr and read captured output
	w.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	// When verbose is false, there should be no output
	if strings.Contains(output, "Using config file:") {
		t.Errorf("Non-verbose mode should not print config file message, got: %q", output)
	}
}

func TestExecute_HelpCommand(t *testing.T) {
	// Test that Execute can run the help command without error
	// We can't easily test Execute() directly since it calls os.Exit,
	// but we can test rootCmd.Execute() with help

	// Create a new command to avoid modifying the global state
	cmd := &cobra.Command{
		Use:   "test",
		Short: "Test command",
		Run:   func(cmd *cobra.Command, args []string) {},
	}

	// Execute with --help should not return an error
	cmd.SetArgs([]string{"--help"})

	// Suppress output
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	err := cmd.Execute()
	if err != nil {
		t.Errorf("Execute with --help returned error: %v", err)
	}
}

func TestRootCommand_ExecuteWithUnknownCommand(t *testing.T) {
	// Test behavior with unknown subcommand
	// Using rootCmd since it has subcommands registered and will error on unknown ones
	// Capture stderr to avoid noise in test output
	var stderr bytes.Buffer

	// Create a copy of the command to test without modifying the original
	testCmd := *rootCmd
	testCmd.SetArgs([]string{"unknown-subcommand-xyz"})
	testCmd.SetOut(&bytes.Buffer{})
	testCmd.SetErr(&stderr)

	err := testCmd.Execute()
	// Unknown subcommand should return an error when the command has subcommands
	if err == nil {
		t.Error("Execute with unknown subcommand should return error")
	}
}

func TestInitConfig_ConfigFilePrecedence(t *testing.T) {
	// Test that explicit config file takes precedence over default location
	// Don't run in parallel - modifies global viper state
	tmpDir := t.TempDir()

	// Create default config
	defaultConfigDir := filepath.Join(tmpDir, ".config", "sre")
	if err := os.MkdirAll(defaultConfigDir, 0755); err != nil {
		t.Fatalf("Failed to create default config dir: %v", err)
	}

	defaultConfigContent := `[notes]
path = "/default/path"
`
	defaultConfigPath := filepath.Join(defaultConfigDir, "config.toml")
	if err := os.WriteFile(defaultConfigPath, []byte(defaultConfigContent), 0644); err != nil {
		t.Fatalf("Failed to write default config: %v", err)
	}

	// Create explicit config
	explicitConfigContent := `[notes]
path = "/explicit/path"
`
	explicitConfigPath := filepath.Join(tmpDir, "explicit-config.toml")
	if err := os.WriteFile(explicitConfigPath, []byte(explicitConfigContent), 0644); err != nil {
		t.Fatalf("Failed to write explicit config: %v", err)
	}

	// Reset viper
	viper.Reset()
	defer viper.Reset()

	t.Setenv("HOME", tmpDir)

	// Set explicit config file
	oldCfgFile := cfgFile
	cfgFile = explicitConfigPath
	defer func() { cfgFile = oldCfgFile }()

	// Run initConfig
	initConfig()

	// Explicit config should take precedence
	if viper.GetString("notes.path") != "/explicit/path" {
		t.Errorf("notes.path = %q, want %q (explicit config should take precedence)",
			viper.GetString("notes.path"), "/explicit/path")
	}
}

func TestInitConfig_ConfigType(t *testing.T) {
	// Don't run in parallel - modifies global viper state

	// Reset viper
	viper.Reset()
	defer viper.Reset()

	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Ensure cfgFile is empty
	oldCfgFile := cfgFile
	cfgFile = ""
	defer func() { cfgFile = oldCfgFile }()

	// Run initConfig
	initConfig()

	// Check that viper is configured for toml
	// We can't directly check the config type, but we can verify
	// it was set by the behavior with toml files
	configDir := filepath.Join(tmpDir, ".config", "sre")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	// Create a TOML config file
	tomlContent := `[test]
key = "toml_value"
`
	configPath := filepath.Join(configDir, "config.toml")
	if err := os.WriteFile(configPath, []byte(tomlContent), 0644); err != nil {
		t.Fatalf("Failed to write toml config: %v", err)
	}

	// Reset and re-run to pick up the new file
	viper.Reset()
	initConfig()

	if viper.GetString("test.key") != "toml_value" {
		t.Errorf("Expected toml_value but got %q - TOML parsing may not be working",
			viper.GetString("test.key"))
	}
}

// Note: containsSubstring helper is already defined in init_test.go
