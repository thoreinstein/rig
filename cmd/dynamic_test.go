package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"thoreinstein.com/rig/pkg/bootstrap"
)

func TestRegisterPluginCommands(t *testing.T) {
	tests := []struct {
		name          string
		setup         func(t *testing.T, tmpDir string)
		wantCmdName   string
		wantCmdShort  string
		wantCmdExists bool
	}{
		{
			name: "plugin in project-local directory",
			setup: func(t *testing.T, tmpDir string) {
				projectPluginDir := filepath.Join(tmpDir, ".rig", "plugins")
				if err := os.MkdirAll(projectPluginDir, 0755); err != nil {
					t.Fatalf("failed to create project plugin dir: %v", err)
				}

				// Create a dummy executable
				pluginPath := filepath.Join(projectPluginDir, "test-plugin")
				if err := os.WriteFile(pluginPath, []byte("#!/bin/sh\necho test"), 0755); err != nil {
					t.Fatalf("failed to write dummy plugin: %v", err)
				}

				// Create a manifest with commands
				manifestContent := "name: test-plugin\nversion: 1.0.0\ncommands:\n  - name: test-cmd\n    short: \"Test command\"\n"
				if err := os.WriteFile(pluginPath+".manifest.yaml", []byte(manifestContent), 0644); err != nil {
					t.Fatalf("failed to write manifest: %v", err)
				}

				// Setup git root
				if err := os.Mkdir(filepath.Join(tmpDir, ".git"), 0755); err != nil {
					t.Fatal(err)
				}
				t.Chdir(tmpDir)
			},
			wantCmdName:   "test-cmd",
			wantCmdShort:  "Test command",
			wantCmdExists: true,
		},
		{
			name: "plugin in user config directory",
			setup: func(t *testing.T, tmpDir string) {
				userPluginDir := filepath.Join(tmpDir, ".config", "rig", "plugins")
				if err := os.MkdirAll(userPluginDir, 0755); err != nil {
					t.Fatalf("failed to create user plugin dir: %v", err)
				}

				t.Setenv("HOME", tmpDir)

				pluginPath := filepath.Join(userPluginDir, "user-plugin")
				if err := os.WriteFile(pluginPath, []byte("#!/bin/sh\necho user"), 0755); err != nil {
					t.Fatal(err)
				}

				manifestContent := "name: user-plugin\nversion: 1.0.0\ncommands:\n  - name: user-cmd\n    short: \"User command\"\n"
				if err := os.WriteFile(pluginPath+".manifest.yaml", []byte(manifestContent), 0644); err != nil {
					t.Fatal(err)
				}
			},
			wantCmdName:   "user-cmd",
			wantCmdShort:  "User command",
			wantCmdExists: true,
		},
		{
			name: "plugin missing manifest",
			setup: func(t *testing.T, tmpDir string) {
				projectPluginDir := filepath.Join(tmpDir, ".rig", "plugins")
				if err := os.MkdirAll(projectPluginDir, 0755); err != nil {
					t.Fatal(err)
				}

				pluginPath := filepath.Join(projectPluginDir, "no-manifest-plugin")
				if err := os.WriteFile(pluginPath, []byte("#!/bin/sh\necho test"), 0755); err != nil {
					t.Fatal(err)
				}

				if err := os.Mkdir(filepath.Join(tmpDir, ".git"), 0755); err != nil {
					t.Fatal(err)
				}
				t.Chdir(tmpDir)
			},
			wantCmdName:   "no-manifest-cmd",
			wantCmdExists: false,
		},
		{
			name: "plugin with invalid manifest format",
			setup: func(t *testing.T, tmpDir string) {
				projectPluginDir := filepath.Join(tmpDir, ".rig", "plugins")
				if err := os.MkdirAll(projectPluginDir, 0755); err != nil {
					t.Fatal(err)
				}

				pluginPath := filepath.Join(projectPluginDir, "bad-manifest-plugin")
				if err := os.WriteFile(pluginPath, []byte("#!/bin/sh\necho test"), 0755); err != nil {
					t.Fatal(err)
				}

				if err := os.WriteFile(pluginPath+".manifest.yaml", []byte("invalid: toml: :"), 0644); err != nil {
					t.Fatal(err)
				}

				if err := os.Mkdir(filepath.Join(tmpDir, ".git"), 0755); err != nil {
					t.Fatal(err)
				}
				t.Chdir(tmpDir)
			},
			wantCmdName:   "bad-manifest-cmd",
			wantCmdExists: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			tc.setup(t, tmpDir)

			t.Setenv("GO_TEST", "true")

			// Reset rootCmd for test
			oldRootCmd := rootCmd
			rootCmd = &cobra.Command{Use: "rig"}
			defer func() { rootCmd = oldRootCmd }()

			registerPluginCommands()

			// Verify command registration
			found := false
			for _, c := range rootCmd.Commands() {
				if c.Name() == tc.wantCmdName {
					found = true
					if tc.wantCmdShort != "" && c.Short != tc.wantCmdShort {
						t.Errorf("cmd.Short = %q, want %q", c.Short, tc.wantCmdShort)
					}
					break
				}
			}

			if found != tc.wantCmdExists {
				t.Errorf("command %q existence = %v, want %v", tc.wantCmdName, found, tc.wantCmdExists)
			}
		})
	}
}

