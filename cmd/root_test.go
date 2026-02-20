package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"thoreinstein.com/rig/pkg/bootstrap"
)

func TestRootCommandStructure(t *testing.T) {
	// Not parallel - accesses global rootCmd
	cmd := rootCmd

	if cmd.Use != "rig" {
		t.Errorf("root command Use = %q, want %q", cmd.Use, "rig")
	}

	if cmd.Short == "" {
		t.Error("root command should have Short description")
	}

	if cmd.Long == "" {
		t.Error("root command should have Long description")
	}

	// Verify key information is in the description
	expectedKeywords := []string{"Rig", "workflow", "automation"}
	for _, keyword := range expectedKeywords {
		if !strings.Contains(cmd.Long, keyword) {
			t.Errorf("root command Long description should mention %q", keyword)
		}
	}
}

func TestRootCommandPersistentFlags(t *testing.T) {
	// Not parallel - accesses global rootCmd
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
		if !strings.Contains(configFlag.Usage, "$HOME/.config/rig") {
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
	// Not parallel - accesses global rootCmd
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
	expectedCommands := []string{"work", "list", "session", "config", "sync", "history", "timeline", "clean", "hack", "update", "version"}
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
	_ = initConfig()

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
	configDir := filepath.Join(tmpDir, ".config", "rig")
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
	_ = initConfig()

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
	_ = initConfig()

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
	_ = initConfig()

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
	configDir := filepath.Join(tmpDir, ".config", "rig")
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
	_ = initConfig()

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
	configDir := filepath.Join(tmpDir, ".config", "rig")
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
	_ = initConfig()

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
	defaultConfigDir := filepath.Join(tmpDir, ".config", "rig")
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
	_ = initConfig()

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
	_ = initConfig()

	// Check that viper is configured for toml
	// We can't directly check the config type, but we can verify
	// it was set by the behavior with toml files
	configDir := filepath.Join(tmpDir, ".config", "rig")
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
	_ = initConfig()

	if viper.GetString("test.key") != "toml_value" {
		t.Errorf("Expected toml_value but got %q - TOML parsing may not be working",
			viper.GetString("test.key"))
	}
}

// Note: containsSubstring helper is already defined in work_test.go

func TestInitConfig_TMUXEnvVarDoesNotOverrideConfig(t *testing.T) {
	// Set TMUX like tmux itself does (socket path, pid, session index)
	// t.Setenv automatically restores the original value after the test
	t.Setenv("TMUX", "/private/tmp/tmux-502/default,12345,0")

	tmpDir := t.TempDir()

	// Create config directory with tmux config that has windows defined
	configDir := filepath.Join(tmpDir, ".config", "rig")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	// Config with explicitly defined tmux windows
	configContent := `[notes]
path = "/test/notes"

[tmux]
session_prefix = "test"

[[tmux.windows]]
name = "editor"
command = "nvim"

[[tmux.windows]]
name = "shell"
`
	configPath := filepath.Join(configDir, "config.toml")
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	viper.Reset()
	defer viper.Reset()

	t.Setenv("HOME", tmpDir)

	oldCfgFile := cfgFile
	cfgFile = ""
	defer func() { cfgFile = oldCfgFile }()

	_ = initConfig()

	// With the RIG_ prefix, plain TMUX env var should not interfere
	// Verify the tmux config section is properly loaded
	windows := viper.Get("tmux.windows")
	if windows == nil {
		t.Error("TMUX env var overwrote tmux config - windows is nil")
	}

	// Check via viper.GetString to see if TMUX env var leaked through
	tmuxVal := viper.GetString("tmux")
	if strings.Contains(tmuxVal, "/private/tmp") || strings.Contains(tmuxVal, "tmux-502") {
		t.Errorf("TMUX env var leaked into config: tmux=%q", tmuxVal)
	}

	// Also verify the session_prefix is correct
	prefix := viper.GetString("tmux.session_prefix")
	if prefix != "test" {
		t.Errorf("tmux.session_prefix = %q, want %q", prefix, "test")
	}
}

// =============================================================================
// findGitRoot() Tests
// =============================================================================

// evalSymlinks resolves symlinks for path comparison (handles macOS /private/var -> /var)
func evalSymlinks(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		// If the path doesn't exist yet or can't be resolved, return original
		return path
	}
	return resolved
}

