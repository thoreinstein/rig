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

	// 1. Compile the mock command plugin once for all cases
	pluginBin := filepath.Join(tmpDir, "mock-cmd-plugin")
	cwd, _ := os.Getwd()
	sourceFile := filepath.Join(cwd, "testdata", "mock-cmd-plugin", "main.go")

	compileCmd := exec.Command("go", "build", "-o", pluginBin, sourceFile)
	if out, err := compileCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to compile mock-cmd-plugin: %v\nOutput: %s", err, string(out))
	}

	// 2. Setup plugin directory structure
	pluginDir := filepath.Join(tmpDir, "plugins")
	if err := os.Mkdir(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}

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

	// 3. Define test cases
	tests := []struct {
		name         string
		command      string
		args         []string
		wantStdout   string
		wantStderr   string
		wantExitCode int32
		wantErr      bool
	}{
		{
			name:         "Echo command success",
			command:      "echo",
			args:         []string{"hello", "world"},
			wantStdout:   "hello world",
			wantExitCode: 0,
		},
		{
			name:         "Unknown command failure",
			command:      "unknown",
			wantStderr:   "Unknown command: unknown",
			wantExitCode: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			scanner := &Scanner{Paths: []string{pluginDir}}
			executor := NewExecutor("")

			configProvider := func(name string) ([]byte, error) {
				return []byte(`{"test":"config"}`), nil
			}

			manager, err := NewManager(executor, scanner, "0.1.0", configProvider, nil)
			if err != nil {
				t.Fatalf("NewManager() failed: %v", err)
			}
			defer manager.StopAll()

			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()

			client, err := manager.GetCommandClient(ctx, "mock-cmd")
			if err != nil {
				t.Fatalf("GetCommandClient() failed: %v", err)
			}

			stream, err := client.Execute(ctx, &apiv1.ExecuteRequest{
				Command: tc.command,
				Args:    tc.args,
			})
			if err != nil {
				if tc.wantErr {
					return
				}
				t.Fatalf("Execute() failed: %v", err)
			}

			var stdout, stderr strings.Builder
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
				if len(resp.Stderr) > 0 {
					stderr.Write(resp.Stderr)
				}
				if resp.Done {
					gotDone = true
					exitCode = resp.ExitCode
					break
				}
			}

			if !gotDone {
				t.Error("expected done=true in stream")
			}
			if exitCode != tc.wantExitCode {
				t.Errorf("exit_code = %d, want %d", exitCode, tc.wantExitCode)
			}
			if tc.wantStdout != "" && stdout.String() != tc.wantStdout {
				t.Errorf("stdout = %q, want %q", stdout.String(), tc.wantStdout)
			}
			if tc.wantStderr != "" && stderr.String() != tc.wantStderr {
				t.Errorf("stderr = %q, want %q", stderr.String(), tc.wantStderr)
			}
		})
	}
}
