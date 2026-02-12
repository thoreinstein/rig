package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadManifest(t *testing.T) {
	tmpDir := t.TempDir()

	manifestPath := filepath.Join(tmpDir, "manifest.yaml")
	content := `
name: test-plugin
version: 1.2.3
description: A test plugin
author: Jim Myers
requirements:
  rig: ">= 1.0.0"
`
	if err := os.WriteFile(manifestPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	manifest, err := loadManifest(manifestPath)
	if err != nil {
		t.Fatalf("loadManifest() error = %v", err)
	}

	if manifest.Name != "test-plugin" {
		t.Errorf("manifest.Name = %q, want %q", manifest.Name, "test-plugin")
	}
	if manifest.Version != "1.2.3" {
		t.Errorf("manifest.Version = %q, want %q", manifest.Version, "1.2.3")
	}
	if manifest.Requirements.Rig != ">= 1.0.0" {
		t.Errorf("manifest.Requirements.Rig = %q, want %q", manifest.Requirements.Rig, ">= 1.0.0")
	}
}
