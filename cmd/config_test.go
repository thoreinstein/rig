package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"thoreinstein.com/rig/pkg/bootstrap"
)

func TestConfigInspect(t *testing.T) {
	// Create a temp config
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")
	content := `
[github]
token = "test-token"
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	// Setup environment
	t.Setenv("HOME", tmpDir)

	// Initialize config
	var err error
	appLoader, appConfig, verbose, err = bootstrap.InitConfig(configPath, false)
	if err != nil {
		t.Fatalf("InitConfig failed: %v", err)
	}

	// Capture output
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"config", "inspect"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	output := buf.String()

	// Verify header
	if !strings.Contains(output, "KEY") || !strings.Contains(output, "VALUE") || !strings.Contains(output, "SOURCE") {
		t.Errorf("missing headers in output: %s", output)
	}

	// Verify masked value
	if !strings.Contains(output, "github.token") || !strings.Contains(output, "********") {
		t.Errorf("github.token should be masked: %s", output)
	}

	// Verify source
	if !strings.Contains(output, "User:") && !strings.Contains(output, configPath) {
		t.Errorf("missing source attribution: %s", output)
	}
}