func TestFindGitRoot_FromRepoRoot(t *testing.T) {
	tmpDir := evalSymlinks(t, t.TempDir())

	// Create a fake git repository
	gitDir := filepath.Join(tmpDir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatalf("Failed to create .git dir: %v", err)
	}

	// Change to repo root (t.Chdir handles cleanup)
	t.Chdir(tmpDir)

	// Find git root
	root, err := bootstrap.FindGitRoot()
	if err != nil {
		t.Fatalf("FindGitRoot() error: %v", err)
	}

	if root != tmpDir {
		t.Errorf("FindGitRoot() = %q, want %q", root, tmpDir)
	}
}

func TestFindGitRoot_FromSubdirectory(t *testing.T) {
	tmpDir := evalSymlinks(t, t.TempDir())

	// Create a fake git repository with subdirectories
	gitDir := filepath.Join(tmpDir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatalf("Failed to create .git dir: %v", err)
	}

	subDir := filepath.Join(tmpDir, "src", "pkg", "deep")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	// Change to deep subdirectory (t.Chdir handles cleanup)
	t.Chdir(subDir)

	// Find git root
	root, err := bootstrap.FindGitRoot()
	if err != nil {
		t.Fatalf("FindGitRoot() error: %v", err)
	}

	if root != tmpDir {
		t.Errorf("FindGitRoot() = %q, want %q", root, tmpDir)
	}
}

func TestFindGitRoot_NotInGitRepo(t *testing.T) {
	tmpDir := evalSymlinks(t, t.TempDir())

	// No .git directory created - just an empty temp dir

	// Change to the temp dir (t.Chdir handles cleanup)
	t.Chdir(tmpDir)

	// Find git root - should return empty string, no error
	root, err := bootstrap.FindGitRoot()
	if err != nil {
		t.Fatalf("FindGitRoot() should not error when not in git repo: %v", err)
	}

	if root != "" {
		t.Errorf("FindGitRoot() = %q, want empty string when not in git repo", root)
	}
}

func TestFindGitRoot_GitWorktree(t *testing.T) {
	tmpDir := evalSymlinks(t, t.TempDir())

	// Create a worktree-style .git file (file, not directory)
	// Git worktrees have a .git file with content: "gitdir: /path/to/main/repo/.git/worktrees/name"
	gitFile := filepath.Join(tmpDir, ".git")
	gitFileContent := "gitdir: /some/other/path/.git/worktrees/feature-branch"
	if err := os.WriteFile(gitFile, []byte(gitFileContent), 0644); err != nil {
		t.Fatalf("Failed to create .git file: %v", err)
	}

	// Change to worktree root (t.Chdir handles cleanup)
	t.Chdir(tmpDir)

	// Find git root - should recognize .git file as valid
	root, err := bootstrap.FindGitRoot()
	if err != nil {
		t.Fatalf("FindGitRoot() error: %v", err)
	}

	if root != tmpDir {
		t.Errorf("FindGitRoot() = %q, want %q (should recognize .git file for worktrees)", root, tmpDir)
	}
}

func TestFindGitRoot_WorktreeSubdirectory(t *testing.T) {
	tmpDir := evalSymlinks(t, t.TempDir())

	// Create a worktree-style .git file
	gitFile := filepath.Join(tmpDir, ".git")
	gitFileContent := "gitdir: /some/other/path/.git/worktrees/feature-branch"
	if err := os.WriteFile(gitFile, []byte(gitFileContent), 0644); err != nil {
		t.Fatalf("Failed to create .git file: %v", err)
	}

	// Create subdirectory
	subDir := filepath.Join(tmpDir, "cmd", "myapp")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	// Change to subdirectory (t.Chdir handles cleanup)
	t.Chdir(subDir)

	// Find git root from worktree subdirectory
	root, err := bootstrap.FindGitRoot()
	if err != nil {
		t.Fatalf("FindGitRoot() error: %v", err)
	}

	if root != tmpDir {
		t.Errorf("FindGitRoot() = %q, want %q (should find worktree root from subdirectory)", root, tmpDir)
	}
}

// =============================================================================
// LoadRepoLocalConfig() Tests
// =============================================================================

