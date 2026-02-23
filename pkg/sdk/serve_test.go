package sdk

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
)

func TestServe_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test-plugin.sock")

	info := Info{
		ID: "test-plugin",
	}
	p := &mockPlugin{info: info}

	ctx, cancel := context.WithCancel(t.Context())

	errCh := make(chan error, 1)
	go func() {
		errCh <- Serve(p, WithEndpoint(socketPath), WithContext(ctx))
	}()

	// Wait for socket to be created
	deadline := time.Now().Add(5 * time.Second)
	var found bool
	for time.Now().Before(deadline) {
		if _, err := os.Stat(socketPath); err == nil {
			found = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !found {
		t.Fatalf("socket file not created in time")
	}

	conn, err := grpc.NewClient("unix://"+socketPath, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("failed to create gRPC client: %v", err)
	}
	defer conn.Close()

	client := apiv1.NewPluginServiceClient(conn)
	resp, err := client.Handshake(t.Context(), &apiv1.HandshakeRequest{})
	if err != nil {
		t.Fatalf("Handshake failed: %v", err)
	}

	if resp.PluginId != info.ID {
		t.Errorf("got PluginId %q, want %q", resp.PluginId, info.ID)
	}

	// Test graceful shutdown
	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("Serve() returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("Serve() did not shut down in time")
	}

	// Verify socket removed
	if _, err := os.Stat(socketPath); !os.IsNotExist(err) {
		t.Error("socket file still exists after shutdown")
	}
}

func TestServe_StaleSocketRemoval(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "stale.sock")

	// Create a stale file
	if err := os.WriteFile(socketPath, []byte("stale"), 0644); err != nil {
		t.Fatalf("failed to create stale file: %v", err)
	}

	info := Info{ID: "test"}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	// Serve should remove it
	go func() {
		_ = Serve(&mockPlugin{info: info}, WithEndpoint(socketPath), WithContext(ctx))
	}()

	// Just wait a bit and check if we can dial it (which means it was replaced by a listener)
	deadline := time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := net.Dial("unix", socketPath); err == nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Error("failed to start server on stale socket path")
}
