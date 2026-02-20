//go:build integration

package daemon

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"google.golang.org/grpc"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
	"thoreinstein.com/rig/pkg/plugin"
)

func TestDaemon_Integration(t *testing.T) {
	// 1. Setup temporary directory for daemon and plugins
	tmpDir := t.TempDir()

	// On Darwin, t.TempDir() can be very long, exceeding AF_UNIX limit.
	// We'll use a shorter base for the socket.
	daemonBase := filepath.Join("/tmp", fmt.Sprintf("rig-test-%d", os.Getpid()))
	if err := os.MkdirAll(daemonBase, 0700); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(daemonBase)

	t.Setenv("XDG_RUNTIME_DIR", daemonBase)

	if err := EnsureDir(); err != nil {
		t.Fatal(err)
	}

	pluginDir := filepath.Join(tmpDir, ".rig", "plugins", "mock-cmd-plugin")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}

	// 2. Compile mock plugin
	pluginSource := filepath.Join("..", "plugin", "testdata", "mock-cmd-plugin")
	pluginPath := filepath.Join(pluginDir, "mock-plugin")

	// Check if source exists
	if _, err := os.Stat(filepath.Join(pluginSource, "main.go")); err != nil {
		t.Skip("mock-cmd-plugin source not found, skipping integration test")
	}

	cmd := exec.Command("go", "build", "-o", pluginPath, filepath.Join(pluginSource, "main.go"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build mock plugin: %v\nOutput: %s", err, string(out))
	}
	// Copy manifest
	manifestData, err := os.ReadFile(filepath.Join(pluginSource, "manifest.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "manifest.yaml"), manifestData, 0644); err != nil {
		t.Fatal(err)
	}

	// 3. Start Daemon Server
	scanner, err := plugin.NewScannerWithProjectRoot(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	executor := plugin.NewExecutor("")
	uiProxy := NewDaemonUIProxy()
	mgr, err := plugin.NewManager(executor, scanner, "1.0.0", nil, slog.Default(), plugin.WithUIServer(uiProxy))
	if err != nil {
		t.Fatal(err)
	}
	defer mgr.StopAll()

	server := NewDaemonServer(mgr, uiProxy, "1.0.0", slog.Default())

	path := SocketPath()
	lis, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer lis.Close()

	s := grpc.NewServer()
	apiv1.RegisterDaemonServiceServer(s, server)
	go func() {
		_ = s.Serve(lis)
	}()
	defer s.Stop()

	if err := WritePIDFile(); err != nil {
		t.Fatal(err)
	}
	defer RemovePIDFile()

	// 4. Test Client Execution
	client, err := NewClient(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	var stdout, stderr bytes.Buffer
	mockUI := &mockUIHandler{}

	err = client.ExecuteCommand(t.Context(), &apiv1.CommandRequest{
		PluginName:  "mock-cmd",
		CommandName: "echo",
		Args:        []string{"hello", "daemon"},
	}, mockUI, &stdout, &stderr)

	if err != nil {
		t.Fatalf("ExecuteCommand failed: %v", err)
	}

	if !bytes.Contains(stdout.Bytes(), []byte("hello daemon")) {
		t.Errorf("unexpected stdout: %s", stdout.String())
	}
}

type mockUIHandler struct{}

func (m *mockUIHandler) HandlePrompt(ctx context.Context, req *apiv1.PromptRequest) (*apiv1.PromptResponse, error) {
	return &apiv1.PromptResponse{Value: "mock-response"}, nil
}
func (m *mockUIHandler) HandleConfirm(ctx context.Context, req *apiv1.ConfirmRequest) (*apiv1.ConfirmResponse, error) {
	return &apiv1.ConfirmResponse{Confirmed: true}, nil
}
func (m *mockUIHandler) HandleSelect(ctx context.Context, req *apiv1.SelectRequest) (*apiv1.SelectResponse, error) {
	return &apiv1.SelectResponse{SelectedIndices: []uint32{0}}, nil
}
func (m *mockUIHandler) HandleProgress(ctx context.Context, req *apiv1.ProgressUpdate) error {
	return nil
}
