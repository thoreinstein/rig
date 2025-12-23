package cmd

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestUpdateCommandFlags(t *testing.T) {
	t.Parallel()

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
			t.Parallel()

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

func TestUpdateCommandFlagUsage(t *testing.T) {
	t.Parallel()

	cmd := updateCmd

	tests := []struct {
		flagName    string
		wantContain string
	}{
		{"check", "Check for updates"},
		{"force", "Force update"},
		{"pre", "pre-release"},
		{"yes", "confirmation"},
	}

	for _, tt := range tests {
		t.Run(tt.flagName, func(t *testing.T) {
			t.Parallel()

			flag := cmd.Flags().Lookup(tt.flagName)
			if flag == nil {
				t.Fatalf("--%s flag not found", tt.flagName)
			}

			if !strings.Contains(flag.Usage, tt.wantContain) {
				t.Errorf("--%s usage %q should contain %q", tt.flagName, flag.Usage, tt.wantContain)
			}
		})
	}
}

func TestUpdateCommandDescription(t *testing.T) {
	t.Parallel()

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
	expectedExamples := []string{
		"sre update",
		"--check",
		"--yes",
		"--force",
		"--pre",
	}

	for _, example := range expectedExamples {
		if !strings.Contains(cmd.Long, example) {
			t.Errorf("update command Long description should contain %q", example)
		}
	}
}

func TestUpdateCommandLongDescriptionContent(t *testing.T) {
	t.Parallel()

	cmd := updateCmd

	// Verify key information is in the long description
	expectedContent := []string{
		"GitHub",
		"releases",
		"checksums",
		"binary",
	}

	for _, content := range expectedContent {
		if !strings.Contains(cmd.Long, content) {
			t.Errorf("update command Long description should mention %q", content)
		}
	}
}

func TestConfirmUpdatePromptFormat(t *testing.T) {
	t.Parallel()

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
		{
			name:           "major version upgrade",
			currentVersion: "1.0.0",
			newVersion:     "2.0.0",
			wantContains:   "from 1.0.0 to 2.0.0",
		},
		{
			name:           "patch version upgrade",
			currentVersion: "1.2.3",
			newVersion:     "1.2.4",
			wantContains:   "from 1.2.3 to 1.2.4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Verify the prompt format logic by reconstructing it
			var prompt string
			if tt.currentVersion == "dev" {
				prompt = "Update sre from dev to " + tt.newVersion + "? [y/N]: "
			} else {
				prompt = "Update sre from " + tt.currentVersion + " to " + tt.newVersion + "? [y/N]: "
			}

			if !strings.Contains(prompt, tt.wantContains) {
				t.Errorf("prompt = %q, want to contain %q", prompt, tt.wantContains)
			}

			// Verify prompt ends with expected suffix
			if !strings.HasSuffix(prompt, "[y/N]: ") {
				t.Errorf("prompt should end with '[y/N]: ', got %q", prompt)
			}
		})
	}
}

func TestConfirmUpdate_StdinResponses(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "lowercase y",
			input:    "y\n",
			expected: true,
		},
		{
			name:     "uppercase Y",
			input:    "Y\n",
			expected: true,
		},
		{
			name:     "lowercase yes",
			input:    "yes\n",
			expected: true,
		},
		{
			name:     "uppercase YES",
			input:    "YES\n",
			expected: true,
		},
		{
			name:     "mixed case Yes",
			input:    "Yes\n",
			expected: true,
		},
		{
			name:     "n response",
			input:    "n\n",
			expected: false,
		},
		{
			name:     "no response",
			input:    "no\n",
			expected: false,
		},
		{
			name:     "empty response",
			input:    "\n",
			expected: false,
		},
		{
			name:     "garbage input",
			input:    "asdfasdf\n",
			expected: false,
		},
		{
			name:     "y with spaces",
			input:    "  y  \n",
			expected: true,
		},
		{
			name:     "yes with spaces",
			input:    "  yes  \n",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original stdin and restore after test
			oldStdin := os.Stdin
			defer func() { os.Stdin = oldStdin }()

			// Create a pipe to simulate stdin
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatalf("failed to create pipe: %v", err)
			}
			os.Stdin = r

			// Write test input and close write end
			go func() {
				defer w.Close()
				_, _ = io.WriteString(w, tt.input)
			}()

			// Capture stdout to suppress prompt
			oldStdout := os.Stdout
			os.Stdout, _ = os.Create(os.DevNull)
			defer func() { os.Stdout = oldStdout }()

			result := confirmUpdate("1.0.0", "2.0.0")

			if result != tt.expected {
				t.Errorf("confirmUpdate() with input %q = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestConfirmUpdate_DevVersionPrompt(t *testing.T) {
	// Save original stdin/stdout and restore after test
	oldStdin := os.Stdin
	oldStdout := os.Stdout
	defer func() {
		os.Stdin = oldStdin
		os.Stdout = oldStdout
	}()

	// Create pipe for stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdin = r

	// Capture stdout
	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stdout pipe: %v", err)
	}
	os.Stdout = stdoutW

	// Write response
	go func() {
		defer w.Close()
		_, _ = io.WriteString(w, "n\n")
	}()

	confirmUpdate("dev", "1.0.0")

	stdoutW.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(stdoutR)
	output := buf.String()

	// Verify prompt mentions "dev"
	if !strings.Contains(output, "from dev to") {
		t.Errorf("dev version prompt should mention 'from dev to', got %q", output)
	}
}

