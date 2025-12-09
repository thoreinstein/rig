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
