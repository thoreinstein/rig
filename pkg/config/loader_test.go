package config

import (
	"os"
	"path/filepath"
	"reflect"
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

	t.Chdir(tmpDir)

	l := &LayeredLoader{
		sources:        make(SourceMap),
		SkipGlobalSync: true,
		projectCtx: &project.ProjectContext{
			RootPath: tmpDir,
			Origin:   tmpDir,
		},
		userFile: userFile,
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
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	l := &LayeredLoader{
		sources:        make(SourceMap),
		SkipGlobalSync: true,
		projectCtx: &project.ProjectContext{
			RootPath: tmpDir,
			Origin:   tmpDir,
		},
		userFile: filepath.Join(tmpDir, "nonexistent.toml"),
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

	t.Chdir(tmpDir)

	l := &LayeredLoader{
		sources:        make(SourceMap),
		SkipGlobalSync: true,
		userFile:       userFile,
		cwd:            tmpDir,
		projectCtx: &project.ProjectContext{
			RootPath: tmpDir,
			Origin:   tmpDir,
		},
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

func TestLayeredLoader_ImmutableKeyBlocked(t *testing.T) {
	tmpDir := t.TempDir()
	root := filepath.Join(tmpDir, "root")
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	// Project config attempts to override immutable key
	projectConfig := `
[github]
token = "malicious-token"
default_merge_method = "squash"
`
	if err := os.WriteFile(filepath.Join(root, ".rig.toml"), []byte(projectConfig), 0644); err != nil {
		t.Fatal(err)
	}

	l := &LayeredLoader{
		sources:        make(SourceMap),
		SkipGlobalSync: true,
		projectCtx: &project.ProjectContext{
			RootPath: root,
			Origin:   root,
		},
		userFile: filepath.Join(tmpDir, "user.toml"),
	}

	// Set a "good" token in defaults/user level indirectly via viper default if needed,
	// but here we just check if it's blocked.
	cfg, err := l.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// Verify immutable key NOT overridden (should be empty string if not set elsewhere)
	if cfg.GitHub.Token == "malicious-token" {
		t.Error("github.token (immutable) was overridden by project config")
	}

	// Verify non-immutable key WAS overridden
	if cfg.GitHub.DefaultMergeMethod != "squash" {
		t.Errorf("github.default_merge_method = %q, want %q", cfg.GitHub.DefaultMergeMethod, "squash")
	}

	// Verify violation recorded
	found := false
	for _, v := range l.Violations() {
		if v.Key == "github.token" && v.Reason == ViolationImmutable {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected immutable violation not found in loader")
	}
}

func TestLayeredLoader_TrustModel(t *testing.T) {
	tmpDir := t.TempDir()
	root := filepath.Join(tmpDir, "root")
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	projectConfig := `
[tmux]
session_prefix = "custom-"
`
	if err := os.WriteFile(filepath.Join(root, ".rig.toml"), []byte(projectConfig), 0644); err != nil {
		t.Fatal(err)
	}

	// Initialize TrustStore in a temp location
	tsPath := filepath.Join(tmpDir, "trust.json")
	ts := &TrustStore{
		path:    tsPath,
		trusted: make(map[string]TrustEntry),
	}

	t.Run("UntrustedProjectWarning", func(t *testing.T) {
		l := &LayeredLoader{
			sources:        make(SourceMap),
			SkipGlobalSync: true,
			projectCtx: &project.ProjectContext{
				RootPath: root,
				Origin:   root,
			},
			userFile:   filepath.Join(tmpDir, "user.toml"),
			trustStore: ts,
		}

		cfg, err := l.Load()
		if err != nil {
			t.Fatal(err)
		}

		// Non-immutable override SHOULD be applied
		if cfg.Tmux.SessionPrefix != "custom-" {
			t.Errorf("tmux.session_prefix = %q, want %q", cfg.Tmux.SessionPrefix, "custom-")
		}

		// Violation SHOULD be recorded
		found := false
		for _, v := range l.Violations() {
			if v.Reason == ViolationUntrustedProject {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected untrusted_project violation not found")
		}
	})

	t.Run("NilTrustStoreTreatedAsUntrusted", func(t *testing.T) {
		l := &LayeredLoader{
			sources:        make(SourceMap),
			SkipGlobalSync: true,
			projectCtx: &project.ProjectContext{
				RootPath: root,
				Origin:   root,
			},
			userFile:   filepath.Join(tmpDir, "user.toml"),
			trustStore: nil, // simulate failed TrustStore initialization
		}

		cfg, err := l.Load()
		if err != nil {
			t.Fatal(err)
		}

		// Override SHOULD still be applied (untrusted = warning, not block)
		if cfg.Tmux.SessionPrefix != "custom-" {
			t.Errorf("tmux.session_prefix = %q, want %q", cfg.Tmux.SessionPrefix, "custom-")
		}

		// Violation SHOULD be recorded because nil trustStore = untrusted
		found := false
		for _, v := range l.Violations() {
			if v.Reason == ViolationUntrustedProject {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected untrusted_project violation when trustStore is nil")
		}
	})

	t.Run("TrustedProjectNoWarning", func(t *testing.T) {
		if err := ts.Add(root); err != nil {
			t.Fatal(err)
		}

		l := &LayeredLoader{
			sources:        make(SourceMap),
			SkipGlobalSync: true,
			projectCtx: &project.ProjectContext{
				RootPath: root,
				Origin:   root,
			},
			userFile:   filepath.Join(tmpDir, "user.toml"),
			trustStore: ts,
		}

		_, err := l.Load()
		if err != nil {
			t.Fatal(err)
		}

		// Violation should NOT be recorded
		for _, v := range l.Violations() {
			if v.Reason == ViolationUntrustedProject {
				t.Error("unexpected untrusted_project violation found for trusted project")
			}
		}
	})
}

func TestDeleteFlatKey(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]interface{}
		key  string
		want map[string]interface{}
	}{
		{
			name: "dotted key in nested map",
			m: map[string]interface{}{
				"github": map[string]interface{}{
					"token": "secret",
					"org":   "acme",
				},
			},
			key: "github.token",
			want: map[string]interface{}{
				"github": map[string]interface{}{
					"org": "acme",
				},
			},
		},
		{
			name: "simple top-level key",
			m: map[string]interface{}{
				"foo": "bar",
			},
			key:  "foo",
			want: map[string]interface{}{},
		},
		{
			name: "non-existent intermediate path",
			m: map[string]interface{}{
				"a": "b",
			},
			key: "x.y.z",
			want: map[string]interface{}{
				"a": "b",
			},
		},
		{
			name: "empty map",
			m:    map[string]interface{}{},
			key:  "foo",
			want: map[string]interface{}{},
		},
		{
			name: "deeply nested key",
			m: map[string]interface{}{
				"a": map[string]interface{}{
					"b": map[string]interface{}{
						"c": "value",
					},
				},
			},
			key: "a.b.c",
			want: map[string]interface{}{
				"a": map[string]interface{}{
					"b": map[string]interface{}{},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deleteFlatKey(tt.m, tt.key)
			if !reflect.DeepEqual(tt.m, tt.want) {
				t.Errorf("deleteFlatKey() = %v, want %v", tt.m, tt.want)
			}
		})
	}
}

func TestLayeredLoader_ImmutableKeyNestedCascade(t *testing.T) {
	tmpDir := t.TempDir()
	root := filepath.Join(tmpDir, "root")
	sub := filepath.Join(root, "sub")
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}

	// Root .rig.toml sets immutable key AND a normal cascading key
	rootConfig := `
[github]
token = "root-token"
default_merge_method = "merge"
`
	if err := os.WriteFile(filepath.Join(root, ".rig.toml"), []byte(rootConfig), 0644); err != nil {
		t.Fatal(err)
	}

	// Sub .rig.toml overrides both
	subConfig := `
[github]
token = "sub-token"
default_merge_method = "squash"
`
	if err := os.WriteFile(filepath.Join(sub, ".rig.toml"), []byte(subConfig), 0644); err != nil {
		t.Fatal(err)
	}

	l := &LayeredLoader{
		sources:        make(SourceMap),
		SkipGlobalSync: true,
		projectCtx: &project.ProjectContext{
			RootPath: root,
			Origin:   sub,
		},
		userFile: filepath.Join(tmpDir, "user.toml"),
	}

	cfg, err := l.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// github.token is immutable — must NOT be overridden by either level
	if cfg.GitHub.Token != "" {
		t.Errorf("github.token = %q, want empty (immutable key should be blocked)", cfg.GitHub.Token)
	}

	// default_merge_method should cascade normally to the sub value
	if cfg.GitHub.DefaultMergeMethod != "squash" {
		t.Errorf("github.default_merge_method = %q, want %q", cfg.GitHub.DefaultMergeMethod, "squash")
	}

	// Should have 2 immutable violations (one per config file)
	var immutableCount int
	for _, v := range l.Violations() {
		if v.Key == "github.token" && v.Reason == ViolationImmutable {
			immutableCount++
		}
	}
	if immutableCount != 2 {
		t.Errorf("immutable violations for github.token = %d, want 2", immutableCount)
	}
}
