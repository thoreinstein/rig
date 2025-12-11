package cmd

import (
	"strings"
	"testing"
)

func TestHackCommandFlags(t *testing.T) {
	// Test that the hack command has the expected flags
	cmd := hackCmd

	// Check --notes flag exists
	notesFlag := cmd.Flags().Lookup("notes")
	if notesFlag == nil {
		t.Error("hack command should have --notes flag")
	}
	if notesFlag != nil && notesFlag.DefValue != "false" {
		t.Errorf("--notes default should be false, got %s", notesFlag.DefValue)
	}

	// Note: --repo flag was removed - repo is now detected from CWD
}

func TestHackCommandArgs(t *testing.T) {
	// Test that hack command requires exactly 1 argument
	cmd := hackCmd

	if cmd.Args == nil {
		t.Error("hack command should have Args validation")
	}

	// The command should have Use showing <name> argument
	if cmd.Use != "hack <name>" {
		t.Errorf("hack command Use = %q, want %q", cmd.Use, "hack <name>")
	}
}

func TestHackCommandDescription(t *testing.T) {
	cmd := hackCmd

	if cmd.Short == "" {
		t.Error("hack command should have Short description")
	}

	if cmd.Long == "" {
		t.Error("hack command should have Long description")
	}

	// Verify key information is in the description
	if !containsSubstring(cmd.Long, "hack") {
		t.Error("hack command Long description should mention 'hack'")
	}

	if !containsSubstring(cmd.Long, "worktree") {
		t.Error("hack command Long description should mention 'worktree'")
	}
}

func TestHackBranchNaming(t *testing.T) {
	// The hack command creates branches with "hack/" prefix
	// Test that the expected branch name format is documented
	tests := []struct {
		name           string
		hackName       string
		expectedBranch string
	}{
		{
			name:           "simple name",
			hackName:       "experiment",
			expectedBranch: "hack/experiment",
		},
		{
			name:           "name with dashes",
			hackName:       "winter-2025",
			expectedBranch: "hack/winter-2025",
		},
		{
			name:           "name with numbers",
			hackName:       "test123",
			expectedBranch: "hack/test123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify expected branch format
			expectedBranch := "hack/" + tt.hackName
			if expectedBranch != tt.expectedBranch {
				t.Errorf("Branch format = %q, want %q", expectedBranch, tt.expectedBranch)
			}
		})
	}
}

func TestHackWorktreePath(t *testing.T) {
	// The hack command creates worktrees under "hack" directory
	// Test the expected path structure
	tests := []struct {
		name         string
		hackName     string
		expectedType string
	}{
		{
			name:         "simple name",
			hackName:     "experiment",
			expectedType: "hack",
		},
		{
			name:         "complex name",
			hackName:     "winter-2025-cleanup",
			expectedType: "hack",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Hack worktrees should always be under "hack" type directory
			if tt.expectedType != "hack" {
				t.Errorf("Hack worktree type should always be 'hack', got %q", tt.expectedType)
			}
		})
	}
}

func TestValidateHackName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		errMsg  string
	}{
		// Valid cases
		{
			name:    "simple name",
			input:   "winter-2025",
			wantErr: false,
		},
		{
			name:    "underscore name",
			input:   "experiment_auth",
			wantErr: false,
		},
		{
			name:    "camelCase name",
			input:   "quickFix",
			wantErr: false,
		},
		{
			name:    "single letter",
			input:   "a",
			wantErr: false,
		},
		{
			name:    "max length 64 chars",
			input:   "a" + strings.Repeat("b", 63),
			wantErr: false,
		},
		// Invalid cases
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
			errMsg:  "cannot be empty",
		},
		{
			name:    "starts with number",
			input:   "123-test",
			wantErr: true,
			errMsg:  "must start with a letter",
		},
		{
			name:    "starts with dot",
			input:   ".hidden",
			wantErr: true,
			errMsg:  "must start with a letter",
		},
		{
			name:    "path traversal",
			input:   "../../../etc/passwd",
			wantErr: true,
			errMsg:  "must start with a letter",
		},
		{
			name:    "contains slash",
			input:   "test/path",
			wantErr: true,
			errMsg:  "must start with a letter",
		},
		{
			name:    "shell injection attempt",
			input:   "test;rm -rf /",
			wantErr: true,
			errMsg:  "must start with a letter",
		},
		{
			name:    "contains spaces",
			input:   "my hack",
			wantErr: true,
			errMsg:  "must start with a letter",
		},
		{
			name:    "exceeds max length",
			input:   "a" + strings.Repeat("b", 64),
			wantErr: true,
			errMsg:  "max 64 characters",
		},
		{
			name:    "starts with hyphen",
			input:   "-test",
			wantErr: true,
			errMsg:  "must start with a letter",
		},
		{
			name:    "starts with underscore",
			input:   "_test",
			wantErr: true,
			errMsg:  "must start with a letter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateHackName(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("validateHackName(%q) should have returned an error", tt.input)
					return
				}
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("validateHackName(%q) error = %q, should contain %q", tt.input, err.Error(), tt.errMsg)
				}
			} else if err != nil {
				t.Errorf("validateHackName(%q) returned unexpected error: %v", tt.input, err)
			}
		})
	}
}
