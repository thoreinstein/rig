package orchestration

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"thoreinstein.com/rig/pkg/plugin"
)

func TestBridgeIntegration(t *testing.T) {
	// Compile the mock plugin binary once to avoid 'go run' overhead in CI
	tmpDir := t.TempDir()
	pluginBin := filepath.Join(tmpDir, "test-node-plugin")
	cmd := exec.Command("go", "build", "-o", pluginBin, "testdata/mock_node_plugin.go")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to compile mock plugin: %v\nOutput: %s", err, string(out))
	}

	// Setup Rig Plugin Manager
	scanner := &plugin.Scanner{Paths: []string{tmpDir}}
	mgr, err := plugin.NewManager(plugin.NewExecutor(), scanner, "dev", nil, nil)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}
	defer mgr.StopAll()

	// Setup bridge and test environment
	bridge := NewPluginNodeBridge(mgr, nil)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	workspace := filepath.Join(tmpDir, "work")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("failed to create workspace: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "allowed.txt"), []byte("data"), 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	baseNode := &Node{
		ID:   "n1",
		Type: "test-node-plugin", // matches plugin binary name
	}

	tests := []struct {
		name          string
		action        string
		caps          *NodeCapabilities
		secrets       map[string]string
		expectErr     bool
		expectOutput  string
		expectFailure string // string to match in error message
	}{
		{
			name:   "read allowed file",
			action: "read_allowed",
			caps:   &NodeCapabilities{Workspace: workspace},
		},
		{
			name:         "read denied file",
			action:       "read_denied",
			caps:         &NodeCapabilities{Workspace: workspace},
			expectOutput: `{"status":"denied_as_expected"}`,
		},
		{
			name:         "network denied",
			action:       "network_denied",
			caps:         &NodeCapabilities{NetworkAccess: false},
			expectOutput: `{"status":"denied_as_expected"}`,
		},
		{
			name:    "secrets mapped",
			action:  "check_secrets",
			caps:    &NodeCapabilities{}, // mapping handled by orchestrator, we just check passed in
			secrets: map[string]string{"MY_API_KEY": "super-secret-value"},
		},
		{
			name:          "plugin panic handling",
			action:        "panic",
			caps:          &NodeCapabilities{},
			expectErr:     true,
			expectFailure: "node execution failed", // stream error
		},
	}

	// Subtests run sequentially (no t.Parallel) because they share a single plugin process.
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pluginCfg := json.RawMessage(`{"action": "` + tt.action + `"}`)
			output, err := bridge.ExecuteNode(ctx, baseNode, tt.caps, pluginCfg, nil, tt.secrets)

			if tt.expectErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tt.expectFailure != "" && !strings.Contains(err.Error(), tt.expectFailure) {
					t.Errorf("expected error containing %q, got: %v", tt.expectFailure, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.expectOutput != "" {
				if string(output) != tt.expectOutput {
					t.Errorf("expected output %q, got %q", tt.expectOutput, string(output))
				}
			} else {
				if string(output) != `{"status":"success"}` {
					t.Errorf("expected success output, got %q", string(output))
				}
			}
		})
	}
}
