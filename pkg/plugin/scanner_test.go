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
		Paths: []string{tmpDir},
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
		Paths: []string{tmpDir},
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

func TestScanner_ScanMultiplePaths(t *testing.T) {
	systemDir := t.TempDir()
	projectDir := t.TempDir()

	// System plugin
	sysPlugin := filepath.Join(systemDir, "sys-plugin")
	if err := os.WriteFile(sysPlugin, []byte("#!/bin/sh\necho sys"), 0755); err != nil {
		t.Fatal(err)
	}

	// Project plugin (different name)
	projPlugin := filepath.Join(projectDir, "proj-plugin")
	if err := os.WriteFile(projPlugin, []byte("#!/bin/sh\necho proj"), 0755); err != nil {
		t.Fatal(err)
	}

	s := &Scanner{
		Paths: []string{systemDir, projectDir},
	}

	result, err := s.Scan()
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Plugins) != 2 {
		t.Fatalf("expected 2 plugins, got %d", len(result.Plugins))
	}

	names := map[string]bool{}
	for _, p := range result.Plugins {
		names[p.Name] = true
	}
	if !names["sys-plugin"] || !names["proj-plugin"] {
		t.Errorf("expected both sys-plugin and proj-plugin, got %v", names)
	}
}

func TestScanner_ProjectOverridesSystem(t *testing.T) {
	systemDir := t.TempDir()
	projectDir := t.TempDir()

	// System plugin named "shared"
	sysPlugin := filepath.Join(systemDir, "shared")
	if err := os.WriteFile(sysPlugin, []byte("#!/bin/sh\necho sys"), 0755); err != nil {
		t.Fatal(err)
	}

	// Project plugin also named "shared" — should override
	projPlugin := filepath.Join(projectDir, "shared")
	if err := os.WriteFile(projPlugin, []byte("#!/bin/sh\necho proj"), 0755); err != nil {
		t.Fatal(err)
	}

	s := &Scanner{
		Paths: []string{systemDir, projectDir},
	}

	result, err := s.Scan()
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Plugins) != 1 {
		t.Fatalf("expected 1 plugin (deduped), got %d", len(result.Plugins))
	}

	p := result.Plugins[0]
	if p.Path != projPlugin {
		t.Errorf("expected project plugin path %q, got %q", projPlugin, p.Path)
	}
	if p.Source != "project" {
		t.Errorf("expected source %q, got %q", "project", p.Source)
	}
}

func TestScanner_EmptyPathSkipped(t *testing.T) {
	validDir := t.TempDir()
	missingDir := filepath.Join(t.TempDir(), "does-not-exist")

	plugin := filepath.Join(validDir, "test-bin")
	if err := os.WriteFile(plugin, []byte("#!/bin/sh\necho hi"), 0755); err != nil {
		t.Fatal(err)
	}

	s := &Scanner{
		Paths: []string{missingDir, validDir},
	}

	result, err := s.Scan()
	if err != nil {
		t.Fatalf("Scan() should not error on missing path: %v", err)
	}

	if len(result.Plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(result.Plugins))
	}
	if result.Plugins[0].Name != "test-bin" {
		t.Errorf("expected plugin name %q, got %q", "test-bin", result.Plugins[0].Name)
	}
}
