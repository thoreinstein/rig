package beads

import (
	"testing"
)

func TestNewCLIClient(t *testing.T) {
	client, err := NewCLIClient("bd", true)
	if err != nil {
		t.Fatalf("NewCLIClient() error = %v, want nil", err)
	}

	if client.CliCommand != "bd" {
		t.Errorf("CliCommand = %q, want %q", client.CliCommand, "bd")
	}
	if !client.Verbose {
		t.Error("Verbose should be true")
	}
}

func TestNewCLIClient_DefaultCommand(t *testing.T) {
	client, err := NewCLIClient("", false)
	if err != nil {
		t.Fatalf("NewCLIClient() error = %v, want nil", err)
	}

	if client.CliCommand != "bd" {
		t.Errorf("CliCommand = %q, want %q", client.CliCommand, "bd")
	}
}

func TestNewCLIClient_InvalidCommand(t *testing.T) {
	tests := []struct {
		name    string
		command string
	}{
		{"semicolon", "bd; rm -rf /"},
		{"pipe", "bd | cat"},
		{"backtick", "bd`whoami`"},
		{"dollar", "bd$(whoami)"},
		{"ampersand", "bd && rm -rf /"},
		{"space", "bd foo"},
		{"quotes", "bd\"test\""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewCLIClient(tt.command, false)
			if err == nil {
				t.Errorf("NewCLIClient(%q) should return error for invalid command", tt.command)
			}
		})
	}
}

func TestNewCLIClient_ValidCommands(t *testing.T) {
	tests := []struct {
		name    string
		command string
	}{
		{"simple", "bd"},
		{"with hyphen", "my-cli"},
		{"with underscore", "my_cli"},
		{"with path", "/usr/local/bin/bd"},
		{"relative path", "bin/bd"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewCLIClient(tt.command, false)
			if err != nil {
				t.Errorf("NewCLIClient(%q) error = %v, want nil", tt.command, err)
			}
			if client == nil {
				t.Errorf("NewCLIClient(%q) returned nil client", tt.command)
			}
		})
	}
}

func TestIsAvailable_NonExistent(t *testing.T) {
	client, err := NewCLIClient("nonexistent-command-xyz-123", false)
	if err != nil {
		t.Fatalf("NewCLIClient() error = %v, want nil", err)
	}

	if client.IsAvailable() {
		t.Error("IsAvailable() should return false for non-existent command")
	}
}

func TestIsAvailable_Existing(t *testing.T) {
	// Test with a command that definitely exists on all systems
	client, err := NewCLIClient("ls", false)
	if err != nil {
		t.Fatalf("NewCLIClient() error = %v, want nil", err)
	}

	if !client.IsAvailable() {
		t.Error("IsAvailable() should return true for 'ls' command")
	}
}

func TestShow_InvalidID(t *testing.T) {
	client, err := NewCLIClient("bd", false)
	if err != nil {
		t.Fatalf("NewCLIClient() error = %v, want nil", err)
	}

	tests := []struct {
		name string
		id   string
	}{
		{"semicolon", "beads-123; rm -rf /"},
		{"space", "beads 123"},
		{"quotes", "beads\"123\""},
		{"pipe", "beads|cat"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.Show(tt.id)
			if err == nil {
				t.Errorf("Show(%q) should return error for invalid ID", tt.id)
			}
		})
	}
}

func TestShow_ValidIDFormats(t *testing.T) {
	// These tests just validate the ID format checking,
	// not the actual CLI execution (which would fail without bd installed)
	tests := []struct {
		name string
		id   string
	}{
		{"standard", "beads-123"},
		{"with underscore", "beads_123"},
		{"alphanumeric", "BEADS123"},
		{"hyphenated", "my-project-beads-123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !validIssueIDPattern.MatchString(tt.id) {
				t.Errorf("validIssueIDPattern should match %q", tt.id)
			}
		})
	}
}

func TestUpdateStatus_InvalidStatus(t *testing.T) {
	client, err := NewCLIClient("nonexistent-bd-command", false)
	if err != nil {
		t.Fatalf("NewCLIClient() error = %v, want nil", err)
	}

	// Even with unavailable CLI, invalid status should be caught first
	err = client.UpdateStatus("beads-123", "invalid-status")
	if err == nil {
		t.Error("UpdateStatus() should return error for invalid status")
	}
}

func TestUpdateStatus_InvalidID(t *testing.T) {
	client, err := NewCLIClient("bd", false)
	if err != nil {
		t.Fatalf("NewCLIClient() error = %v, want nil", err)
	}

	err = client.UpdateStatus("invalid; id", "open")
	if err == nil {
		t.Error("UpdateStatus() should return error for invalid ID")
	}
}