func TestLoadRepoLocalConfig_FromGitRoot(t *testing.T) {
	// Don't run in parallel - modifies global viper state
	tmpDir := t.TempDir()

	// Create fake git repo
	gitDir := filepath.Join(tmpDir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatalf("Failed to create .git dir: %v", err)
	}

	// Create .rig.toml at git root
	rigConfig := `[github]
default_merge_method = "rebase"

[ai]
provider = "ollama"
ollama_model = "codellama"
`
	rigConfigPath := filepath.Join(tmpDir, ".rig.toml")
	if err := os.WriteFile(rigConfigPath, []byte(rigConfig), 0644); err != nil {
		t.Fatalf("Failed to write .rig.toml: %v", err)
	}

	// Setup: reset viper, change to repo
	viper.Reset()
	defer viper.Reset()

	t.Chdir(tmpDir)

	// Load repo local config
	bootstrap.LoadRepoLocalConfig(false)

	// Verify values were loaded
	if got := viper.GetString("github.default_merge_method"); got != "rebase" {
		t.Errorf("github.default_merge_method = %q, want %q", got, "rebase")
	}
	if got := viper.GetString("ai.provider"); got != "ollama" {
		t.Errorf("ai.provider = %q, want %q", got, "ollama")
	}
	if got := viper.GetString("ai.ollama_model"); got != "codellama" {
		t.Errorf("ai.ollama_model = %q, want %q", got, "codellama")
	}
}

func TestLoadRepoLocalConfig_FromSubdirectory(t *testing.T) {
	// Don't run in parallel - modifies global viper state
	tmpDir := t.TempDir()

	// Create fake git repo with subdirectory
	gitDir := filepath.Join(tmpDir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatalf("Failed to create .git dir: %v", err)
	}

	subDir := filepath.Join(tmpDir, "services", "api")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	// Create .rig.toml in subdirectory only
	rigConfig := `[github]
default_merge_method = "squash"

[tmux]
session_prefix = "api-"
`
	rigConfigPath := filepath.Join(subDir, ".rig.toml")
	if err := os.WriteFile(rigConfigPath, []byte(rigConfig), 0644); err != nil {
		t.Fatalf("Failed to write .rig.toml: %v", err)
	}

	// Setup
	viper.Reset()
	defer viper.Reset()

	t.Chdir(subDir)

	// Load repo local config
	bootstrap.LoadRepoLocalConfig(false)

	// Verify values from subdirectory config
	if got := viper.GetString("github.default_merge_method"); got != "squash" {
		t.Errorf("github.default_merge_method = %q, want %q", got, "squash")
	}
	if got := viper.GetString("tmux.session_prefix"); got != "api-" {
		t.Errorf("tmux.session_prefix = %q, want %q", got, "api-")
	}
}

func TestLoadRepoLocalConfig_CascadingMerge(t *testing.T) {
	// Don't run in parallel - modifies global viper state
	tmpDir := t.TempDir()

	// Create fake git repo
	gitDir := filepath.Join(tmpDir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatalf("Failed to create .git dir: %v", err)
	}

	subDir := filepath.Join(tmpDir, "services", "api")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	// Create .rig.toml at git root
	rootConfig := `[github]
default_merge_method = "merge"
delete_branch_on_merge = true

[ai]
provider = "anthropic"
model = "claude-sonnet"

[tmux]
session_prefix = "root-"
`
	rootConfigPath := filepath.Join(tmpDir, ".rig.toml")
	if err := os.WriteFile(rootConfigPath, []byte(rootConfig), 0644); err != nil {
		t.Fatalf("Failed to write root .rig.toml: %v", err)
	}

	// Create .rig.toml in subdirectory that overrides some values
	subConfig := `[github]
default_merge_method = "squash"

[tmux]
session_prefix = "api-"
`
	subConfigPath := filepath.Join(subDir, ".rig.toml")
	if err := os.WriteFile(subConfigPath, []byte(subConfig), 0644); err != nil {
		t.Fatalf("Failed to write subdirectory .rig.toml: %v", err)
	}

	// Setup
	viper.Reset()
	defer viper.Reset()

	t.Chdir(subDir)

	// Load repo local config
	bootstrap.LoadRepoLocalConfig(false)

	// Verify subdirectory overrides root
	if got := viper.GetString("github.default_merge_method"); got != "squash" {
		t.Errorf("github.default_merge_method = %q, want %q (subdirectory should override root)", got, "squash")
	}
	if got := viper.GetString("tmux.session_prefix"); got != "api-" {
		t.Errorf("tmux.session_prefix = %q, want %q (subdirectory should override root)", got, "api-")
	}

	// Verify root values that weren't overridden are preserved
	if got := viper.GetBool("github.delete_branch_on_merge"); !got {
		t.Error("github.delete_branch_on_merge should be true (from root config)")
	}
	if got := viper.GetString("ai.provider"); got != "anthropic" {
		t.Errorf("ai.provider = %q, want %q (from root config)", got, "anthropic")
	}
	if got := viper.GetString("ai.model"); got != "claude-sonnet" {
		t.Errorf("ai.model = %q, want %q (from root config)", got, "claude-sonnet")
	}
}

