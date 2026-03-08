package plugin

import (
	"log/slog"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
)

// TestIntegration_WireCompatibility proves that a host running the new Context/Secret
// services remains compatible with a client that only expects the legacy V1 response shape.
func TestIntegration_WireCompatibility(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Setup a host with real secret resolver
	configProvider := func(pluginName string) ([]byte, error) {
		return []byte(`{"secrets": {"API_KEY": "secret-value"}}`), nil
	}

	// We use a mock scanner since we're not actually starting plugin processes here,
	// just testing the host's server compatibility.
	scanner := &Scanner{Paths: []string{tmpDir}}

	mgr, err := NewManager(&mockExecutor{}, scanner, "1.0.0", configProvider, slog.Default())
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}
	defer mgr.StopAll()

	// 2. Simulate a plugin "session" by registering a token manually
	token := "legacy-plugin-token"
	mgr.tokenStore.Register(token, "legacy-plugin")

	// 3. Connect a raw gRPC client to the host's UDS endpoint
	// We'll use the generated client but explicitly only read the legacy field.
	conn, err := grpc.NewClient("unix://"+mgr.hostPath, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("failed to connect to host: %v", err)
	}
	defer conn.Close()

	client := apiv1.NewSecretServiceClient(conn)

	t.Run("GetSecret populates legacy value field", func(t *testing.T) {
		resp, err := client.GetSecret(t.Context(), &apiv1.GetSecretRequest{
			Key:   "API_KEY",
			Token: token,
		})
		if err != nil {
			t.Fatalf("GetSecret failed: %v", err)
		}

		// ASSERT: The legacy field 'Value' MUST be populated for backward compatibility.
		if resp.Value != "secret-value" { //nolint:staticcheck // testing wire compatibility
			t.Errorf("legacy Value field not populated: got %q, want %q", resp.Value, "secret-value") //nolint:staticcheck
		}

		// ASSERT: The new field 'Secret' is also populated for modern clients.
		if resp.Secret == nil || resp.Secret.Value != "secret-value" {
			t.Errorf("new Secret field not correctly populated: %+v", resp.Secret)
		}
	})
}

// TestIntegration_MultipleRotatedTokensCleanup proves that all tokens (original and rotated)
// are correctly purged from the host when a plugin session ends.
func TestIntegration_TokenCleanup(t *testing.T) {
	tmpDir := t.TempDir()
	scanner := &Scanner{Paths: []string{tmpDir}}
	mgr, err := NewManager(&mockExecutor{}, scanner, "1.0.0", nil, slog.Default())
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}
	defer mgr.StopAll()

	name := "rotating-plugin"
	t1 := "initial-token"
	mgr.tokenStore.Register(t1, name)

	// Simulate rotation
	_, t2, err := mgr.tokenStore.Rotate(t1)
	if err != nil {
		t.Fatalf("failed to rotate token: %v", err)
	}

	// Verify both existed (t1 should be gone, t2 should exist)
	if _, ok := mgr.tokenStore.Resolve(t1); ok {
		t.Error("initial token should have been removed by rotation")
	}
	if _, ok := mgr.tokenStore.Resolve(t2); !ok {
		t.Error("rotated token should exist")
	}

	// Simulate one more rotation
	_, t3, err := mgr.tokenStore.Rotate(t2)
	if err != nil {
		t.Fatalf("failed to rotate token again: %v", err)
	}

	// Now stop the plugin (simulate via name cleanup as StopAll/StopPluginIfIdle does)
	mgr.tokenStore.UnregisterPlugin(name)

	// ASSERT: All tokens associated with the plugin MUST be gone.
	for _, tok := range []string{t1, t2, t3} {
		if _, ok := mgr.tokenStore.Resolve(tok); ok {
			t.Errorf("token %s still exists after plugin cleanup", tok)
		}
	}
}
