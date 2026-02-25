package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/zalando/go-keyring"
)

func TestLayeredLoader_Cascade(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Create a nested structure: root -> sub1 -> sub2
	root := tmpDir
	sub1 := filepath.Join(root, "sub1")
	sub2 := filepath.Join(sub1, "sub2")

	if err := os.MkdirAll(sub2, 0755); err != nil {
		t.Fatalf("failed to create dirs: %v", err)
	}

	// .rig.toml at root
	rootConfig := `
[github]
default_merge_method = "merge"
delete_branch_on_merge = false
`
	if err := os.WriteFile(filepath.Join(root, ".rig.toml"), []byte(rootConfig), 0644); err != nil {
		t.Fatalf("failed to write root config: %v", err)
	}

	// .rig.toml at sub1
	sub1Config := `
[github]
default_merge_method = "squash"
`
	if err := os.WriteFile(filepath.Join(sub1, ".rig.toml"), []byte(sub1Config), 0644); err != nil {
		t.Fatalf("failed to write sub1 config: %v", err)
	}

	// .rig.toml at sub2
	sub2Config := `
[tmux]
session_prefix = "sub2-"
`
	if err := os.WriteFile(filepath.Join(sub2, ".rig.toml"), []byte(sub2Config), 0644); err != nil {
		t.Fatalf("failed to write sub2 config: %v", err)
	}

	// We need a .git to define root, or we pass it explicitly.
	// NewLayeredLoader uses os.Getwd() and findGitRoot.
	// For testing, we'll mock the paths in the loader.

	l := &LayeredLoader{
		sources:        make(SourceMap),
		SkipGlobalSync: true,
		verbose:        true,
		gitRoot:        root,
		cwd:            sub2,
		userFile:       filepath.Join(tmpDir, "nonexistent.toml"),
	}

	cfg, err := l.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// Verify cascade: sub1 overrides root
	if cfg.GitHub.DefaultMergeMethod != "squash" {
		t.Errorf("github.default_merge_method = %q, want %q", cfg.GitHub.DefaultMergeMethod, "squash")
	}

	// Verify root value preserved
	if cfg.GitHub.DeleteBranchOnMerge {
		t.Error("github.delete_branch_on_merge should be false")
	}

	// Verify sub2 value
	if cfg.Tmux.SessionPrefix != "sub2-" {
		t.Errorf("tmux.session_prefix = %q, want %q", cfg.Tmux.SessionPrefix, "sub2-")
	}

	// Verify source tracking
	sources := l.Sources()
	if sources.Get("github.default_merge_method") != "Project: "+filepath.Join(sub1, ".rig.toml") {
		t.Errorf("source for github.default_merge_method = %q", sources.Get("github.default_merge_method"))
	}
	if sources.Get("github.delete_branch_on_merge") != "Project: "+filepath.Join(root, ".rig.toml") {
		t.Errorf("source for github.delete_branch_on_merge = %q", sources.Get("github.delete_branch_on_merge"))
	}
	if sources.Get("tmux.session_prefix") != "Project: "+filepath.Join(sub2, ".rig.toml") {
		t.Errorf("source for tmux.session_prefix = %q", sources.Get("tmux.session_prefix"))
	}
}

func TestCollectProjectConfigs(t *testing.T) {
	root := "/home/user/project"
	cwd := "/home/user/project/a/b"

	configs := CollectProjectConfigs(root, cwd)

	expected := []string{
		filepath.Join(root, ".rig.toml"),
		filepath.Join(root, "a", ".rig.toml"),
		filepath.Join(root, "a", "b", ".rig.toml"),
	}

	if len(configs) != len(expected) {
		t.Fatalf("got %d configs, want %d", len(configs), len(expected))
	}

	for i, c := range configs {
		if c != expected[i] {
			t.Errorf("config[%d] = %q, want %q", i, c, expected[i])
		}
	}
}

func TestCollectProjectConfigs_Deduplication(t *testing.T) {
	root := "/home/user/project"
	cwd := "/home/user/project"

	configs := CollectProjectConfigs(root, cwd)

	if len(configs) != 1 {
		t.Fatalf("got %d configs, want 1 (deduplicated)", len(configs))
	}
	expected := filepath.Join(root, ".rig.toml")
	if configs[0] != expected {
		t.Errorf("config = %q, want %q", configs[0], expected)
	}
}

