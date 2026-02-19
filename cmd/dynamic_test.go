package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func TestRegisterPluginCommands(t *testing.T) {
	// Setup a temporary plugin directory
	tmpDir := t.TempDir()
	pluginDir := filepath.Join(tmpDir, "plugins")
	if err := os.Mkdir(pluginDir, 0755); err != nil {
		t.Fatalf("failed to create plugin dir: %v", err)
	}

	// Create a dummy executable
	pluginPath := filepath.Join(pluginDir, "test-plugin")
	if err := os.WriteFile(pluginPath, []byte("#!/bin/sh\necho test"), 0755); err != nil {
		t.Fatalf("failed to write dummy plugin: %v", err)
	}

	// Create a manifest with commands
	manifestPath := filepath.Join(pluginDir, "test-plugin.manifest.yaml")
	manifestContent := `
name: test-plugin
version: 1.0.0
commands:
  - name: test-cmd
    short: "Test command"
`
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	// Mock findGitRoot to return our temp dir
	// We need to inject the search path somehow.
	// Since registerPluginCommands uses findGitRoot() and hardcoded paths,
	// we can try to point it to our temp dir.

	// Wait, registerPluginCommands uses:
	// 1. ~/.config/rig/plugins
	// 2. <git-root>/.rig/plugins

	// I'll create <tmpDir>/.rig/plugins
	projectPluginDir := filepath.Join(tmpDir, ".rig", "plugins")
	if err := os.MkdirAll(projectPluginDir, 0755); err != nil {
		t.Fatalf("failed to create project plugin dir: %v", err)
	}

	// Move files to project plugin dir
	if err := os.Rename(pluginPath, filepath.Join(projectPluginDir, "test-plugin")); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(manifestPath, filepath.Join(projectPluginDir, "test-plugin.manifest.yaml")); err != nil {
		t.Fatal(err)
	}

	// Change CWD to tmpDir so findGitRoot finds it (if it looks for .git)
	// But findGitRoot looks for .git directory.
	if err := os.Mkdir(filepath.Join(tmpDir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	t.Chdir(tmpDir)

	// We need to ensure loadConfig doesn't fail
	// initConfig already ran, so appConfig might be set.
	// In test, loadConfig reloads if GO_TEST=true.
	t.Setenv("GO_TEST", "true")

	// Reset rootCmd for test
	oldRootCmd := rootCmd
	rootCmd = &cobra.Command{Use: "rig"}
	defer func() { rootCmd = oldRootCmd }()

	registerPluginCommands()

	// Verify command was registered
	found := false
	for _, c := range rootCmd.Commands() {
		if c.Name() == "test-cmd" {
			found = true
			if c.Short != "Test command" {
				t.Errorf("cmd.Short = %q, want %q", c.Short, "Test command")
			}
			break
		}
	}

	if !found {
		t.Error("test-cmd was not registered")
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
	preParseGlobalFlags()

	if cfgFile != "custom.toml" {
		t.Errorf("cfgFile = %q, want %q", cfgFile, "custom.toml")
	}
	if !verbose {
		t.Error("verbose should be true")
	}

	// Test shorthand config
	cfgFile = ""
	os.Args = []string{"rig", "-c", "short.toml"}
	preParseGlobalFlags()
	if cfgFile != "short.toml" {
		t.Errorf("cfgFile = %q, want %q", cfgFile, "short.toml")
	}

	// Test flag with equals
	cfgFile = ""
	os.Args = []string{"rig", "--config=equals.toml"}
	preParseGlobalFlags()
	if cfgFile != "equals.toml" {
		t.Errorf("cfgFile = %q, want %q", cfgFile, "equals.toml")
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
	preParseGlobalFlags()

	if !verbose {
		t.Error("verbose should be true")
	}
	if cfgFile != "" {
		t.Errorf("cfgFile should be empty (intercepted plugin flag), got %q", cfgFile)
	}
}
