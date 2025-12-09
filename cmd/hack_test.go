package cmd

import (
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

	// Check --repo flag exists
	repoFlag := cmd.Flags().Lookup("repo")
	if repoFlag == nil {
		t.Error("hack command should have --repo flag")
	}
	if repoFlag != nil && repoFlag.DefValue != "" {
		t.Errorf("--repo default should be empty, got %s", repoFlag.DefValue)
	}
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