func TestLoadRepoLocalConfig_NoConfigPresent(t *testing.T) {
	// Don't run in parallel - modifies global viper state
	tmpDir := t.TempDir()

	// Create fake git repo without any .rig.toml
	gitDir := filepath.Join(tmpDir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatalf("Failed to create .git dir: %v", err)
	}

	// Setup
	viper.Reset()
	defer viper.Reset()

	// Set a known value before calling LoadRepoLocalConfig
	viper.Set("test.existing_value", "preserved")

	t.Chdir(tmpDir)

	// Should not panic or error when no .rig.toml exists
	bootstrap.LoadRepoLocalConfig(false)

	// Existing values should be preserved
	if got := viper.GetString("test.existing_value"); got != "preserved" {
		t.Errorf("test.existing_value = %q, want %q (should be preserved when no .rig.toml)", got, "preserved")
	}
}

func TestLoadRepoLocalConfig_MalformedConfig(t *testing.T) {
	// Don't run in parallel - modifies global viper state
	tmpDir := t.TempDir()

	// Create fake git repo
	gitDir := filepath.Join(tmpDir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatalf("Failed to create .git dir: %v", err)
	}

	// Create malformed .rig.toml
	malformedConfig := `[github]
default_merge_method = "squash"  # valid
this is not valid toml syntax
[broken
`
	rigConfigPath := filepath.Join(tmpDir, ".rig.toml")
	if err := os.WriteFile(rigConfigPath, []byte(malformedConfig), 0644); err != nil {
		t.Fatalf("Failed to write .rig.toml: %v", err)
	}

	// Setup
	viper.Reset()
	defer viper.Reset()

	// Set a known value before
	viper.Set("test.existing_value", "preserved")

	t.Chdir(tmpDir)

	// Should not panic - gracefully handle malformed config
	bootstrap.LoadRepoLocalConfig(false)

	// Existing values should be preserved even with malformed config
	if got := viper.GetString("test.existing_value"); got != "preserved" {
		t.Errorf("test.existing_value = %q, want %q (should be preserved even with malformed .rig.toml)", got, "preserved")
	}
}

func TestLoadRepoLocalConfig_MalformedConfigVerbose(t *testing.T) {
	// Don't run in parallel - modifies global viper state and verbose flag
	tmpDir := t.TempDir()

	// Create fake git repo
	gitDir := filepath.Join(tmpDir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatalf("Failed to create .git dir: %v", err)
	}

	// Create malformed .rig.toml
	malformedConfig := `this is completely invalid`
	rigConfigPath := filepath.Join(tmpDir, ".rig.toml")
	if err := os.WriteFile(rigConfigPath, []byte(malformedConfig), 0644); err != nil {
		t.Fatalf("Failed to write .rig.toml: %v", err)
	}

	// Setup
	viper.Reset()
	defer viper.Reset()

	oldVerbose := verbose
	verbose = true
	defer func() { verbose = oldVerbose }()

	t.Chdir(tmpDir)

	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	bootstrap.LoadRepoLocalConfig(true)

	w.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	// In verbose mode, should warn about malformed config
	if !strings.Contains(output, "Warning") || !strings.Contains(output, ".rig.toml") {
		t.Errorf("Verbose mode should warn about malformed .rig.toml, got: %q", output)
	}
}

