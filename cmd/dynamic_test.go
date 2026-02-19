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
