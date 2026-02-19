package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanner_ExtensionStripping(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a plugin with an extension
	pluginPath := filepath.Join(tmpDir, "my-tool.sh")
	if err := os.WriteFile(pluginPath, []byte("#!/bin/sh\necho hi"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create a manifest without the extension
	manifestPath := filepath.Join(tmpDir, "my-tool.manifest.yaml")
	manifestContent := `
name: Stripped Name
version: 2.0.0
`
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0644); err != nil {
		t.Fatal(err)
	}

	s := &Scanner{
		Paths: []string{tmpDir},
	}

	result, err := s.Scan()
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, p := range result.Plugins {
		if filepath.Base(p.Path) == "my-tool.sh" {
			found = true
			if p.Name != "Stripped Name" {
				t.Errorf("p.Name = %q, want %q (extension was not stripped during manifest lookup)", p.Name, "Stripped Name")
			}
		}
	}

	if !found {
		t.Error("my-tool.sh was not discovered")
	}
}
