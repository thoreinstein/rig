package orchestration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestParseNodeConfig(t *testing.T) {
	tests := []struct {
		name         string
		rawJSON      string
		expectCaps   *NodeCapabilities
		expectIO     *NodeIOSchema
		expectPlugin string
		expectErr    bool
	}{
		{
			name:    "empty config",
			rawJSON: ``,
			expectCaps: &NodeCapabilities{
				Workspace:      "",
				AllowedPaths:   nil,
				NetworkAccess:  false,
				SecretsMapping: nil,
			},
			expectIO:     &NodeIOSchema{},
			expectPlugin: `{}`,
			expectErr:    false,
		},
		{
			name: "full capabilities and io",
			rawJSON: `{
				"capabilities": {
					"workspace": "/tmp/work",
					"allowed_paths": ["/shared/data"],
					"network_access": true,
					"secrets_mapping": {"API_KEY": "prod/api-key"}
				},
				"io": {
					"inputs": {"input1": "string"},
					"outputs": {"output1": "number"}
				},
				"plugin": {"temperature": 0.7}
			}`,
			expectCaps: &NodeCapabilities{
				Workspace:      "/tmp/work",
				AllowedPaths:   []string{"/shared/data"},
				NetworkAccess:  true,
				SecretsMapping: map[string]string{"API_KEY": "prod/api-key"},
			},
			expectIO: &NodeIOSchema{
				Inputs:  map[string]IOType{"input1": IOTypeString},
				Outputs: map[string]IOType{"output1": IOTypeNumber},
			},
			expectPlugin: `{"temperature": 0.7}`,
			expectErr:    false,
		},
		{
			name: "io-only wrapper",
			rawJSON: `{
				"io": {
					"inputs": {"input1": "boolean"}
				}
			}`,
			expectCaps: &NodeCapabilities{},
			expectIO: &NodeIOSchema{
				Inputs: map[string]IOType{"input1": IOTypeBoolean},
			},
			expectPlugin: `{}`,
			expectErr:    false,
		},
		{
			name: "legacy top-level plugin config",
			rawJSON: `{
				"model": "gpt-4",
				"temperature": 0.2
			}`,
			expectCaps:   &NodeCapabilities{},
			expectIO:     &NodeIOSchema{},
			expectPlugin: `{"model":"gpt-4","temperature":0.2}`,
			expectErr:    false,
		},
		{
			name:         "invalid json",
			rawJSON:      `{invalid`,
			expectCaps:   nil,
			expectIO:     nil,
			expectPlugin: ``,
			expectErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caps, io, pluginCfg, err := ParseNodeConfig(json.RawMessage(tt.rawJSON))

			if tt.expectErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if caps.Workspace != tt.expectCaps.Workspace {
				t.Errorf("expected workspace %q, got %q", tt.expectCaps.Workspace, caps.Workspace)
			}
			if caps.NetworkAccess != tt.expectCaps.NetworkAccess {
				t.Errorf("expected network_access %v, got %v", tt.expectCaps.NetworkAccess, caps.NetworkAccess)
			}

			if tt.expectIO != nil {
				if len(io.Inputs) != len(tt.expectIO.Inputs) {
					t.Errorf("expected %d inputs, got %d", len(tt.expectIO.Inputs), len(io.Inputs))
				}
				for k, v := range tt.expectIO.Inputs {
					if io.Inputs[k] != v {
						t.Errorf("expected input %q to be %q, got %q", k, v, io.Inputs[k])
					}
				}
			}

			if string(pluginCfg) != tt.expectPlugin && len(pluginCfg) > 0 && len(tt.expectPlugin) > 0 {
				t.Errorf("expected plugin config %q, got %q", tt.expectPlugin, string(pluginCfg))
			}
		})
	}
}

func TestParseNodeCapabilities(t *testing.T) {
	tests := []struct {
		name         string
		rawJSON      string
		expectCaps   *NodeCapabilities
		expectPlugin string
		expectErr    bool
	}{
		{
			name:    "empty config",
			rawJSON: ``,
			expectCaps: &NodeCapabilities{
				Workspace:      "",
				AllowedPaths:   nil,
				NetworkAccess:  false,
				SecretsMapping: nil,
			},
			expectPlugin: `{}`,
			expectErr:    false,
		},
		{
			name:    "no capabilities specified (deny all)",
			rawJSON: `{"plugin": {"model": "claude-3"}}`,
			expectCaps: &NodeCapabilities{
				Workspace:      "",
				AllowedPaths:   nil,
				NetworkAccess:  false,
				SecretsMapping: nil,
			},
			expectPlugin: `{"model": "claude-3"}`,
			expectErr:    false,
		},
		{
			name: "full capabilities",
			rawJSON: `{
				"capabilities": {
					"workspace": "/tmp/work",
					"allowed_paths": ["/shared/data"],
					"network_access": true,
					"secrets_mapping": {"API_KEY": "prod/api-key"}
				},
				"plugin": {"temperature": 0.7}
			}`,
			expectCaps: &NodeCapabilities{
				Workspace:      "/tmp/work",
				AllowedPaths:   []string{"/shared/data"},
				NetworkAccess:  true,
				SecretsMapping: map[string]string{"API_KEY": "prod/api-key"},
			},
			expectPlugin: `{"temperature": 0.7}`,
			expectErr:    false,
		},
		{
			name: "legacy top-level plugin config",
			rawJSON: `{
				"model": "gpt-4",
				"temperature": 0.2
			}`,
			expectCaps: &NodeCapabilities{
				Workspace:      "",
				AllowedPaths:   nil,
				NetworkAccess:  false,
				SecretsMapping: nil,
			},
			expectPlugin: `{"model":"gpt-4","temperature":0.2}`,
			expectErr:    false,
		},
		{
			name: "capabilities-only wrapper",
			rawJSON: `{
				"capabilities": {
					"network_access": true
				}
			}`,
			expectCaps: &NodeCapabilities{
				Workspace:      "",
				AllowedPaths:   nil,
				NetworkAccess:  true,
				SecretsMapping: nil,
			},
			expectPlugin: `{}`,
			expectErr:    false,
		},
		{
			name: "legacy top-level config containing 'capabilities' key",
			rawJSON: `{
				"capabilities": {"internal_feature": true},
				"model": "gpt-4"
			}`,
			expectCaps: &NodeCapabilities{
				Workspace:      "",
				AllowedPaths:   nil,
				NetworkAccess:  false,
				SecretsMapping: nil,
			},
			expectPlugin: `{"capabilities":{"internal_feature":true},"model":"gpt-4"}`,
			expectErr:    false,
		},
		{
			name:         "invalid json",
			rawJSON:      `{invalid`,
			expectCaps:   nil,
			expectPlugin: ``,
			expectErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caps, pluginCfg, err := ParseNodeCapabilities(json.RawMessage(tt.rawJSON))

			if tt.expectErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if caps.Workspace != tt.expectCaps.Workspace {
				t.Errorf("expected workspace %q, got %q", tt.expectCaps.Workspace, caps.Workspace)
			}
			if caps.NetworkAccess != tt.expectCaps.NetworkAccess {
				t.Errorf("expected network_access %v, got %v", tt.expectCaps.NetworkAccess, caps.NetworkAccess)
			}

			if string(pluginCfg) != tt.expectPlugin && len(pluginCfg) > 0 && len(tt.expectPlugin) > 0 {
				t.Errorf("expected plugin config %q, got %q", tt.expectPlugin, string(pluginCfg))
			}
		})
	}
}

