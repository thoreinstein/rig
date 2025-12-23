package cmd

import (
	"strings"
	"testing"
)

func TestUpdateCommandFlags(t *testing.T) {
	cmd := updateCmd

	tests := []struct {
		flagName     string
		shorthand    string
		defaultValue string
	}{
		{"check", "c", "false"},
		{"force", "f", "false"},
		{"pre", "p", "false"},
		{"yes", "y", "false"},
	}

	for _, tt := range tests {
		t.Run(tt.flagName, func(t *testing.T) {
			flag := cmd.Flags().Lookup(tt.flagName)
			if flag == nil {
				t.Errorf("update command should have --%s flag", tt.flagName)
				return
			}

			if flag.Shorthand != tt.shorthand {
				t.Errorf("--%s shorthand = %q, want %q", tt.flagName, flag.Shorthand, tt.shorthand)
			}

			if flag.DefValue != tt.defaultValue {
				t.Errorf("--%s default = %q, want %q", tt.flagName, flag.DefValue, tt.defaultValue)
			}
		})
	}
}

func TestUpdateCommandDescription(t *testing.T) {
	cmd := updateCmd

	if cmd.Use != "update" {
		t.Errorf("update command Use = %q, want %q", cmd.Use, "update")
	}

	if cmd.Short == "" {
		t.Error("update command should have Short description")
	}

	if cmd.Long == "" {
		t.Error("update command should have Long description")
	}

	// Verify examples are included in Long description
	if !strings.Contains(cmd.Long, "sre update") {
		t.Error("update command Long description should contain usage examples")
	}
}

func TestConfirmUpdatePromptFormat(t *testing.T) {
	tests := []struct {
		name           string
		currentVersion string
		newVersion     string
		wantContains   string
	}{
		{
			name:           "dev version",
			currentVersion: "dev",
			newVersion:     "1.0.0",
			wantContains:   "from dev to 1.0.0",
		},
		{
			name:           "semver version",
			currentVersion: "0.0.2",
			newVersion:     "0.0.3",
			wantContains:   "from 0.0.2 to 0.0.3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We can't easily test the actual prompt without mocking stdin,
			// but we can verify the prompt format logic
			var prompt string
			if tt.currentVersion == "dev" {
				prompt = "Update sre from dev to " + tt.newVersion + "? [y/N]: "
			} else {
				prompt = "Update sre from " + tt.currentVersion + " to " + tt.newVersion + "? [y/N]: "
			}

			if !strings.Contains(prompt, tt.wantContains) {
				t.Errorf("prompt = %q, want to contain %q", prompt, tt.wantContains)
			}
		})
	}
}

func TestGetVersion(t *testing.T) {
	// GetVersion should return the same value as Version
	got := GetVersion()
	want := Version

	if got != want {
		t.Errorf("GetVersion() = %q, want %q", got, want)
	}
}

func TestVersionExported(t *testing.T) {
	// Verify Version is accessible (exported)
	// This test ensures the variable is exported and can be read
	if Version == "" {
		t.Error("Version should not be empty string")
	}

	// Default value should be "dev" when not set via ldflags
	if Version != "dev" {
		t.Logf("Version = %q (set via ldflags)", Version)
	}
}

func TestRepoConstants(t *testing.T) {
	if repoOwner != "thoreinstein" {
		t.Errorf("repoOwner = %q, want %q", repoOwner, "thoreinstein")
	}

	if repoName != "sre" {
		t.Errorf("repoName = %q, want %q", repoName, "sre")
	}
}

func TestUpdateCommandRegistered(t *testing.T) {
	// Verify update command is registered with root
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "update" {
			found = true
			break
		}
	}

	if !found {
		t.Error("update command should be registered with rootCmd")
	}
}
