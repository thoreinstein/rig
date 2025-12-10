package cmd

import (
	"testing"
)

func TestParseTicket(t *testing.T) {
	tests := []struct {
		name        string
		ticket      string
		wantFull    string
		wantType    string
		wantNumber  string
		expectError bool
	}{
		{
			name:       "lowercase fraas ticket",
			ticket:     "fraas-25857",
			wantFull:   "fraas-25857",
			wantType:   "fraas",
			wantNumber: "25857",
		},
		{
			name:       "uppercase FRAAS ticket",
			ticket:     "FRAAS-25857",
			wantFull:   "FRAAS-25857",
			wantType:   "fraas",
			wantNumber: "25857",
		},
		{
			name:       "mixed case ticket",
			ticket:     "Fraas-12345",
			wantFull:   "Fraas-12345",
			wantType:   "fraas",
			wantNumber: "12345",
		},
		{
			name:       "cre ticket",
			ticket:     "cre-123",
			wantFull:   "cre-123",
			wantType:   "cre",
			wantNumber: "123",
		},
		{
			name:       "incident ticket",
			ticket:     "incident-456",
			wantFull:   "incident-456",
			wantType:   "incident",
			wantNumber: "456",
		},
		{
			name:       "ops ticket",
			ticket:     "OPS-789",
			wantFull:   "OPS-789",
			wantType:   "ops",
			wantNumber: "789",
		},
		{
			name:       "single digit number",
			ticket:     "test-1",
			wantFull:   "test-1",
			wantType:   "test",
			wantNumber: "1",
		},
		{
			name:       "large number",
			ticket:     "jira-999999999",
			wantFull:   "jira-999999999",
			wantType:   "jira",
			wantNumber: "999999999",
		},
		{
			name:        "missing number",
			ticket:      "fraas-",
			expectError: true,
		},
		{
			name:        "missing type",
			ticket:      "-123",
			expectError: true,
		},
		{
			name:        "no dash",
			ticket:      "fraas123",
			expectError: true,
		},
		{
			name:        "only number",
			ticket:      "12345",
			expectError: true,
		},
		{
			name:        "only type",
			ticket:      "fraas",
			expectError: true,
		},
		{
			name:        "empty string",
			ticket:      "",
			expectError: true,
		},
		{
			name:        "letters in number",
			ticket:      "fraas-abc",
			expectError: true,
		},
		{
			name:        "mixed letters and numbers",
			ticket:      "fraas-123abc",
			expectError: true,
		},
		{
			name:        "multiple dashes",
			ticket:      "fraas-123-456",
			expectError: true,
		},
		{
			name:        "spaces in ticket",
			ticket:      "fraas -123",
			expectError: true,
		},
		{
			name:        "special characters in type",
			ticket:      "fra@s-123",
			expectError: true,
		},
		{
			name:        "underscore in type",
			ticket:      "fra_s-123",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseTicket(tt.ticket)

			if tt.expectError {
				if err == nil {
					t.Errorf("parseTicket(%q) expected error, got nil", tt.ticket)
				}
				return
			}

			if err != nil {
				t.Fatalf("parseTicket(%q) unexpected error: %v", tt.ticket, err)
			}

			if result.Full != tt.wantFull {
				t.Errorf("parseTicket(%q).Full = %q, want %q", tt.ticket, result.Full, tt.wantFull)
			}
			if result.Type != tt.wantType {
				t.Errorf("parseTicket(%q).Type = %q, want %q", tt.ticket, result.Type, tt.wantType)
			}
			if result.Number != tt.wantNumber {
				t.Errorf("parseTicket(%q).Number = %q, want %q", tt.ticket, result.Number, tt.wantNumber)
			}
		})
	}
}

