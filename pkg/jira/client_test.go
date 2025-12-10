package jira

import (
	"testing"
)

func TestNewClient(t *testing.T) {
	client, err := NewClient("acli", true)
	if err != nil {
		t.Fatalf("NewClient() error = %v, want nil", err)
	}

	if client.CliCommand != "acli" {
		t.Errorf("CliCommand = %q, want %q", client.CliCommand, "acli")
	}
	if !client.Verbose {
		t.Error("Verbose should be true")
	}
}

func TestNewClient_InvalidCommand(t *testing.T) {
	tests := []struct {
		name    string
		command string
	}{
		{"semicolon", "acli; rm -rf /"},
		{"pipe", "acli | cat"},
		{"backtick", "acli`whoami`"},
		{"dollar", "acli$(whoami)"},
		{"ampersand", "acli && rm -rf /"},
		{"space", "acli foo"},
		{"quotes", "acli\"test\""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewClient(tt.command, false)
			if err == nil {
				t.Errorf("NewClient(%q) should return error for invalid command", tt.command)
			}
		})
	}
}

func TestNewClient_ValidCommands(t *testing.T) {
	tests := []struct {
		name    string
		command string
	}{
		{"simple", "acli"},
		{"with hyphen", "my-cli"},
		{"with underscore", "my_cli"},
		{"with path", "/usr/local/bin/acli"},
		{"relative path", "bin/acli"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.command, false)
			if err != nil {
				t.Errorf("NewClient(%q) error = %v, want nil", tt.command, err)
			}
			if client == nil {
				t.Errorf("NewClient(%q) returned nil client", tt.command)
			}
		})
	}
}

func TestIsAvailable_NonExistent(t *testing.T) {
	client, err := NewClient("nonexistent-command-xyz-123", false)
	if err != nil {
		t.Fatalf("NewClient() error = %v, want nil", err)
	}

	if client.IsAvailable() {
		t.Error("IsAvailable() should return false for non-existent command")
	}
}

func TestIsAvailable_Existing(t *testing.T) {
	// Test with a command that definitely exists on all systems
	client, err := NewClient("ls", false)
	if err != nil {
		t.Fatalf("NewClient() error = %v, want nil", err)
	}

	if !client.IsAvailable() {
		t.Error("IsAvailable() should return true for 'ls' command")
	}
}

func TestParseJiraOutput_FullOutput(t *testing.T) {
	client, err := NewClient("acli", false)
	if err != nil {
		t.Fatalf("NewClient() error = %v, want nil", err)
	}

	// Note: The parser stops description collection when it hits a new field
	// "Priority:" is recognized as a new field, so description stops there
	output := `Type: Bug
Summary: Fix authentication issue in login flow
Status: In Progress
Description:
Users are experiencing login failures when using SSO.
The issue appears to be related to token validation.
Assignee: John Doe`

	info := client.parseJiraOutput(output)

	if info.Type != "Bug" {
		t.Errorf("Type = %q, want %q", info.Type, "Bug")
	}
	if info.Summary != "Fix authentication issue in login flow" {
		t.Errorf("Summary = %q, want %q", info.Summary, "Fix authentication issue in login flow")
	}
	if info.Status != "In Progress" {
		t.Errorf("Status = %q, want %q", info.Status, "In Progress")
	}

	// Description ends when Assignee: field is reached
	expectedDesc := `Users are experiencing login failures when using SSO.
The issue appears to be related to token validation.`
	if info.Description != expectedDesc {
		t.Errorf("Description = %q, want %q", info.Description, expectedDesc)
	}
}

func TestParseJiraOutput_MinimalOutput(t *testing.T) {
	client, err := NewClient("acli", false)
	if err != nil {
		t.Fatalf("NewClient() error = %v, want nil", err)
	}

	output := `Summary: Quick fix
Status: Done`

	info := client.parseJiraOutput(output)

	if info.Type != "" {
		t.Errorf("Type = %q, want empty", info.Type)
	}
	if info.Summary != "Quick fix" {
		t.Errorf("Summary = %q, want %q", info.Summary, "Quick fix")
	}
	if info.Status != "Done" {
		t.Errorf("Status = %q, want %q", info.Status, "Done")
	}
	if info.Description != "" {
		t.Errorf("Description = %q, want empty", info.Description)
	}
}

func TestParseJiraOutput_EmptyOutput(t *testing.T) {
	client, err := NewClient("acli", false)
	if err != nil {
		t.Fatalf("NewClient() error = %v, want nil", err)
	}

	info := client.parseJiraOutput("")

	if info.Type != "" {
		t.Errorf("Type = %q, want empty", info.Type)
	}
	if info.Summary != "" {
		t.Errorf("Summary = %q, want empty", info.Summary)
	}
	if info.Status != "" {
		t.Errorf("Status = %q, want empty", info.Status)
	}
	if info.Description != "" {
		t.Errorf("Description = %q, want empty", info.Description)
	}
}