func TestLayeredLoader_EnvOverride(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	t.Setenv("HOME", t.TempDir())
	t.Setenv("RIG_GITHUB_TOKEN", "env-token")
	t.Setenv("RIG_JIRA_ENABLED", "false")

	l := &LayeredLoader{
		sources:        make(SourceMap),
		SkipGlobalSync: true,
		userFile:       filepath.Join(t.TempDir(), "config.toml"),
	}

	cfg, err := l.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.GitHub.Token != "env-token" {
		t.Errorf("github.token = %q, want %q", cfg.GitHub.Token, "env-token")
	}
	if cfg.Jira.Enabled != false {
		t.Error("jira.enabled should be false")
	}

	sources := l.Sources()
	if sources.Get("github.token") != "Env" {
		t.Errorf("source for github.token = %q", sources.Get("github.token"))
	}
}

func TestLayeredLoader_EnvOverrideEmpty(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	t.Setenv("HOME", t.TempDir())
	// Set an env var to an empty string — should still be attributed to Env
	t.Setenv("RIG_GITHUB_TOKEN", "")

	l := &LayeredLoader{
		sources:        make(SourceMap),
		SkipGlobalSync: true,
		userFile:       filepath.Join(t.TempDir(), "config.toml"),
	}

	_, err := l.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	sources := l.Sources()
	if sources.Get("github.token") != "Env" {
		t.Errorf("source for github.token = %q, want %q (empty env var should still be attributed to Env)", sources.Get("github.token"), "Env")
	}
}

func TestLayeredLoader_Keychain(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	t.Setenv("HOME", t.TempDir())
	// Use mock keyring backend for deterministic CI behavior
	keyring.MockInit()

	service := "rig-test"
	account := "test-secret"
	secret := "shhh-secret"

	if err := keyring.Set(service, account, secret); err != nil {
		t.Fatalf("failed to set mock keyring secret: %v", err)
	}

	tmpDir := t.TempDir()
	configContent := `
[github]
token = "keychain://rig-test/test-secret"
`
	configPath := filepath.Join(tmpDir, "config.toml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	l := &LayeredLoader{
		sources:        make(SourceMap),
		SkipGlobalSync: true,
		userFile:       configPath,
	}

	cfg, err := l.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.GitHub.Token != secret {
		t.Errorf("github.token = %q, want %q", cfg.GitHub.Token, secret)
	}

	sources := l.Sources()
	if sources.Get("github.token") != "Keychain: rig-test/test-secret" {
		t.Errorf("source for github.token = %q", sources.Get("github.token"))
	}
}

func TestLayeredLoader_KeychainFailure(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	t.Setenv("HOME", t.TempDir())
	// Use mock keyring with NO secrets set — lookup will fail
	keyring.MockInit()

	tmpDir := t.TempDir()
	configContent := `
[github]
token = "keychain://missing-service/missing-account"
`
	configPath := filepath.Join(tmpDir, "config.toml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	l := &LayeredLoader{
		sources:        make(SourceMap),
		userFile:       configPath,
		SkipGlobalSync: true,
	}

	cfg, err := l.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// The raw keychain URI should remain as the config value
	if cfg.GitHub.Token != "keychain://missing-service/missing-account" {
		t.Errorf("github.token = %q, want raw keychain URI preserved", cfg.GitHub.Token)
	}

	// Source attribution should still show User (the tier that set the value),
	// NOT Keychain (since resolution failed)
	sources := l.Sources()
	got := sources.Get("github.token")
	if got != "User: "+configPath {
		t.Errorf("source for github.token = %q, want User attribution (not Keychain)", got)
	}
}

func TestResolveKeychainValues_Slice(t *testing.T) {
	keyring.MockInit()
	service := "rig-test"
	account := "test-slice-secret"
	secret := "slice-secret-value"
	_ = keyring.Set(service, account, secret)

	settings := map[string]interface{}{
		"list": []interface{}{
			"normal-item",
			"keychain://rig-test/test-slice-secret",
		},
	}
	sources := make(SourceMap)

	err := ResolveKeychainValues(settings, sources, false)
	if err != nil {
		t.Fatalf("ResolveKeychainValues failed: %v", err)
	}

	list := settings["list"].([]interface{})
	if list[1] != secret {
		t.Errorf("list[1] = %q, want %q", list[1], secret)
	}

	if sources.Get("list[1]") != "Keychain: rig-test/test-slice-secret" {
		t.Errorf("source for list[1] = %q", sources.Get("list[1]"))
	}
}