func TestIsPathAllowed(t *testing.T) {
	// Normalize paths for cross-platform testing
	var root string
	if runtime.GOOS == "windows" {
		root = "C:\\"
	} else {
		root = "/"
	}

	wsPath := filepath.Join(root, "tmp", "work")
	sharedPath := filepath.Join(root, "shared", "data")

	caps := &NodeCapabilities{
		Workspace:    wsPath,
		AllowedPaths: []string{sharedPath},
	}

	tests := []struct {
		name    string
		path    string
		allowed bool
	}{
		{
			name:    "exact workspace",
			path:    wsPath,
			allowed: true,
		},
		{
			name:    "inside workspace",
			path:    filepath.Join(wsPath, "file.txt"),
			allowed: true,
		},
		{
			name:    "nested inside workspace",
			path:    filepath.Join(wsPath, "dir", "file.txt"),
			allowed: true,
		},
		{
			name:    "exact allowed path",
			path:    sharedPath,
			allowed: true,
		},
		{
			name:    "inside allowed path",
			path:    filepath.Join(sharedPath, "dataset.csv"),
			allowed: true,
		},
		{
			name:    "outside workspace and allowed paths",
			path:    filepath.Join(root, "etc", "passwd"),
			allowed: false,
		},
		{
			name:    "relative path (denied)",
			path:    "tmp/work/file.txt",
			allowed: false,
		},
		{
			name:    "directory traversal attempt (escape workspace)",
			path:    filepath.Join(wsPath, "..", "..", "etc", "passwd"),
			allowed: false,
		},
		{
			name:    "directory traversal attempt (fake sub-path)",
			path:    wsPath + "-fake",
			allowed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed := caps.IsPathAllowed(tt.path)
			if allowed != tt.allowed {
				t.Errorf("IsPathAllowed(%q) = %v, expected %v", tt.path, allowed, tt.allowed)
			}
		})
	}
}

func TestIsPathAllowed_SymlinkEscape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink test requires unix")
	}

	// Setup: workspace with a symlink pointing outside
	tmpDir := t.TempDir()
	workspace := filepath.Join(tmpDir, "workspace")
	outside := filepath.Join(tmpDir, "outside")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a symlink inside workspace that points outside
	symlink := filepath.Join(workspace, "escape")
	if err := os.Symlink(outside, symlink); err != nil {
		t.Fatal(err)
	}

	caps := &NodeCapabilities{Workspace: workspace}

	// The symlink path looks like it's inside workspace, but resolves outside
	if caps.IsPathAllowed(filepath.Join(symlink, "secret.txt")) {
		t.Error("symlink escape should be denied, but was allowed")
	}

	// A real file inside workspace should still be allowed
	realFile := filepath.Join(workspace, "real.txt")
	if err := os.WriteFile(realFile, []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !caps.IsPathAllowed(realFile) {
		t.Error("real file inside workspace should be allowed")
	}
}

func TestIsPathAllowed_SymlinkEscape_NewFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink test requires unix")
	}

	// Setup: workspace with a symlink pointing outside
	tmpDir := t.TempDir()
	workspace := filepath.Join(tmpDir, "workspace")
	outside := filepath.Join(tmpDir, "outside")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a symlink inside workspace that points outside
	symlink := filepath.Join(workspace, "escape")
	if err := os.Symlink(outside, symlink); err != nil {
		t.Fatal(err)
	}

	caps := &NodeCapabilities{Workspace: workspace}

	// The target file does NOT exist.
	// IsPathAllowed must resolve the parent directory 'escape' to 'outside'
	// and recognize that it falls outside the workspace.
	newFile := filepath.Join(symlink, "new.txt")
	if caps.IsPathAllowed(newFile) {
		t.Error("symlink escape for new file should be denied, but was allowed")
	}
}