func TestRegisterPluginCommands_Incompatible(t *testing.T) {
	tests := []struct {
		name                 string
		manifestRequirements string
		rigVersion           string
		wantRegistered       bool
	}{
		{
			name:                 "Incompatible version constraint (too high)",
			manifestRequirements: ">= 2.0.0",
			rigVersion:           "1.0.0",
			wantRegistered:       false,
		},
		{
			name:                 "Compatible version constraint",
			manifestRequirements: ">= 1.0.0",
			rigVersion:           "1.2.3",
			wantRegistered:       true,
		},
		{
			name:                 "Incompatible version constraint (too low)",
			manifestRequirements: "< 1.0.0",
			rigVersion:           "1.5.0",
			wantRegistered:       false,
		},
		{
			name:                 "Compatible tilde constraint",
			manifestRequirements: "~> 1.5.0",
			rigVersion:           "1.5.2",
			wantRegistered:       true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			pluginDir := filepath.Join(tmpDir, ".rig", "plugins")
			if err := os.MkdirAll(pluginDir, 0755); err != nil {
				t.Fatalf("failed to create plugin dir: %v", err)
			}

			// Create a dummy executable
			pluginPath := filepath.Join(pluginDir, "version-plugin")
			if err := os.WriteFile(pluginPath, []byte("#!/bin/sh\necho test"), 0755); err != nil {
				t.Fatalf("failed to write dummy plugin: %v", err)
			}

			// Create a manifest with commands
			manifestContent := fmt.Sprintf("name: version-plugin\nversion: 1.0.0\nrequirements:\n  rig: %q\ncommands:\n  - name: version-cmd\n    short: \"Version command\"\n", tc.manifestRequirements)
			if err := os.WriteFile(pluginPath+".manifest.yaml", []byte(manifestContent), 0644); err != nil {
				t.Fatalf("failed to write manifest: %v", err)
			}

			// Setup git root
			if err := os.Mkdir(filepath.Join(tmpDir, ".git"), 0755); err != nil {
				t.Fatal(err)
			}

			// Set Rig version
			oldVersion := Version
			Version = tc.rigVersion
			defer func() { Version = oldVersion }()

			t.Setenv("GO_TEST", "true")
			t.Chdir(tmpDir)

			// Reset rootCmd
			oldRootCmd := rootCmd
			rootCmd = &cobra.Command{Use: "rig"}
			defer func() { rootCmd = oldRootCmd }()

			registerPluginCommands()

			// Verify command registration
			found := false
			for _, c := range rootCmd.Commands() {
				if c.Name() == "version-cmd" {
					found = true
					break
				}
			}

			if found != tc.wantRegistered {
				t.Errorf("command registered = %v, want %v", found, tc.wantRegistered)
			}
		})
	}
}

