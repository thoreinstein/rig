package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanner_ManifestInheritanceBug(t *testing.T) {
	tmpDir := t.TempDir()

	// Create Plugin A with its own manifest
	pluginAPath := filepath.Join(tmpDir, "plugin-a")
	if err := os.WriteFile(pluginAPath, []byte("bin-a"), 0755); err != nil {
		t.Fatal(err)
	}
	manifestA := pluginAPath + ".manifest.yaml"
	if err := os.WriteFile(manifestA, []byte("name: Plugin A\nversion: 1.0.0"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create Plugin B without a manifest
	pluginBPath := filepath.Join(tmpDir, "plugin-b")
	if err := os.WriteFile(pluginBPath, []byte("bin-b"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create a root-level manifest.yaml
	rootManifest := filepath.Join(tmpDir, "manifest.yaml")
	if err := os.WriteFile(rootManifest, []byte("name: Rogue Manifest\nversion: 6.6.6"), 0644); err != nil {
		t.Fatal(err)
	}

	s := &Scanner{
		Paths: []string{tmpDir},
	}

	result, err := s.Scan()
	if err != nil {
		t.Fatal(err)
	}

	foundB := false
	for _, p := range result.Plugins {
		if filepath.Base(p.Path) == "plugin-b" {
			foundB = true
			if p.Name == "Rogue Manifest" {
				t.Errorf("BUG: plugin-b inherited root manifest name %q", p.Name)
			}
			if p.Version == "6.6.6" {
				t.Errorf("BUG: plugin-b inherited root manifest version %q", p.Version)
			}
		}
	}
	if !foundB {
		t.Fatal("plugin-b not found")
	}
}