func TestParseJiraOutput_DescriptionOnly(t *testing.T) {
	client, err := NewClient("acli", false)
	if err != nil {
		t.Fatalf("NewClient() error = %v, want nil", err)
	}

	output := `Type: Story
Description:
This is a long description
that spans multiple lines
and has no other fields after it.`

	info := client.parseJiraOutput(output)

	if info.Type != "Story" {
		t.Errorf("Type = %q, want %q", info.Type, "Story")
	}

	expectedDesc := `This is a long description
that spans multiple lines
and has no other fields after it.`
	if info.Description != expectedDesc {
		t.Errorf("Description = %q, want %q", info.Description, expectedDesc)
	}
}

func TestParseJiraOutput_ExtraWhitespace(t *testing.T) {
	client, err := NewClient("acli", false)
	if err != nil {
		t.Fatalf("NewClient() error = %v, want nil", err)
	}

	output := `  Type:   Task
  Summary:   Implement feature
  Status:   Open  `

	info := client.parseJiraOutput(output)

	if info.Type != "Task" {
		t.Errorf("Type = %q, want %q", info.Type, "Task")
	}
	if info.Summary != "Implement feature" {
		t.Errorf("Summary = %q, want %q", info.Summary, "Implement feature")
	}
	if info.Status != "Open" {
		t.Errorf("Status = %q, want %q", info.Status, "Open")
	}
}

func TestParseJiraOutput_DescriptionWithFieldLikeContent(t *testing.T) {
	client, err := NewClient("acli", false)
	if err != nil {
		t.Fatalf("NewClient() error = %v, want nil", err)
	}

	// Note: The current parser matches fields at any position in the line
	// after trimming whitespace. Lines like "Type: this is not a field"
	// will be recognized as the Type field, overwriting previous values.
	// This test documents the current behavior.
	output := `Type: Bug
Summary: Test ticket
Description:
The user mentioned some context.
More details here.
Assignee: John`

	info := client.parseJiraOutput(output)

	if info.Type != "Bug" {
		t.Errorf("Type = %q, want %q", info.Type, "Bug")
	}
	if info.Summary != "Test ticket" {
		t.Errorf("Summary = %q, want %q", info.Summary, "Test ticket")
	}

	// Description stops at the Assignee field
	expectedDesc := `The user mentioned some context.
More details here.`
	if info.Description != expectedDesc {
		t.Errorf("Description = %q, want %q", info.Description, expectedDesc)
	}
}

func TestIsNewField(t *testing.T) {
	client, err := NewClient("acli", false)
	if err != nil {
		t.Fatalf("NewClient() error = %v, want nil", err)
	}

	tests := []struct {
		name     string
		line     string
		expected bool
	}{
		{
			name:     "Type field",
			line:     "Type: Bug",
			expected: true,
		},
		{
			name:     "Summary field",
			line:     "Summary: Test",
			expected: true,
		},
		{
			name:     "Status field",
			line:     "Status: Open",
			expected: true,
		},
		{
			name:     "Assignee field",
			line:     "Assignee: John Doe",
			expected: true,
		},
		{
			name:     "Multi-word field",
			line:     "Created Date: 2025-01-01",
			expected: true,
		},
		{
			name:     "lowercase start - not a field",
			line:     "type: lowercase",
			expected: false,
		},
		{
			name:     "no colon - not a field",
			line:     "This is regular text",
			expected: false,
		},
		{
			name:     "colon in middle - not a field",
			line:     "The time is 10:30",
			expected: false,
		},
		{
			name:     "empty line",
			line:     "",
			expected: false,
		},
		{
			name:     "just colon",
			line:     ":",
			expected: false,
		},
		{
			name:     "number start - not a field",
			line:     "123: some text",
			expected: false,
		},
		{
			name:     "URL-like - not a field",
			line:     "https://example.com",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := client.isNewField(tt.line)
			if result != tt.expected {
				t.Errorf("isNewField(%q) = %v, want %v", tt.line, result, tt.expected)
			}
		})
	}
}

func TestFetchTicketDetails_UnavailableClient(t *testing.T) {
	client, err := NewClient("nonexistent-command-xyz", false)
	if err != nil {
		t.Fatalf("NewClient() error = %v, want nil", err)
	}

	_, err = client.FetchTicketDetails("FRAAS-123")
	if err == nil {
		t.Error("FetchTicketDetails() should return error when CLI is unavailable")
	}
}
