package cmd

import (
	"os"
	"path/filepath"
	"reflect"
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
					if c.Short != tc.wantCmdShort {
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
	// Setup a temporary plugin directory
	tmpDir := t.TempDir()
	pluginDir := filepath.Join(tmpDir, ".rig", "plugins")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("failed to create plugin dir: %v", err)
	}

	// Create a dummy executable
	pluginPath := filepath.Join(pluginDir, "incompatible-plugin")
	if err := os.WriteFile(pluginPath, []byte("#!/bin/sh\necho test"), 0755); err != nil {
		t.Fatalf("failed to write dummy plugin: %v", err)
	}

	// Create a manifest with commands that requires Rig >= 2.0.0
	manifestPath := filepath.Join(pluginDir, "incompatible-plugin.manifest.yaml")
	manifestContent := `
name: incompatible-plugin
version: 1.0.0
requirements:
  rig: ">= 2.0.0"
commands:
  - name: fail-cmd
    short: "Should not be registered"
`
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	// Setup git root so scanner finds it
	if err := os.Mkdir(filepath.Join(tmpDir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(tmpDir)

	// Set Rig version to 1.0.0 (incompatible with >= 2.0.0)
	oldVersion := Version
	Version = "1.0.0"
	defer func() { Version = oldVersion }()

	t.Setenv("GO_TEST", "true")

	// Reset rootCmd for test
	oldRootCmd := rootCmd
	rootCmd = &cobra.Command{Use: "rig"}
	defer func() { rootCmd = oldRootCmd }()

	registerPluginCommands()

	// Verify command was NOT registered
	for _, c := range rootCmd.Commands() {
		if c.Name() == "fail-cmd" {
			t.Error("fail-cmd was registered even though plugin is incompatible")
		}
	}
}

func TestRegisterPluginCommands_Collision(t *testing.T) {
	// Setup a temporary plugin directory
	tmpDir := t.TempDir()
	pluginDir := filepath.Join(tmpDir, ".rig", "plugins")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("failed to create plugin dir: %v", err)
	}

	// Create two plugins: one that collides with built-in, and one that collides with another plugin

	// Plugin 1: name collides with built-in "version", alias collides with built-in "v" (fake alias)
	// Actually, let's just use "help" as it's built-in.
	p1Path := filepath.Join(pluginDir, "p1")
	if err := os.WriteFile(p1Path, []byte("#!/bin/sh\necho p1"), 0755); err != nil {
		t.Fatal(err)
	}

	p1Manifest := `
name: p1
version: 1.0.0
commands:
  - name: version
    short: "Collides with built-in"
  - name: p1-cmd
    short: "Should be registered"
    aliases: ["help"]
`
	if err := os.WriteFile(p1Path+".manifest.yaml", []byte(p1Manifest), 0644); err != nil {
		t.Fatal(err)
	}

	// Plugin 2: alias collides with p1-cmd
	p2Path := filepath.Join(pluginDir, "p2")
	if err := os.WriteFile(p2Path, []byte("#!/bin/sh\necho p2"), 0755); err != nil {
		t.Fatal(err)
	}

	p2Manifest := `
name: p2
version: 1.0.0
commands:
  - name: p2-cmd
    short: "Should be registered"
    aliases: ["p1-cmd"]
`
	if err := os.WriteFile(p2Path+".manifest.yaml", []byte(p2Manifest), 0644); err != nil {
		t.Fatal(err)
	}

	// Setup git root
	if err := os.Mkdir(filepath.Join(tmpDir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(tmpDir)

	t.Setenv("GO_TEST", "true")

	// Reset rootCmd
	oldRootCmd := rootCmd
	rootCmd = &cobra.Command{Use: "rig"}
	// Add dummy built-in commands to test against
	rootCmd.AddCommand(&cobra.Command{Use: "version"})
	rootCmd.AddCommand(&cobra.Command{Use: "help"})
	defer func() { rootCmd = oldRootCmd }()

	registerPluginCommands()

	// Verify p1 collisions
	foundP1 := false
	for _, c := range rootCmd.Commands() {
		if c.Name() == "version" && c.Short == "Collides with built-in" {
			t.Error("plugin command 'version' should have been skipped")
		}
		if c.Name() == "p1-cmd" {
			foundP1 = true
			for _, a := range c.Aliases {
				if a == "help" {
					t.Error("alias 'help' should have been filtered out")
				}
			}
		}
	}
	if !foundP1 {
		t.Error("p1-cmd should have been registered (only its alias was bad)")
	}

	// Verify p2 collisions
	foundP2 := false
	for _, c := range rootCmd.Commands() {
		if c.Name() == "p2-cmd" {
			foundP2 = true
			for _, a := range c.Aliases {
				if a == "p1-cmd" {
					t.Error("alias 'p1-cmd' should have been filtered out (collides with p1-cmd name)")
				}
			}
		}
	}
	if !foundP2 {
		t.Error("p2-cmd should have been registered")
	}
}

func TestRegisterPluginCommands_Reserved(t *testing.T) {
	// Setup a temporary plugin directory
	tmpDir := t.TempDir()
	pluginDir := filepath.Join(tmpDir, ".rig", "plugins")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("failed to create plugin dir: %v", err)
	}

	// Create a plugin that tries to claim "help" and "h"
	pPath := filepath.Join(pluginDir, "bad-plugin")
	if err := os.WriteFile(pPath, []byte("#!/bin/sh\necho bad"), 0755); err != nil {
		t.Fatal(err)
	}

	pManifest := `
name: bad-plugin
version: 1.0.0
commands:
  - name: help
    short: "Trying to override help"
  - name: my-cmd
    short: "Valid command"
    aliases: ["h", "completion"]
`
	if err := os.WriteFile(pPath+".manifest.yaml", []byte(pManifest), 0644); err != nil {
		t.Fatal(err)
	}

	// Setup git root
	if err := os.Mkdir(filepath.Join(tmpDir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(tmpDir)

	t.Setenv("GO_TEST", "true")

	// Reset rootCmd (WITHOUT adding help/h/completion manually)
	oldRootCmd := rootCmd
	rootCmd = &cobra.Command{Use: "rig"}
	defer func() { rootCmd = oldRootCmd }()

	registerPluginCommands()

	// Verify reservations
	for _, c := range rootCmd.Commands() {
		if c.Name() == "help" {
			t.Error("plugin should not have been allowed to register 'help' command")
		}
		if c.Name() == "my-cmd" {
			for _, a := range c.Aliases {
				if a == "h" || a == "completion" {
					t.Errorf("alias %q should have been filtered out as reserved", a)
				}
			}
		}
	}
}

func TestPreParseGlobalFlags(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	// Reset global variables
	cfgFile = ""
	verbose = false

	os.Args = []string{"rig", "--config", "custom.toml", "-v"}
	cfgFile, verbose = bootstrap.PreParseGlobalFlags(os.Args)

	if cfgFile != "custom.toml" {
		t.Errorf("cfgFile = %q, want %q", cfgFile, "custom.toml")
	}
	if !verbose {
		t.Error("verbose should be true")
	}

	// Test shorthand config
	cfgFile = ""
	os.Args = []string{"rig", "-C", "short.toml"}
	cfgFile, verbose = bootstrap.PreParseGlobalFlags(os.Args)
	if cfgFile != "short.toml" {
		t.Errorf("cfgFile = %q, want %q", cfgFile, "short.toml")
	}

	// Test flag with equals
	cfgFile = ""
	os.Args = []string{"rig", "--config=equals.toml"}
	cfgFile, verbose = bootstrap.PreParseGlobalFlags(os.Args)
	if cfgFile != "equals.toml" {
		t.Errorf("cfgFile = %q, want %q", cfgFile, "equals.toml")
	}

	// Test shorthand with equals
	cfgFile = ""
	os.Args = []string{"rig", "-C=shorthand_equals.toml"}
	cfgFile, verbose = bootstrap.PreParseGlobalFlags(os.Args)
	if cfgFile != "shorthand_equals.toml" {
		t.Errorf("cfgFile = %q, want %q", cfgFile, "shorthand_equals.toml")
	}

	// Test shorthand with attached value
	cfgFile = ""
	os.Args = []string{"rig", "-Cattached.toml"}
	cfgFile, verbose = bootstrap.PreParseGlobalFlags(os.Args)
	if cfgFile != "attached.toml" {
		t.Errorf("cfgFile = %q, want %q", cfgFile, "attached.toml")
	}
}

func TestPreParseGlobalFlags_StopsAtSubcommand(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	// Reset global variables
	cfgFile = ""
	verbose = false

	// rig --verbose my-subcommand --config plugin.yaml
	// The host should pick up --verbose but NOT --config plugin.yaml
	os.Args = []string{"rig", "--verbose", "my-subcommand", "--config", "plugin.yaml"}
	cfgFile, verbose = bootstrap.PreParseGlobalFlags(os.Args)

	if !verbose {
		t.Error("verbose should be true")
	}
	if cfgFile != "" {
		t.Errorf("cfgFile should be empty (intercepted plugin flag), got %q", cfgFile)
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
			gotPlugin, gotHost := filterHostFlags(tt.args)

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