func TestLoadRepoLocalConfig_NotInGitRepo(t *testing.T) {
	// Don't run in parallel - modifies global viper state
	tmpDir := t.TempDir()

	// No .git directory - not a git repo

	// Create .rig.toml in current directory (fallback behavior)
	rigConfig := `[github]
default_merge_method = "rebase"
`
	rigConfigPath := filepath.Join(tmpDir, ".rig.toml")
	if err := os.WriteFile(rigConfigPath, []byte(rigConfig), 0644); err != nil {
		t.Fatalf("Failed to write .rig.toml: %v", err)
	}

	// Setup
	viper.Reset()
	defer viper.Reset()

	t.Chdir(tmpDir)

	// Load repo local config - should use fallback to current directory
	bootstrap.LoadRepoLocalConfig(false)

	// Should still load .rig.toml from current directory
	if got := viper.GetString("github.default_merge_method"); got != "rebase" {
		t.Errorf("github.default_merge_method = %q, want %q (fallback should load from cwd)", got, "rebase")
	}
}

func TestLoadRepoLocalConfig_VerboseOutput(t *testing.T) {
	// Don't run in parallel - modifies global viper state and verbose flag
	tmpDir := t.TempDir()

	// Create fake git repo
	gitDir := filepath.Join(tmpDir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatalf("Failed to create .git dir: %v", err)
	}

	// Create .rig.toml
	rigConfig := `[github]
default_merge_method = "squash"
`
	rigConfigPath := filepath.Join(tmpDir, ".rig.toml")
	if err := os.WriteFile(rigConfigPath, []byte(rigConfig), 0644); err != nil {
		t.Fatalf("Failed to write .rig.toml: %v", err)
	}

	// Setup
	viper.Reset()
	defer viper.Reset()

	oldVerbose := verbose
	verbose = true
	defer func() { verbose = oldVerbose }()

	t.Chdir(tmpDir)

	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	bootstrap.LoadRepoLocalConfig(true)

	w.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	// In verbose mode, should mention using repository config
	if !strings.Contains(output, "Using repository config") {
		t.Errorf("Verbose mode should print 'Using repository config', got: %q", output)
	}
	if !strings.Contains(output, ".rig.toml") {
		t.Errorf("Verbose mode should mention .rig.toml, got: %q", output)
	}
}

// =============================================================================
// Integration Tests: Full Precedence Chain
// =============================================================================

func TestConfigPrecedence_EnvOverridesRepoConfig(t *testing.T) {
	// Don't run in parallel - modifies global viper state
	tmpDir := t.TempDir()

	// Create fake git repo
	gitDir := filepath.Join(tmpDir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatalf("Failed to create .git dir: %v", err)
	}

	// Create .rig.toml with a value
	rigConfig := `[github]
default_merge_method = "squash"

[ai]
provider = "ollama"
`
	rigConfigPath := filepath.Join(tmpDir, ".rig.toml")
	if err := os.WriteFile(rigConfigPath, []byte(rigConfig), 0644); err != nil {
		t.Fatalf("Failed to write .rig.toml: %v", err)
	}

	// Setup
	viper.Reset()
	defer viper.Reset()

	// Set environment variable that should override repo config
	t.Setenv("RIG_GITHUB_DEFAULT_MERGE_METHOD", "rebase")
	t.Setenv("HOME", tmpDir) // Ensure no user config is loaded

	oldCfgFile := cfgFile
	cfgFile = ""
	defer func() { cfgFile = oldCfgFile }()

	t.Chdir(tmpDir)

	// Run full initConfig
	_ = initConfig()

	// Environment variable should take precedence over repo config
	if got := viper.GetString("github.default_merge_method"); got != "rebase" {
		t.Errorf("github.default_merge_method = %q, want %q (env var should override repo config)", got, "rebase")
	}

	// Repo config value that wasn't overridden should still be present
	if got := viper.GetString("ai.provider"); got != "ollama" {
		t.Errorf("ai.provider = %q, want %q (repo config should be loaded)", got, "ollama")
	}
}

