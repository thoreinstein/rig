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

func TestLoadManifest_WithCommands(t *testing.T) {
	tests := []struct {
		name              string
		manifestPath      string
		wantCommandsLen   int
		wantCmdName       string
		wantCmdShort      string
		wantCmdAliasesLen int
	}{
		{
			name:              "Manifest with commands",
			manifestPath:      filepath.Join("testdata", "manifests", "with_commands.yaml"),
			wantCommandsLen:   1,
			wantCmdName:       "echo",
			wantCmdShort:      "Echo arguments",
			wantCmdAliasesLen: 2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			manifest, err := loadManifest(tc.manifestPath)
			if err != nil {
				t.Fatalf("loadManifest() error = %v", err)
			}

			if len(manifest.Commands) != tc.wantCommandsLen {
				t.Fatalf("len(manifest.Commands) = %d, want %d", len(manifest.Commands), tc.wantCommandsLen)
			}

			if tc.wantCommandsLen > 0 {
				cmd := manifest.Commands[0]
				if cmd.Name != tc.wantCmdName {
					t.Errorf("cmd.Name = %q, want %q", cmd.Name, tc.wantCmdName)
				}
				if cmd.Short != tc.wantCmdShort {
					t.Errorf("cmd.Short = %q, want %q", cmd.Short, tc.wantCmdShort)
				}
				if len(cmd.Aliases) != tc.wantCmdAliasesLen {
					t.Errorf("len(cmd.Aliases) = %d, want %d", len(cmd.Aliases), tc.wantCmdAliasesLen)
				}
			}
		})
	}
}
