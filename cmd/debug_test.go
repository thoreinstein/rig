package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"thoreinstein.com/rig/pkg/config"
)

func TestOutputConfigJSON_Redaction(t *testing.T) {
	sources := config.SourceMap{
		"github.token": config.SourceEntry{
			Value:  "secret-token",
			Source: config.SourceDefault,
		},
		"notes.path": config.SourceEntry{
			Value:  "/tmp/notes",
			Source: config.SourceDefault,
		},
	}

	discovery := []config.DiscoveryEvent{
		{Tier: "default", Message: "test message"},
	}

	violations := []config.TrustViolation{
		{
			Key:            "github.token",
			File:           ".rig.toml",
			Reason:         config.ViolationImmutable,
			AttemptedValue: "leaked-secret",
		},
		{
			Key:            "notes.path",
			File:           ".rig.toml",
			Reason:         config.ViolationUntrustedProject,
			AttemptedValue: "/other/path",
		},
	}

	var buf bytes.Buffer
	err := outputConfigJSON(&buf, "/user/config.toml", sources, discovery, violations)
	if err != nil {
		t.Fatalf("outputConfigJSON() error: %v", err)
	}

	var out debugConfigOutput
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("Failed to unmarshal JSON output: %v", err)
	}

	// Verify effective config redaction
	if val, ok := out.Config["github.token"]; !ok || val.Value != "********" {
		t.Errorf("github.token effective value = %v, want ********", val.Value)
	}
	if val, ok := out.Config["notes.path"]; !ok || val.Value != "/tmp/notes" {
		t.Errorf("notes.path effective value = %v, want /tmp/notes", val.Value)
	}

	// Verify violation redaction
	foundSecret := false
	foundNonSecret := false
	for _, v := range out.Violations {
		if v.Key == "github.token" {
			foundSecret = true
			if v.AttemptedValue != "********" {
				t.Errorf("github.token violation attempted value = %v, want ********", v.AttemptedValue)
			}
		}
		if v.Key == "notes.path" {
			foundNonSecret = true
			if v.AttemptedValue != "/other/path" {
				t.Errorf("notes.path violation attempted value = %v, want /other/path", v.AttemptedValue)
			}
		}
	}

	if !foundSecret {
		t.Error("github.token violation not found in output")
	}
	if !foundNonSecret {
		t.Error("notes.path violation not found in output")
	}

	// Final paranoid check: ensure the raw secret strings are nowhere in the JSON buffer
	rawJSON := buf.String()
	if strings.Contains(rawJSON, "secret-token") {
		t.Error("JSON output contains raw github.token secret value!")
	}
	if strings.Contains(rawJSON, "leaked-secret") {
		t.Error("JSON output contains raw github.token attempted value secret!")
	}
}
