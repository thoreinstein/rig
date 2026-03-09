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

	// 2. Simulate a plugin session by creating a host server manually
	p := &Plugin{Name: "legacy-plugin"}
	srv, lis, path, err := mgr.newPluginHostServer(p)
	if err != nil {
		t.Fatalf("failed to create host server: %v", err)
	}
	go func() { _ = srv.Serve(lis) }()
	defer srv.Stop()
	defer lis.Close()

	// 3. Connect a raw gRPC client to the host's UDS endpoint
	// We'll use the generated client but explicitly only read the legacy field.
	conn, err := grpc.NewClient("unix://"+path, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("failed to connect to host: %v", err)
	}
	defer conn.Close()

	client := apiv1.NewSecretServiceClient(conn)

	t.Run("GetSecret populates legacy value field", func(t *testing.T) {
		resp, err := client.GetSecret(t.Context(), &apiv1.GetSecretRequest{
			Key: "API_KEY",
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
