package plugin

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
)

func TestCommandExecution(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Compile the mock command plugin
	pluginBin := filepath.Join(tmpDir, "mock-cmd-plugin")
	// Use absolute path for the source file to avoid issues with CWD
	cwd, _ := os.Getwd()
	sourceFile := filepath.Join(cwd, "testdata", "mock-cmd-plugin", "main.go")

	cmd := exec.Command("go", "build", "-o", pluginBin, sourceFile)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to compile mock-cmd-plugin: %v\nOutput: %s", err, string(out))
	}

	// 2. Setup plugin directory structure for scanning
	pluginDir := filepath.Join(tmpDir, "plugins")
	if err := os.Mkdir(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Move binary and copy manifest
	targetBin := filepath.Join(pluginDir, "mock-cmd")
	if err := os.Rename(pluginBin, targetBin); err != nil {
		t.Fatal(err)
	}

	manifestSrc := filepath.Join(cwd, "testdata", "mock-cmd-plugin", "manifest.yaml")
	manifestData, err := os.ReadFile(manifestSrc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(targetBin+".manifest.yaml", manifestData, 0644); err != nil {
		t.Fatal(err)
	}

	// 3. Initialize Manager
	scanner := &Scanner{Paths: []string{pluginDir}}
	executor := NewExecutor("")

	// Mock config provider that returns a test config
	configProvider := func(name string) ([]byte, error) {
		return []byte(`{"test":"config"}`), nil
	}

	manager, err := NewManager(executor, scanner, "0.1.0", configProvider)
	if err != nil {
		t.Fatalf("NewManager() failed: %v", err)
	}
	defer manager.StopAll()

	// 4. Get command client (starts plugin + handshake)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	client, err := manager.GetCommandClient(ctx, "mock-cmd")
	if err != nil {
		t.Fatalf("GetCommandClient() failed: %v", err)
	}

	// 5. Execute command
	args := []string{"hello", "world"}
	stream, err := client.Execute(ctx, &apiv1.ExecuteRequest{
		Command: "echo",
		Args:    args,
	})
	if err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}

	// 6. Collect output
	var stdout strings.Builder
	var gotDone bool
	var exitCode int32

	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("stream.Recv() failed: %v", err)
		}

		if len(resp.Stdout) > 0 {
			stdout.Write(resp.Stdout)
		}
		if resp.Done {
			gotDone = true
			exitCode = resp.ExitCode
			break
		}
	}

	// 7. Verify results
	if !gotDone {
		t.Error("expected done=true in stream")
	}
	if exitCode != 0 {
		t.Errorf("expected exit_code=0, got %d", exitCode)
	}
	if stdout.String() != "hello world" {
		t.Errorf("stdout = %q, want %q", stdout.String(), "hello world")
	}
}