func TestConfigPrecedence_RepoConfigOverridesUserConfig(t *testing.T) {
	// Don't run in parallel - modifies global viper state
	tmpDir := t.TempDir()

	// Create user config directory
	userConfigDir := filepath.Join(tmpDir, ".config", "rig")
	if err := os.MkdirAll(userConfigDir, 0755); err != nil {
		t.Fatalf("Failed to create user config dir: %v", err)
	}

	// Create user config
	userConfig := `[github]
default_merge_method = "merge"
delete_branch_on_merge = false

[ai]
provider = "anthropic"
`
	userConfigPath := filepath.Join(userConfigDir, "config.toml")
	if err := os.WriteFile(userConfigPath, []byte(userConfig), 0644); err != nil {
		t.Fatalf("Failed to write user config: %v", err)
	}

	// Create a "repo" directory inside tmpDir
	repoDir := filepath.Join(tmpDir, "myrepo")
	gitDir := filepath.Join(repoDir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatalf("Failed to create .git dir: %v", err)
	}

	// Create repo .rig.toml that overrides some values
	repoConfig := `[github]
default_merge_method = "squash"

[tmux]
session_prefix = "repo-"
`
	repoConfigPath := filepath.Join(repoDir, ".rig.toml")
	if err := os.WriteFile(repoConfigPath, []byte(repoConfig), 0644); err != nil {
		t.Fatalf("Failed to write repo .rig.toml: %v", err)
	}

	// Setup
	viper.Reset()
	defer viper.Reset()

	t.Setenv("HOME", tmpDir)

	oldCfgFile := cfgFile
	cfgFile = ""
	defer func() { cfgFile = oldCfgFile }()

	t.Chdir(repoDir)

	// Run full initConfig
	_ = initConfig()

	// Repo config should override user config
	if got := viper.GetString("github.default_merge_method"); got != "squash" {
		t.Errorf("github.default_merge_method = %q, want %q (repo config should override user config)", got, "squash")
	}

	// Values only in repo config should be present
	if got := viper.GetString("tmux.session_prefix"); got != "repo-" {
		t.Errorf("tmux.session_prefix = %q, want %q (from repo config)", got, "repo-")
	}

	// User config values not overridden should still be present
	if got := viper.GetBool("github.delete_branch_on_merge"); got != false {
		t.Error("github.delete_branch_on_merge should be false (from user config)")
	}
	if got := viper.GetString("ai.provider"); got != "anthropic" {
		t.Errorf("ai.provider = %q, want %q (from user config)", got, "anthropic")
	}
}

func TestConfigPrecedence_FullChain(t *testing.T) {
	// Don't run in parallel - modifies global viper state
	// Tests: env var > repo config > user config > defaults
	tmpDir := t.TempDir()

	// Create user config directory
	userConfigDir := filepath.Join(tmpDir, ".config", "rig")
	if err := os.MkdirAll(userConfigDir, 0755); err != nil {
		t.Fatalf("Failed to create user config dir: %v", err)
	}

	// Create user config with multiple values
	userConfig := `[github]
default_merge_method = "merge"
delete_branch_on_merge = true

[ai]
provider = "anthropic"
model = "user-model"

[notes]
path = "/user/notes"
`
	userConfigPath := filepath.Join(userConfigDir, "config.toml")
	if err := os.WriteFile(userConfigPath, []byte(userConfig), 0644); err != nil {
		t.Fatalf("Failed to write user config: %v", err)
	}

	// Create a "repo" directory
	repoDir := filepath.Join(tmpDir, "myrepo")
	gitDir := filepath.Join(repoDir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatalf("Failed to create .git dir: %v", err)
	}

	// Create repo .rig.toml
	repoConfig := `[github]
default_merge_method = "squash"

[ai]
provider = "ollama"
`
	repoConfigPath := filepath.Join(repoDir, ".rig.toml")
	if err := os.WriteFile(repoConfigPath, []byte(repoConfig), 0644); err != nil {
		t.Fatalf("Failed to write repo .rig.toml: %v", err)
	}

	// Setup
	viper.Reset()
	defer viper.Reset()

	// Set env var that overrides everything
	t.Setenv("RIG_AI_PROVIDER", "groq")
	t.Setenv("HOME", tmpDir)

	oldCfgFile := cfgFile
	cfgFile = ""
	defer func() { cfgFile = oldCfgFile }()

	t.Chdir(repoDir)

	// Run full initConfig
	_ = initConfig()

	// Test precedence chain:

	// 1. Env var should win over everything
	if got := viper.GetString("ai.provider"); got != "groq" {
		t.Errorf("ai.provider = %q, want %q (env var should override all)", got, "groq")
	}

	// 2. Repo config should override user config
	if got := viper.GetString("github.default_merge_method"); got != "squash" {
		t.Errorf("github.default_merge_method = %q, want %q (repo config should override user config)", got, "squash")
	}

	// 3. User config value not overridden by repo config should persist
	if got := viper.GetBool("github.delete_branch_on_merge"); !got {
		t.Error("github.delete_branch_on_merge should be true (from user config, not overridden by repo)")
	}
	if got := viper.GetString("ai.model"); got != "user-model" {
		t.Errorf("ai.model = %q, want %q (from user config)", got, "user-model")
	}
	if got := viper.GetString("notes.path"); got != "/user/notes" {
		t.Errorf("notes.path = %q, want %q (from user config)", got, "/user/notes")
	}
}

