package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/zalando/go-keyring"

	"thoreinstein.com/rig/pkg/project"
)

func TestLayeredLoader_Cascade(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a project structure with nested .rig.toml files
	// root/.git
	// root/.rig.toml
	// root/sub1/.rig.toml
	// root/sub1/sub2/.rig.toml

	root := filepath.Join(tmpDir, "root")
	sub1 := filepath.Join(root, "sub1")
	sub2 := filepath.Join(sub1, "sub2")

	if err := os.MkdirAll(sub2, 0755); err != nil {
		t.Fatalf("failed to create test directories: %v", err)
	}

	// Create .git at root to define project boundary
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0755); err != nil {
		t.Fatalf("failed to create .git: %v", err)
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
		projectCtx: &project.ProjectContext{
			RootPath: root,
			Origin:   sub2,
		},
		userFile: filepath.Join(tmpDir, "nonexistent.toml"),
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
}

func TestLayeredLoader_UserConfig(t *testing.T) {
	tmpDir := t.TempDir()

	userConfig := `
[ai]
provider = "groq"
`
	userFile := filepath.Join(tmpDir, "user.toml")
	if err := os.WriteFile(userFile, []byte(userConfig), 0644); err != nil {
		t.Fatalf("failed to write user config: %v", err)
	}

	l := &LayeredLoader{
		sources:        make(SourceMap),
		SkipGlobalSync: true,
		userFile:       userFile,
	}

	cfg, err := l.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.AI.Provider != "groq" {
		t.Errorf("ai.provider = %q, want %q", cfg.AI.Provider, "groq")
	}

	if l.sources["ai.provider"].Source != SourceUser {
		t.Errorf("ai.provider source = %v, want %v", l.sources["ai.provider"].Source, SourceUser)
	}
}

func TestLayeredLoader_EnvOverrides(t *testing.T) {
	t.Setenv("RIG_AI_PROVIDER", "groq")

	l := &LayeredLoader{
		sources:        make(SourceMap),
		SkipGlobalSync: true,
		userFile:       "/nonexistent.toml",
	}

	cfg, err := l.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.AI.Provider != "groq" {
		t.Errorf("ai.provider = %q, want %q", cfg.AI.Provider, "groq")
	}

	if l.sources["ai.provider"].Source != SourceEnv {
		t.Errorf("ai.provider source = %v, want %v", l.sources["ai.provider"].Source, SourceEnv)
	}
}

func TestLayeredLoader_Keychain(t *testing.T) {
	// Keyring mock
	keyring.MockInit()

	service := "rig-ai-anthropic"
	user := "api-key"
	secret := "sk-test-key"

	if err := keyring.Set(service, user, secret); err != nil {
		t.Fatalf("failed to set keyring: %v", err)
	}

	userConfig := `
[ai]
api_key = "keychain://rig-ai-anthropic/api-key"
`
	tmpDir := t.TempDir()
	userFile := filepath.Join(tmpDir, "user.toml")
	if err := os.WriteFile(userFile, []byte(userConfig), 0644); err != nil {
		t.Fatalf("failed to write user config: %v", err)
	}

	l := &LayeredLoader{
		sources:        make(SourceMap),
		SkipGlobalSync: true,
		userFile:       userFile,
	}

	cfg, err := l.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.AI.APIKey != secret {
		t.Errorf("ai.api_key = %q, want %q", cfg.AI.APIKey, secret)
	}

	if l.sources["ai.api_key"].Source != SourceKeychain {
		t.Errorf("ai.api_key source = %v, want %v", l.sources["ai.api_key"].Source, SourceKeychain)
	}
}