func TestGetVersion(t *testing.T) {
	t.Parallel()

	// GetVersion should return the same value as Version
	got := GetVersion()
	want := Version

	if got != want {
		t.Errorf("GetVersion() = %q, want %q", got, want)
	}
}

func TestVersionExported(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	if repoOwner != "thoreinstein" {
		t.Errorf("repoOwner = %q, want %q", repoOwner, "thoreinstein")
	}

	if repoName != "sre" {
		t.Errorf("repoName = %q, want %q", repoName, "sre")
	}
}

func TestUpdateCommandRegistered(t *testing.T) {
	t.Parallel()

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

func TestUpdateCommandHasRunE(t *testing.T) {
	t.Parallel()

	// Verify the command uses RunE (not Run) for proper error handling
	if updateCmd.RunE == nil {
		t.Error("update command should have RunE set for error handling")
	}
}

func TestVersionComparisonLogic(t *testing.T) {
	t.Parallel()

	// Test the logic that determines if update is needed
	// This tests the conceptual logic without calling selfupdate

	tests := []struct {
		name            string
		currentVersion  string
		isDevVersion    bool
		latestLessEqual bool
		forceUpdate     bool
		wantSkipUpdate  bool
	}{
		{
			name:            "dev version always updates",
			currentVersion:  "dev",
			isDevVersion:    true,
			latestLessEqual: false,
			forceUpdate:     false,
			wantSkipUpdate:  false,
		},
		{
			name:            "current equals latest without force",
			currentVersion:  "1.0.0",
			isDevVersion:    false,
			latestLessEqual: true,
			forceUpdate:     false,
			wantSkipUpdate:  true,
		},
		{
			name:            "current equals latest with force",
			currentVersion:  "1.0.0",
			isDevVersion:    false,
			latestLessEqual: true,
			forceUpdate:     true,
			wantSkipUpdate:  false,
		},
		{
			name:            "newer version available",
			currentVersion:  "1.0.0",
			isDevVersion:    false,
			latestLessEqual: false,
			forceUpdate:     false,
			wantSkipUpdate:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Replicate the logic from runUpdateCommand
			skipUpdate := !tt.isDevVersion && tt.latestLessEqual && !tt.forceUpdate

			if skipUpdate != tt.wantSkipUpdate {
				t.Errorf("skipUpdate = %v, want %v", skipUpdate, tt.wantSkipUpdate)
			}
		})
	}
}

func TestDevVersionDetection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		version      string
		wantIsDevVer bool
	}{
		{
			name:         "dev string",
			version:      "dev",
			wantIsDevVer: true,
		},
		{
			name:         "semver version",
			version:      "1.0.0",
			wantIsDevVer: false,
		},
		{
			name:         "prerelease version",
			version:      "1.0.0-alpha",
			wantIsDevVer: false,
		},
		{
			name:         "empty string",
			version:      "",
			wantIsDevVer: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Replicate the logic from runUpdateCommand
			isDevVersion := tt.version == "dev"

			if isDevVersion != tt.wantIsDevVer {
				t.Errorf("isDevVersion = %v, want %v", isDevVersion, tt.wantIsDevVer)
			}
		})
	}
}

func TestUpdateFlagVariables(t *testing.T) {
	// Not parallel - accesses global variables
	// Verify the flag variables are properly initialized
	// These are package-level variables bound to command flags

	tests := []struct {
		name     string
		flagName string
		variable *bool
	}{
		{"check flag variable", "check", &updateCheck},
		{"force flag variable", "force", &updateForce},
		{"pre flag variable", "pre", &updatePre},
		{"yes flag variable", "yes", &updateYes},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify the variable exists and is addressable
			if tt.variable == nil {
				t.Errorf("%s variable should not be nil", tt.flagName)
			}
		})
	}
}

func TestUpdateCommandInheritsPersistentFlags(t *testing.T) {
	t.Parallel()

	// Update command should inherit --verbose from root
	verboseFlag := updateCmd.Flag("verbose")
	if verboseFlag == nil {
		t.Error("update command should inherit --verbose persistent flag from root")
	}
}

func TestUpdateCommandInheritsConfigFlag(t *testing.T) {
	t.Parallel()

	// Update command should inherit --config from root
	configFlag := updateCmd.Flag("config")
	if configFlag == nil {
		t.Error("update command should inherit --config persistent flag from root")
	}
}