func TestRegisterPluginCommands_Collision(t *testing.T) {
	tests := []struct {
		name          string
		setup         func(t *testing.T, pluginDir string)
		wantCommands  []string // Primary names that should exist
		wantForbidden []string // Names/aliases that should NOT exist
	}{
		{
			name: "collision with built-in command name",
			setup: func(t *testing.T, pluginDir string) {
				pPath := filepath.Join(pluginDir, "collision-p")
				_ = os.WriteFile(pPath, []byte("#!/bin/sh\necho collision"), 0755)
				manifest := "name: collision-p\nversion: 1.0.0\ncommands:\n  - name: version\n    short: \"Collides with built-in\"\n"
				_ = os.WriteFile(pPath+".manifest.yaml", []byte(manifest), 0644)
			},
			wantForbidden: []string{"version"}, // The one with the short description should be missing
		},
		{
			name: "collision with built-in alias",
			setup: func(t *testing.T, pluginDir string) {
				pPath := filepath.Join(pluginDir, "alias-p")
				_ = os.WriteFile(pPath, []byte("#!/bin/sh\necho alias"), 0755)
				manifest := "name: alias-p\nversion: 1.0.0\ncommands:\n  - name: my-cmd\n    short: \"Valid\"\n    aliases: [\"help\"]\n"
				_ = os.WriteFile(pPath+".manifest.yaml", []byte(manifest), 0644)
			},
			wantCommands:  []string{"my-cmd"},
			wantForbidden: []string{"help"}, // Should be the built-in help, not plugin's alias
		},
		{
			name: "collision between two plugins",
			setup: func(t *testing.T, pluginDir string) {
				// Plugin 1
				p1 := filepath.Join(pluginDir, "p1")
				_ = os.WriteFile(p1, []byte("#!/bin/sh\necho p1"), 0755)
				_ = os.WriteFile(p1+".manifest.yaml", []byte("name: p1\nversion: 1.0.0\ncommands:\n  - name: shared-cmd\n"), 0644)

				// Plugin 2 tries to use same name
				p2 := filepath.Join(pluginDir, "p2")
				_ = os.WriteFile(p2, []byte("#!/bin/sh\necho p2"), 0755)
				_ = os.WriteFile(p2+".manifest.yaml", []byte("name: p2\nversion: 1.0.0\ncommands:\n  - name: shared-cmd\n    short: \"Second plugin\"\n"), 0644)
			},
			wantCommands: []string{"shared-cmd"}, // First one wins
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			pluginDir := filepath.Join(tmpDir, ".rig", "plugins")
			_ = os.MkdirAll(pluginDir, 0755)

			tc.setup(t, pluginDir)

			if err := os.Mkdir(filepath.Join(tmpDir, ".git"), 0755); err != nil {
				t.Fatal(err)
			}
			t.Chdir(tmpDir)

			t.Setenv("GO_TEST", "true")

			// Reset rootCmd
			oldRootCmd := rootCmd
			rootCmd = &cobra.Command{Use: "rig"}
			// Add dummy built-in commands
			rootCmd.AddCommand(&cobra.Command{Use: "version", Short: "Built-in version"})
			rootCmd.AddCommand(&cobra.Command{Use: "help", Short: "Built-in help"})
			defer func() { rootCmd = oldRootCmd }()

			registerPluginCommands()

			// Check expected commands
			for _, name := range tc.wantCommands {
				found := false
				for _, c := range rootCmd.Commands() {
					if c.Name() == name {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected command %q to be registered", name)
				}
			}

			// Check forbidden commands/shadowing
			for _, name := range tc.wantForbidden {
				for _, c := range rootCmd.Commands() {
					if c.Name() == name && !strings.Contains(c.Short, "Built-in") {
						t.Errorf("command %q was registered by plugin even though it collides", name)
					}
				}
			}
		})
	}
}

func TestRegisterPluginCommands_Reserved(t *testing.T) {
	tests := []struct {
		name          string
		manifest      string
		wantForbidden []string // Reserved names that should NOT be registered
		wantAllowed   []string // Other names that should be registered
	}{
		{
			name:          "plugin tries to claim help and h",
			manifest:      "name: bad-plugin\nversion: 1.0.0\ncommands:\n  - name: help\n  - name: my-cmd\n    aliases: [\"h\"]\n",
			wantForbidden: []string{"help", "h"},
			wantAllowed:   []string{"my-cmd"},
		},
		{
			name:          "plugin tries to claim completion",
			manifest:      "name: completion-plugin\nversion: 1.0.0\ncommands:\n  - name: completion\n",
			wantForbidden: []string{"completion"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			pluginDir := filepath.Join(tmpDir, ".rig", "plugins")
			_ = os.MkdirAll(pluginDir, 0755)

			pPath := filepath.Join(pluginDir, "p")
			_ = os.WriteFile(pPath, []byte("#!/bin/sh\necho test"), 0755)
			_ = os.WriteFile(pPath+".manifest.yaml", []byte(tc.manifest), 0644)

			if err := os.Mkdir(filepath.Join(tmpDir, ".git"), 0755); err != nil {
				t.Fatal(err)
			}
			t.Chdir(tmpDir)

			t.Setenv("GO_TEST", "true")

			// Reset rootCmd
			oldRootCmd := rootCmd
			rootCmd = &cobra.Command{Use: "rig"}
			defer func() { rootCmd = oldRootCmd }()

			registerPluginCommands()

			// Verify reservations
			for _, name := range tc.wantForbidden {
				for _, c := range rootCmd.Commands() {
					if c.Name() == name {
						t.Errorf("reserved name %q was registered as a command", name)
					}
					for _, a := range c.Aliases {
						if a == name {
							t.Errorf("reserved name %q was registered as an alias", name)
						}
					}
				}
			}

			// Verify allowed
			for _, name := range tc.wantAllowed {
				found := false
				for _, c := range rootCmd.Commands() {
					if c.Name() == name {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected allowed command %q to be registered", name)
				}
			}
		})
	}
}

func TestPreParseGlobalFlags(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	tests := []struct {
		name        string
		args        []string
		wantCfg     string
		wantVerbose bool
	}{
		{
			name:        "Long config flag and verbose",
			args:        []string{"rig", "--config", "custom.toml", "-v"},
			wantCfg:     "custom.toml",
			wantVerbose: true,
		},
		{
			name:    "Shorthand config flag",
			args:    []string{"rig", "-C", "short.toml"},
			wantCfg: "short.toml",
		},
		{
			name:    "Long config flag with equals",
			args:    []string{"rig", "--config=equals.toml"},
			wantCfg: "equals.toml",
		},
		{
			name:    "Shorthand config flag with equals",
			args:    []string{"rig", "-C=shorthand_equals.toml"},
			wantCfg: "shorthand_equals.toml",
		},
		{
			name:    "Shorthand config flag with attached value",
			args:    []string{"rig", "-Cattached.toml"},
			wantCfg: "attached.toml",
		},
		{
			name:        "Stop parsing at subcommand",
			args:        []string{"rig", "--verbose", "my-subcommand", "--config", "plugin.yaml"},
			wantVerbose: true,
			wantCfg:     "", // Should NOT intercept plugin flag
		},
		{
			name:        "Stop parsing at double dash",
			args:        []string{"rig", "--verbose", "--", "--config", "plugin.yaml"},
			wantVerbose: true,
			wantCfg:     "", // Should NOT intercept flag after --
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset global variables
			cfgFile = ""
			verbose = false

			os.Args = tt.args
			gotCfg, gotVerbose := bootstrap.PreParseGlobalFlags(os.Args)

			if gotCfg != tt.wantCfg {
				t.Errorf("PreParseGlobalFlags() cfgFile = %q, want %q", gotCfg, tt.wantCfg)
			}
			if gotVerbose != tt.wantVerbose {
				t.Errorf("PreParseGlobalFlags() verbose = %v, want %v", gotVerbose, tt.wantVerbose)
			}
		})
	}
}