func TestTicketInfo(t *testing.T) {
	// Test the TicketInfo struct
	info := &TicketInfo{
		Full:   "FRAAS-123",
		Type:   "fraas",
		Number: "123",
	}

	if info.Full != "FRAAS-123" {
		t.Errorf("Full = %q, want %q", info.Full, "FRAAS-123")
	}
	if info.Type != "fraas" {
		t.Errorf("Type = %q, want %q", info.Type, "fraas")
	}
	if info.Number != "123" {
		t.Errorf("Number = %q, want %q", info.Number, "123")
	}
}

func TestInitCommandDescription(t *testing.T) {
	cmd := initCmd

	if cmd.Use != "init <ticket>" {
		t.Errorf("init command Use = %q, want %q", cmd.Use, "init <ticket>")
	}

	if cmd.Short == "" {
		t.Error("init command should have Short description")
	}

	if cmd.Long == "" {
		t.Error("init command should have Long description")
	}

	// Verify key information is in the description
	if !containsSubstring(cmd.Long, "worktree") {
		t.Error("init command Long description should mention 'worktree'")
	}

	if !containsSubstring(cmd.Long, "Obsidian") {
		t.Error("init command Long description should mention 'Obsidian'")
	}

	if !containsSubstring(cmd.Long, "tmux") {
		t.Error("init command Long description should mention 'tmux'")
	}
}

func TestInitCommandArgs(t *testing.T) {
	cmd := initCmd

	// Command should require exactly 1 argument
	if cmd.Args == nil {
		t.Error("init command should have Args validation")
	}
}

func TestTicketTypeNormalization(t *testing.T) {
	// Test that ticket types are normalized to lowercase
	tests := []struct {
		name         string
		ticket       string
		expectedType string
	}{
		{
			name:         "uppercase type",
			ticket:       "FRAAS-123",
			expectedType: "fraas",
		},
		{
			name:         "lowercase type",
			ticket:       "fraas-123",
			expectedType: "fraas",
		},
		{
			name:         "mixed case type",
			ticket:       "FrAaS-123",
			expectedType: "fraas",
		},
		{
			name:         "CRE uppercase",
			ticket:       "CRE-456",
			expectedType: "cre",
		},
		{
			name:         "incident mixed",
			ticket:       "Incident-789",
			expectedType: "incident",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseTicket(tt.ticket)
			if err != nil {
				t.Fatalf("parseTicket(%q) error: %v", tt.ticket, err)
			}

			if result.Type != tt.expectedType {
				t.Errorf("parseTicket(%q).Type = %q, want %q", tt.ticket, result.Type, tt.expectedType)
			}
		})
	}
}

func TestTicketFullPreservation(t *testing.T) {
	// Test that the original ticket format is preserved in Full field
	tests := []struct {
		name         string
		ticket       string
		expectedFull string
	}{
		{
			name:         "uppercase preserved",
			ticket:       "FRAAS-123",
			expectedFull: "FRAAS-123",
		},
		{
			name:         "lowercase preserved",
			ticket:       "fraas-123",
			expectedFull: "fraas-123",
		},
		{
			name:         "mixed case preserved",
			ticket:       "FrAaS-123",
			expectedFull: "FrAaS-123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseTicket(tt.ticket)
			if err != nil {
				t.Fatalf("parseTicket(%q) error: %v", tt.ticket, err)
			}

			if result.Full != tt.expectedFull {
				t.Errorf("parseTicket(%q).Full = %q, want %q", tt.ticket, result.Full, tt.expectedFull)
			}
		})
	}
}

func TestParseTicketErrorMessages(t *testing.T) {
	// Test that error messages are helpful
	_, err := parseTicket("invalid")
	if err == nil {
		t.Fatal("Expected error for invalid ticket")
	}

	errorMsg := err.Error()

	// Error should mention expected format
	if !containsSubstring(errorMsg, "TYPE-NUMBER") {
		t.Error("Error message should mention expected format TYPE-NUMBER")
	}

	// Error should give an example
	if !containsSubstring(errorMsg, "proj-123") {
		t.Error("Error message should include an example like 'proj-123'")
	}
}