// =============================================================================
// Table-Driven Test for FindGitRoot
// =============================================================================

func TestFindGitRoot_TableDriven(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(tmpDir string) string // returns the directory to chdir to
		wantRoot   bool                       // true if we expect a non-empty root
		wantSame   bool                       // true if root should equal cwd (for repo root case)
		wantParent bool                       // true if root should be parent of cwd (for subdirectory case)
	}{
		{
			name: "regular git repo at root",
			setup: func(tmpDir string) string {
				_ = os.MkdirAll(filepath.Join(tmpDir, ".git"), 0755)
				return tmpDir
			},
			wantRoot: true,
			wantSame: true,
		},
		{
			name: "regular git repo from subdirectory",
			setup: func(tmpDir string) string {
				_ = os.MkdirAll(filepath.Join(tmpDir, ".git"), 0755)
				subDir := filepath.Join(tmpDir, "a", "b", "c")
				_ = os.MkdirAll(subDir, 0755)
				return subDir
			},
			wantRoot:   true,
			wantParent: true,
		},
		{
			name: "git worktree at root",
			setup: func(tmpDir string) string {
				_ = os.WriteFile(filepath.Join(tmpDir, ".git"), []byte("gitdir: /path/to/.git/worktrees/x"), 0644)
				return tmpDir
			},
			wantRoot: true,
			wantSame: true,
		},
		{
			name: "git worktree from subdirectory",
			setup: func(tmpDir string) string {
				_ = os.WriteFile(filepath.Join(tmpDir, ".git"), []byte("gitdir: /path/to/.git/worktrees/x"), 0644)
				subDir := filepath.Join(tmpDir, "pkg", "cmd")
				_ = os.MkdirAll(subDir, 0755)
				return subDir
			},
			wantRoot:   true,
			wantParent: true,
		},
		{
			name: "not in git repo",
			setup: func(tmpDir string) string {
				return tmpDir
			},
			wantRoot: false,
		},
		{
			name: "deeply nested not in git repo",
			setup: func(tmpDir string) string {
				subDir := filepath.Join(tmpDir, "a", "b", "c", "d", "e")
				_ = os.MkdirAll(subDir, 0755)
				return subDir
			},
			wantRoot: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := evalSymlinks(t, t.TempDir())
			cwd := tt.setup(tmpDir)

			t.Chdir(cwd)

			root, err := bootstrap.FindGitRoot()
			if err != nil {
				t.Fatalf("FindGitRoot() error: %v", err)
			}

			if tt.wantRoot && root == "" {
				t.Error("FindGitRoot() = empty, want non-empty root")
			}
			if !tt.wantRoot && root != "" {
				t.Errorf("FindGitRoot() = %q, want empty string", root)
			}
			if tt.wantSame && root != cwd {
				t.Errorf("FindGitRoot() = %q, want %q (same as cwd)", root, cwd)
			}
			if tt.wantParent && root != tmpDir {
				t.Errorf("FindGitRoot() = %q, want %q (parent tmpDir)", root, tmpDir)
			}
		})
	}
}

// =============================================================================
// LoadRepoLocalConfig() Tests
// =============================================================================

