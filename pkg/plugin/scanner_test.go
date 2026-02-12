package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanner_Scan(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a dummy executable plugin
	pluginPath := filepath.Join(tmpDir, "test-bin")
	if err := os.WriteFile(pluginPath, []byte("#!/bin/sh\necho hi"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create a manifest for it
	manifestPath := pluginPath + ".manifest.yaml"
	manifestContent := `
name: Pretty Name
version: 1.0.0
`
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a non-executable file (should be skipped)
	if err := os.WriteFile(filepath.Join(tmpDir, "readme.txt"), []byte("not a plugin"), 0644); err != nil {
		t.Fatal(err)
	}

	s := &Scanner{
		Path: tmpDir,
	}

	result, err := s.Scan()
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	if len(result.Plugins) != 1 {
		t.Errorf("len(result.Plugins) = %d, want 1", len(result.Plugins))
	}

	p := result.Plugins[0]
	if p.Name != "Pretty Name" {
		t.Errorf("p.Name = %q, want %q", p.Name, "Pretty Name")
	}
	if p.Version != "1.0.0" {
		t.Errorf("p.Version = %q, want %q", p.Version, "1.0.0")
	}
}

func TestScanner_ScanDirectoryPluginPath(t *testing.T) {
	tmpDir := t.TempDir()
	pluginDir := filepath.Join(tmpDir, "my-plugin")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}

	execPath := filepath.Join(pluginDir, "run-me")
	if err := os.WriteFile(execPath, []byte("#!/bin/sh\necho hi"), 0755); err != nil {
		t.Fatal(err)
	}

	s := &Scanner{
		Path: tmpDir,
	}

	result, err := s.Scan()
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(result.Plugins))
	}

	p := result.Plugins[0]
	if p.Path != execPath {
		t.Errorf("p.Path = %q, want %q", p.Path, execPath)
	}
}