func TestFilterHostFlags(t *testing.T) {
	// Setup test args
	// --verbose is boolean host flag
	// --config is non-boolean host flag
	tests := []struct {
		name       string
		args       []string
		wantPlugin []string
		wantHost   []string
	}{
		{
			name:       "No host flags",
			args:       []string{"arg1", "arg2"},
			wantPlugin: []string{"arg1", "arg2"},
			wantHost:   []string{},
		},
		{
			name:       "Boolean host flag",
			args:       []string{"--verbose", "arg1"},
			wantPlugin: []string{"arg1"},
			wantHost:   []string{"--verbose"},
		},
		{
			name:       "Non-boolean host flag with value",
			args:       []string{"--config", "custom.toml", "arg1"},
			wantPlugin: []string{"arg1"},
			wantHost:   []string{"--config", "custom.toml"},
		},
		{
			name:       "Mixed flags",
			args:       []string{"--verbose", "arg1", "--config", "c.toml", "--plugin-flag"},
			wantPlugin: []string{"arg1", "--plugin-flag"},
			wantHost:   []string{"--verbose", "--config", "c.toml"},
		},
		{
			name:       "Double dash preserves everything after",
			args:       []string{"--verbose", "--", "--verbose", "arg1"},
			wantPlugin: []string{"--", "--verbose", "arg1"},
			wantHost:   []string{"--verbose"},
		},
		{
			name:       "Shorthand flags",
			args:       []string{"-v", "-C", "c.toml", "arg1"},
			wantPlugin: []string{"arg1"},
			wantHost:   []string{"-v", "-C", "c.toml"},
		},
		{
			name:       "Flag with equals sign",
			args:       []string{"--config=c.toml", "arg1"},
			wantPlugin: []string{"arg1"},
			wantHost:   []string{"--config=c.toml"},
		},
		{
			name:       "Ambiguous plugin shorthand NOT consumed by host",
			args:       []string{"-cfoo", "arg1"},
			wantPlugin: []string{"-cfoo", "arg1"},
			wantHost:   []string{},
		},
		{
			name:       "Attached host shorthand consumed",
			args:       []string{"-Cattached.toml", "arg1"},
			wantPlugin: []string{"arg1"},
			wantHost:   []string{"-Cattached.toml"},
		},
		{
			name:       "Boolean shorthand prefix NOT consumed by host",
			args:       []string{"-vfoo", "arg1"},
			wantPlugin: []string{"-vfoo", "arg1"},
			wantHost:   []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPlugin, gotHost := bootstrap.FilterHostFlags(rootCmd.PersistentFlags(), tt.args)

			if !reflect.DeepEqual(gotPlugin, tt.wantPlugin) {
				t.Errorf("filterHostFlags() pluginArgs = %v, want %v", gotPlugin, tt.wantPlugin)
			}

			// Use empty slice instead of nil for comparison
			if len(gotHost) == 0 && len(tt.wantHost) == 0 {
				return
			}
			if !reflect.DeepEqual(gotHost, tt.wantHost) {
				t.Errorf("filterHostFlags() hostArgs = %v, want %v", gotHost, tt.wantHost)
			}
		})
	}
}