func TestLoadRepoLocalConfig_TableDriven(t *testing.T) {
	tests := []struct {
		name         string
		setupRepo    func(tmpDir string) string // returns directory to chdir to
		setupConfigs func(tmpDir, cwd string)   // setup .rig.toml files
		presetViper  map[string]string          // values to set before loading
		wantValues   map[string]string          // expected values after loading
		wantMissing  []string                   // keys that should NOT be set
	}{
		{
			name: "single config at git root",
			setupRepo: func(tmpDir string) string {
				_ = os.MkdirAll(filepath.Join(tmpDir, ".git"), 0755)
				return tmpDir
			},
			setupConfigs: func(tmpDir, _ string) {
				cfg := `[github]
default_merge_method = "squash"
`
				_ = os.WriteFile(filepath.Join(tmpDir, ".rig.toml"), []byte(cfg), 0644)
			},
			wantValues: map[string]string{
				"github.default_merge_method": "squash",
			},
		},
		{
			name: "config in subdirectory only",
			setupRepo: func(tmpDir string) string {
				_ = os.MkdirAll(filepath.Join(tmpDir, ".git"), 0755)
				subDir := filepath.Join(tmpDir, "services")
				_ = os.MkdirAll(subDir, 0755)
				return subDir
			},
			setupConfigs: func(tmpDir, cwd string) {
				cfg := `[tmux]
session_prefix = "sub-"
`
				_ = os.WriteFile(filepath.Join(cwd, ".rig.toml"), []byte(cfg), 0644)
			},
			wantValues: map[string]string{
				"tmux.session_prefix": "sub-",
			},
		},
		{
			name: "cascading configs - subdirectory overrides root",
			setupRepo: func(tmpDir string) string {
				_ = os.MkdirAll(filepath.Join(tmpDir, ".git"), 0755)
				subDir := filepath.Join(tmpDir, "services")
				_ = os.MkdirAll(subDir, 0755)
				return subDir
			},
			setupConfigs: func(tmpDir, cwd string) {
				rootCfg := `[github]
default_merge_method = "merge"
delete_branch_on_merge = true
`
				_ = os.WriteFile(filepath.Join(tmpDir, ".rig.toml"), []byte(rootCfg), 0644)

				subCfg := `[github]
default_merge_method = "rebase"
`
				_ = os.WriteFile(filepath.Join(cwd, ".rig.toml"), []byte(subCfg), 0644)
			},
			wantValues: map[string]string{
				"github.default_merge_method":   "rebase", // overridden
				"github.delete_branch_on_merge": "true",   // from root
			},
		},
		{
			name: "no config files - preserves existing values",
			setupRepo: func(tmpDir string) string {
				_ = os.MkdirAll(filepath.Join(tmpDir, ".git"), 0755)
				return tmpDir
			},
			setupConfigs: func(_, _ string) {
				// No configs created
			},
			presetViper: map[string]string{
				"preset.value": "should-stay",
			},
			wantValues: map[string]string{
				"preset.value": "should-stay",
			},
		},
		{
			name: "not in git repo - uses cwd fallback",
			setupRepo: func(tmpDir string) string {
				// No .git directory
				return tmpDir
			},
			setupConfigs: func(tmpDir, _ string) {
				cfg := `[notes]
path = "/fallback/path"
`
				_ = os.WriteFile(filepath.Join(tmpDir, ".rig.toml"), []byte(cfg), 0644)
			},
			wantValues: map[string]string{
				"notes.path": "/fallback/path",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Don't run in parallel - modifies global viper state
			tmpDir := t.TempDir()
			cwd := tt.setupRepo(tmpDir)
			tt.setupConfigs(tmpDir, cwd)

			// Setup viper
			viper.Reset()
			defer viper.Reset()

			for k, v := range tt.presetViper {
				viper.Set(k, v)
			}

			t.Chdir(cwd)

			// Load repo local config
			bootstrap.LoadRepoLocalConfig(false)

			// Check expected values
			for key, want := range tt.wantValues {
				if got := viper.GetString(key); got != want {
					t.Errorf("%s = %q, want %q", key, got, want)
				}
			}

			// Check missing keys
			for _, key := range tt.wantMissing {
				if viper.IsSet(key) {
					t.Errorf("%s should not be set, but got %q", key, viper.GetString(key))
				}
			}
		})
	}
}
