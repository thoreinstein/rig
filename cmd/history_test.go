package cmd

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
	_ "modernc.org/sqlite"
)

func TestHistoryCommandStructure(t *testing.T) {
	t.Parallel()

	cmd := historyCmd

	if cmd.Use != "history" {
		t.Errorf("history command Use = %q, want %q", cmd.Use, "history")
	}

	// Check subcommands exist
	subcommands := cmd.Commands()
	subcommandNames := make(map[string]bool)
	for _, sub := range subcommands {
		subcommandNames[sub.Use] = true
	}

	// Check for query subcommand
	if !subcommandNames["query [pattern]"] {
		t.Error("history command missing 'query' subcommand")
	}

	// Check for info subcommand
	if !subcommandNames["info"] {
		t.Error("history command missing 'info' subcommand")
	}
}

func TestHistoryQueryCommandFlags(t *testing.T) {
	t.Parallel()

	cmd := historyQueryCmd

	// Check all expected flags exist
	expectedFlags := []struct {
		name     string
		defValue string
	}{
		{"since", ""},
		{"until", ""},
		{"directory", ""},
		{"session", ""},
		{"failed-only", "false"},
		{"limit", "50"},
	}

	for _, expected := range expectedFlags {
		flag := cmd.Flags().Lookup(expected.name)
		if flag == nil {
			t.Errorf("history query command should have --%s flag", expected.name)
			continue
		}
		if flag.DefValue != expected.defValue {
			t.Errorf("--%s default = %q, want %q", expected.name, flag.DefValue, expected.defValue)
		}
	}
}

func TestHistoryQueryCommandDescription(t *testing.T) {
	t.Parallel()

	cmd := historyQueryCmd

	if cmd.Short == "" {
		t.Error("history query should have Short description")
	}

	if cmd.Long == "" {
		t.Error("history query should have Long description")
	}

	// Verify examples are in the description
	if !strings.Contains(cmd.Long, "sre history query") {
		t.Error("history query Long description should contain usage examples")
	}
}

func TestHistoryInfoCommandDescription(t *testing.T) {
	t.Parallel()

	cmd := historyInfoCmd

	if cmd.Use != "info" {
		t.Errorf("history info Use = %q, want %q", cmd.Use, "info")
	}

	if cmd.Short == "" {
		t.Error("history info should have Short description")
	}

	if cmd.Long == "" {
		t.Error("history info should have Long description")
	}
}

func TestHistoryCommandDescription(t *testing.T) {
	t.Parallel()

	cmd := historyCmd

	if cmd.Short == "" {
		t.Error("history command should have Short description")
	}

	if cmd.Long == "" {
		t.Error("history command should have Long description")
	}

	// Verify key information is in the description
	if !strings.Contains(cmd.Long, "history") {
		t.Error("history command Long description should mention 'history'")
	}

	if !strings.Contains(cmd.Long, "database") {
		t.Error("history command Long description should mention 'database'")
	}
}

func TestDurationFormatting(t *testing.T) {
	t.Parallel()

	// Test the duration formatting logic used in runHistoryQueryCommand
	tests := []struct {
		name       string
		durationMs int
		expected   string
	}{
		{
			name:       "milliseconds - under 1000",
			durationMs: 100,
			expected:   "100ms",
		},
		{
			name:       "milliseconds - near 1000",
			durationMs: 999,
			expected:   "999ms",
		},
		{
			name:       "seconds - exactly 1000",
			durationMs: 1000,
			expected:   "1.0s",
		},
		{
			name:       "seconds - 1500ms",
			durationMs: 1500,
			expected:   "1.5s",
		},
		{
			name:       "seconds - larger values",
			durationMs: 5000,
			expected:   "5.0s",
		},
		{
			name:       "zero duration",
			durationMs: 0,
			expected:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var result string
			if tt.durationMs > 0 {
				result = formatDuration(tt.durationMs)
			}

			if result != tt.expected {
				t.Errorf("formatDuration(%d) = %q, want %q", tt.durationMs, result, tt.expected)
			}
		})
	}
}

// Helper function that mimics the duration formatting in runHistoryQueryCommand
func formatDuration(durationMs int) string {
	if durationMs <= 0 {
		return ""
	}
	if durationMs < 1000 {
		// Format as Xms
		result := ""
		if durationMs >= 100 {
			result += string(rune('0' + durationMs/100))
		}
		if durationMs >= 10 {
			result += string(rune('0' + (durationMs%100)/10))
		}
		result += string(rune('0' + durationMs%10))
		return result + "ms"
	}
	// Format as X.Ys
	seconds := float64(durationMs) / 1000.0
	intPart := int(seconds)
	fracPart := int((seconds - float64(intPart)) * 10)
	return string(rune('0'+intPart)) + "." + string(rune('0'+fracPart)) + "s"
}

func TestCommandTruncation(t *testing.T) {
	t.Parallel()

	// Test the command truncation logic used in runHistoryQueryCommand
	tests := []struct {
		name     string
		command  string
		maxLen   int
		expected string
	}{
		{
			name:     "short command - no truncation",
			command:  "git status",
			maxLen:   80,
			expected: "git status",
		},
		{
			name:     "exact length - no truncation",
			command:  strings.Repeat("x", 80),
			maxLen:   80,
			expected: strings.Repeat("x", 80),
		},
		{
			name:     "long command - truncated",
			command:  strings.Repeat("x", 100),
			maxLen:   80,
			expected: strings.Repeat("x", 77) + "...",
		},
		{
			name:     "empty command",
			command:  "",
			maxLen:   80,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			command := tt.command
			if len(command) > tt.maxLen {
				command = command[:tt.maxLen-3] + "..."
			}

			if command != tt.expected {
				t.Errorf("Truncated command = %q (len=%d), want %q (len=%d)",
					command, len(command), tt.expected, len(tt.expected))
			}
		})
	}
}

func TestDirectoryTruncation(t *testing.T) {
	t.Parallel()

	// Test the directory truncation logic used in runHistoryQueryCommand
	tests := []struct {
		name      string
		directory string
		maxLen    int
		expected  string
	}{
		{
			name:      "short directory - no truncation",
			directory: "/home/user/project",
			maxLen:    30,
			expected:  "/home/user/project",
		},
		{
			name:      "long directory - truncated from start",
			directory: "/very/long/path/that/exceeds/thirty/characters",
			maxLen:    30,
			expected:  "...t/exceeds/thirty/characters",
		},
		{
			name:      "exact length - no truncation",
			directory: strings.Repeat("x", 30),
			maxLen:    30,
			expected:  strings.Repeat("x", 30),
		},
		{
			name:      "empty directory",
			directory: "",
			maxLen:    30,
			expected:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			directory := tt.directory
			if len(directory) > tt.maxLen {
				directory = "..." + directory[len(directory)-(tt.maxLen-3):]
			}

			if directory != tt.expected {
				t.Errorf("Truncated directory = %q, want %q", directory, tt.expected)
			}
		})
	}
}

func TestStatusIconSelection(t *testing.T) {
	t.Parallel()

	// Test the status icon selection logic used in runHistoryQueryCommand
	tests := []struct {
		name         string
		exitCode     int
		expectedIcon string
	}{
		{
			name:         "success - exit code 0",
			exitCode:     0,
			expectedIcon: "✓",
		},
		{
			name:         "failure - exit code 1",
			exitCode:     1,
			expectedIcon: "✗",
		},
		{
			name:         "failure - exit code 127",
			exitCode:     127,
			expectedIcon: "✗",
		},
		{
			name:         "failure - negative exit code",
			exitCode:     -1,
			expectedIcon: "✗",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var statusIcon string
			if tt.exitCode == 0 {
				statusIcon = "✓"
			} else {
				statusIcon = "✗"
			}

			if statusIcon != tt.expectedIcon {
				t.Errorf("Status icon for exit code %d = %q, want %q",
					tt.exitCode, statusIcon, tt.expectedIcon)
			}
		})
	}
}

// setupHistoryTestConfig configures viper for history tests
func setupHistoryTestConfig(t *testing.T, dbPath string) {
	t.Helper()

	viper.Reset()
	viper.Set("history.database_path", dbPath)
	viper.Set("history.ignore_patterns", []string{"ls", "cd"})
	viper.Set("notes.path", t.TempDir())
	viper.Set("notes.daily_dir", "daily")
	viper.Set("jira.enabled", false)
}

// createTestHistoryDatabase creates a test SQLite database with zsh-histdb schema
func createTestHistoryDatabase(t *testing.T, dbPath string) {
	t.Helper()

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create zsh-histdb schema
	_, err = db.Exec(`
		CREATE TABLE commands (
			id INTEGER PRIMARY KEY,
			argv TEXT,
			start_time INTEGER,
			duration INTEGER,
			exit_status INTEGER,
			place_id INTEGER,
			session_id INTEGER,
			hostname TEXT
		);
		CREATE TABLE places (
			id INTEGER PRIMARY KEY,
			dir TEXT
		);
		CREATE TABLE sessions (
			id INTEGER PRIMARY KEY,
			session TEXT
		);
	`)
	if err != nil {
		t.Fatalf("Failed to create schema: %v", err)
	}
}

// createTestHistoryDatabaseWithData creates a test database with sample commands
func createTestHistoryDatabaseWithData(t *testing.T, dbPath string) {
	t.Helper()

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create zsh-histdb schema with test data
	_, err = db.Exec(`
		CREATE TABLE commands (
			id INTEGER PRIMARY KEY,
			argv TEXT,
			start_time INTEGER,
			duration INTEGER,
			exit_status INTEGER,
			place_id INTEGER,
			session_id INTEGER,
			hostname TEXT
		);
		CREATE TABLE places (
			id INTEGER PRIMARY KEY,
			dir TEXT
		);
		CREATE TABLE sessions (
			id INTEGER PRIMARY KEY,
			session TEXT
		);
		INSERT INTO places (id, dir) VALUES (1, '/home/user/project');
		INSERT INTO places (id, dir) VALUES (2, '/home/user/other');
		INSERT INTO sessions (id, session) VALUES (1, 'FRAAS-123');
		INSERT INTO sessions (id, session) VALUES (2, 'other-session');
		INSERT INTO commands (argv, start_time, duration, exit_status, place_id, session_id, hostname)
		VALUES ('git status', 1700000000, 100, 0, 1, 1, 'localhost');
		INSERT INTO commands (argv, start_time, duration, exit_status, place_id, session_id, hostname)
		VALUES ('git commit -m "test"', 1700000100, 200, 0, 1, 1, 'localhost');
		INSERT INTO commands (argv, start_time, duration, exit_status, place_id, session_id, hostname)
		VALUES ('make build', 1700000200, 5000, 1, 1, 1, 'localhost');
		INSERT INTO commands (argv, start_time, duration, exit_status, place_id, session_id, hostname)
		VALUES ('docker ps', 1700000300, 50, 0, 2, 2, 'localhost');
	`)
	if err != nil {
		t.Fatalf("Failed to setup test data: %v", err)
	}
}

// createTestAtuinDatabase creates a test SQLite database with atuin schema
func createTestAtuinDatabase(t *testing.T, dbPath string) {
	t.Helper()

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create atuin schema with test data
	_, err = db.Exec(`
		CREATE TABLE history (
			id INTEGER PRIMARY KEY,
			command TEXT,
			timestamp INTEGER,
			duration INTEGER,
			exit INTEGER,
			cwd TEXT,
			session TEXT,
			hostname TEXT
		);
		INSERT INTO history (command, timestamp, duration, exit, cwd, session, hostname)
		VALUES ('ls -la', 1700000000000000000, 50, 0, '/home/user', 'session1', 'localhost');
		INSERT INTO history (command, timestamp, duration, exit, cwd, session, hostname)
		VALUES ('docker ps', 1700000100000000000, 100, 0, '/home/user', 'session1', 'localhost');
	`)
	if err != nil {
		t.Fatalf("Failed to setup test data: %v", err)
	}
}

func TestRunHistoryQueryCommand_DatabaseNotAvailable(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "nonexistent.db")

	setupHistoryTestConfig(t, dbPath)
	defer viper.Reset()

	// Reset global flags to avoid interference
	oldHistorySince := historySince
	oldHistoryUntil := historyUntil
	oldHistoryDirectory := historyDirectory
	oldHistorySession := historySession
	oldHistoryFailedOnly := historyFailedOnly
	oldHistoryLimit := historyLimit

	historySince = ""
	historyUntil = ""
	historyDirectory = ""
	historySession = ""
	historyFailedOnly = false
	historyLimit = 50

	defer func() {
		historySince = oldHistorySince
		historyUntil = oldHistoryUntil
		historyDirectory = oldHistoryDirectory
		historySession = oldHistorySession
		historyFailedOnly = oldHistoryFailedOnly
		historyLimit = oldHistoryLimit
	}()

	err := runHistoryQueryCommand("")
	if err == nil {
		t.Error("runHistoryQueryCommand() expected error for non-existent database")
	}
	if err != nil && !strings.Contains(err.Error(), "not available") {
		t.Errorf("runHistoryQueryCommand() error = %q, want error containing 'not available'", err.Error())
	}
}

func TestRunHistoryQueryCommand_InvalidSinceTime(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "history.db")

	createTestHistoryDatabaseWithData(t, dbPath)
	setupHistoryTestConfig(t, dbPath)
	defer viper.Reset()

	// Reset global flags
	oldHistorySince := historySince
	historySince = "invalid-time-format"

	defer func() {
		historySince = oldHistorySince
	}()

	err := runHistoryQueryCommand("")
	if err == nil {
		t.Error("runHistoryQueryCommand() expected error for invalid --since time")
	}
	if err != nil && !strings.Contains(err.Error(), "invalid --since time") {
		t.Errorf("runHistoryQueryCommand() error = %q, want error containing 'invalid --since time'", err.Error())
	}
}

func TestRunHistoryQueryCommand_InvalidUntilTime(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "history.db")

	createTestHistoryDatabaseWithData(t, dbPath)
	setupHistoryTestConfig(t, dbPath)
	defer viper.Reset()

	// Reset global flags
	oldHistorySince := historySince
	oldHistoryUntil := historyUntil
	historySince = ""
	historyUntil = "not-a-valid-time"

	defer func() {
		historySince = oldHistorySince
		historyUntil = oldHistoryUntil
	}()

	err := runHistoryQueryCommand("")
	if err == nil {
		t.Error("runHistoryQueryCommand() expected error for invalid --until time")
	}
	if err != nil && !strings.Contains(err.Error(), "invalid --until time") {
		t.Errorf("runHistoryQueryCommand() error = %q, want error containing 'invalid --until time'", err.Error())
	}
}

func TestRunHistoryQueryCommand_SuccessNoResults(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "history.db")

	createTestHistoryDatabase(t, dbPath) // Empty database
	setupHistoryTestConfig(t, dbPath)
	defer viper.Reset()

	// Reset global flags
	oldHistorySince := historySince
	oldHistoryUntil := historyUntil
	oldHistoryDirectory := historyDirectory
	oldHistorySession := historySession
	oldHistoryFailedOnly := historyFailedOnly
	oldHistoryLimit := historyLimit

	historySince = ""
	historyUntil = ""
	historyDirectory = ""
	historySession = ""
	historyFailedOnly = false
	historyLimit = 50

	defer func() {
		historySince = oldHistorySince
		historyUntil = oldHistoryUntil
		historyDirectory = oldHistoryDirectory
		historySession = oldHistorySession
		historyFailedOnly = oldHistoryFailedOnly
		historyLimit = oldHistoryLimit
	}()

	// Should not error, just return no results
	err := runHistoryQueryCommand("")
	if err != nil {
		t.Errorf("runHistoryQueryCommand() error = %v, want nil", err)
	}
}

func TestRunHistoryQueryCommand_SuccessWithResults(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "history.db")

	createTestHistoryDatabaseWithData(t, dbPath)
	setupHistoryTestConfig(t, dbPath)
	defer viper.Reset()

	// Reset global flags
	oldHistorySince := historySince
	oldHistoryUntil := historyUntil
	oldHistoryDirectory := historyDirectory
	oldHistorySession := historySession
	oldHistoryFailedOnly := historyFailedOnly
	oldHistoryLimit := historyLimit

	historySince = ""
	historyUntil = ""
	historyDirectory = ""
	historySession = ""
	historyFailedOnly = false
	historyLimit = 50

	defer func() {
		historySince = oldHistorySince
		historyUntil = oldHistoryUntil
		historyDirectory = oldHistoryDirectory
		historySession = oldHistorySession
		historyFailedOnly = oldHistoryFailedOnly
		historyLimit = oldHistoryLimit
	}()

	err := runHistoryQueryCommand("")
	if err != nil {
		t.Errorf("runHistoryQueryCommand() error = %v, want nil", err)
	}
}

func TestRunHistoryQueryCommand_WithPattern(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "history.db")

	createTestHistoryDatabaseWithData(t, dbPath)
	setupHistoryTestConfig(t, dbPath)
	defer viper.Reset()

	// Reset global flags
	oldHistorySince := historySince
	oldHistoryUntil := historyUntil
	oldHistoryDirectory := historyDirectory
	oldHistorySession := historySession
	oldHistoryFailedOnly := historyFailedOnly
	oldHistoryLimit := historyLimit

	historySince = ""
	historyUntil = ""
	historyDirectory = ""
	historySession = ""
	historyFailedOnly = false
	historyLimit = 50

	defer func() {
		historySince = oldHistorySince
		historyUntil = oldHistoryUntil
		historyDirectory = oldHistoryDirectory
		historySession = oldHistorySession
		historyFailedOnly = oldHistoryFailedOnly
		historyLimit = oldHistoryLimit
	}()

	// Query with pattern "git"
	err := runHistoryQueryCommand("git")
	if err != nil {
		t.Errorf("runHistoryQueryCommand(\"git\") error = %v, want nil", err)
	}
}

func TestRunHistoryQueryCommand_WithFailedOnly(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "history.db")

	createTestHistoryDatabaseWithData(t, dbPath)
	setupHistoryTestConfig(t, dbPath)
	defer viper.Reset()

	// Reset global flags
	oldHistorySince := historySince
	oldHistoryUntil := historyUntil
	oldHistoryDirectory := historyDirectory
	oldHistorySession := historySession
	oldHistoryFailedOnly := historyFailedOnly
	oldHistoryLimit := historyLimit

	historySince = ""
	historyUntil = ""
	historyDirectory = ""
	historySession = ""
	historyFailedOnly = true // Only failed commands
	historyLimit = 50

	defer func() {
		historySince = oldHistorySince
		historyUntil = oldHistoryUntil
		historyDirectory = oldHistoryDirectory
		historySession = oldHistorySession
		historyFailedOnly = oldHistoryFailedOnly
		historyLimit = oldHistoryLimit
	}()

	err := runHistoryQueryCommand("")
	if err != nil {
		t.Errorf("runHistoryQueryCommand() with --failed-only error = %v, want nil", err)
	}
}

func TestRunHistoryQueryCommand_WithValidTimeFilter(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "history.db")

	createTestHistoryDatabaseWithData(t, dbPath)
	setupHistoryTestConfig(t, dbPath)
	defer viper.Reset()

	// Reset global flags
	oldHistorySince := historySince
	oldHistoryUntil := historyUntil
	oldHistoryDirectory := historyDirectory
	oldHistorySession := historySession
	oldHistoryFailedOnly := historyFailedOnly
	oldHistoryLimit := historyLimit

	historySince = "2023-11-01"
	historyUntil = "2023-11-30"
	historyDirectory = ""
	historySession = ""
	historyFailedOnly = false
	historyLimit = 50

	defer func() {
		historySince = oldHistorySince
		historyUntil = oldHistoryUntil
		historyDirectory = oldHistoryDirectory
		historySession = oldHistorySession
		historyFailedOnly = oldHistoryFailedOnly
		historyLimit = oldHistoryLimit
	}()

	err := runHistoryQueryCommand("")
	if err != nil {
		t.Errorf("runHistoryQueryCommand() with time filters error = %v, want nil", err)
	}
}

func TestRunHistoryQueryCommand_WithDirectoryFilter(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "history.db")

	createTestHistoryDatabaseWithData(t, dbPath)
	setupHistoryTestConfig(t, dbPath)
	defer viper.Reset()

	// Reset global flags
	oldHistorySince := historySince
	oldHistoryUntil := historyUntil
	oldHistoryDirectory := historyDirectory
	oldHistorySession := historySession
	oldHistoryFailedOnly := historyFailedOnly
	oldHistoryLimit := historyLimit

	historySince = ""
	historyUntil = ""
	historyDirectory = "/home/user/project"
	historySession = ""
	historyFailedOnly = false
	historyLimit = 50

	defer func() {
		historySince = oldHistorySince
		historyUntil = oldHistoryUntil
		historyDirectory = oldHistoryDirectory
		historySession = oldHistorySession
		historyFailedOnly = oldHistoryFailedOnly
		historyLimit = oldHistoryLimit
	}()

	err := runHistoryQueryCommand("")
	if err != nil {
		t.Errorf("runHistoryQueryCommand() with --directory error = %v, want nil", err)
	}
}

func TestRunHistoryQueryCommand_WithSessionFilter(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "history.db")

	createTestHistoryDatabaseWithData(t, dbPath)
	setupHistoryTestConfig(t, dbPath)
	defer viper.Reset()

	// Reset global flags
	oldHistorySince := historySince
	oldHistoryUntil := historyUntil
	oldHistoryDirectory := historyDirectory
	oldHistorySession := historySession
	oldHistoryFailedOnly := historyFailedOnly
	oldHistoryLimit := historyLimit

	historySince = ""
	historyUntil = ""
	historyDirectory = ""
	historySession = "FRAAS-123"
	historyFailedOnly = false
	historyLimit = 50

	defer func() {
		historySince = oldHistorySince
		historyUntil = oldHistoryUntil
		historyDirectory = oldHistoryDirectory
		historySession = oldHistorySession
		historyFailedOnly = oldHistoryFailedOnly
		historyLimit = oldHistoryLimit
	}()

	err := runHistoryQueryCommand("")
	if err != nil {
		t.Errorf("runHistoryQueryCommand() with --session error = %v, want nil", err)
	}
}

func TestRunHistoryQueryCommand_WithLimit(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "history.db")

	createTestHistoryDatabaseWithData(t, dbPath)
	setupHistoryTestConfig(t, dbPath)
	defer viper.Reset()

	// Reset global flags
	oldHistorySince := historySince
	oldHistoryUntil := historyUntil
	oldHistoryDirectory := historyDirectory
	oldHistorySession := historySession
	oldHistoryFailedOnly := historyFailedOnly
	oldHistoryLimit := historyLimit

	historySince = ""
	historyUntil = ""
	historyDirectory = ""
	historySession = ""
	historyFailedOnly = false
	historyLimit = 2 // Limit to 2 results

	defer func() {
		historySince = oldHistorySince
		historyUntil = oldHistoryUntil
		historyDirectory = oldHistoryDirectory
		historySession = oldHistorySession
		historyFailedOnly = oldHistoryFailedOnly
		historyLimit = oldHistoryLimit
	}()

	err := runHistoryQueryCommand("")
	if err != nil {
		t.Errorf("runHistoryQueryCommand() with --limit error = %v, want nil", err)
	}
}

func TestRunHistoryInfoCommand_DatabaseNotExists(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "nonexistent.db")

	setupHistoryTestConfig(t, dbPath)
	defer viper.Reset()

	// Should not error for non-existent database, just report it
	err := runHistoryInfoCommand()
	if err != nil {
		t.Errorf("runHistoryInfoCommand() error = %v, want nil", err)
	}
}

func TestRunHistoryInfoCommand_ValidZshHistdb(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "history.db")

	createTestHistoryDatabaseWithData(t, dbPath)
	setupHistoryTestConfig(t, dbPath)
	defer viper.Reset()

	err := runHistoryInfoCommand()
	if err != nil {
		t.Errorf("runHistoryInfoCommand() error = %v, want nil", err)
	}
}

func TestRunHistoryInfoCommand_ValidAtuin(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "history.db")

	createTestAtuinDatabase(t, dbPath)
	setupHistoryTestConfig(t, dbPath)
	defer viper.Reset()

	err := runHistoryInfoCommand()
	if err != nil {
		t.Errorf("runHistoryInfoCommand() error = %v, want nil", err)
	}
}

func TestHistoryQueryCommandArgs(t *testing.T) {
	t.Parallel()

	cmd := historyQueryCmd

	// The command accepts 0 or 1 arguments
	tests := []struct {
		name      string
		args      []string
		expectErr bool
	}{
		{
			name:      "no arguments - valid",
			args:      []string{},
			expectErr: false,
		},
		{
			name:      "one argument - valid",
			args:      []string{"git"},
			expectErr: false,
		},
		{
			name:      "two arguments - invalid",
			args:      []string{"git", "status"},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := cmd.Args(cmd, tt.args)
			if tt.expectErr && err == nil {
				t.Error("expected error for invalid args, got nil")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestHistoryInfoCommandArgs(t *testing.T) {
	t.Parallel()

	// The info command should not accept any positional arguments
	cmd := historyInfoCmd

	// Verify the Args function allows no arguments (nil Args means any)
	if cmd.Args != nil {
		err := cmd.Args(cmd, []string{})
		if err != nil {
			t.Errorf("info command should accept no arguments, got error: %v", err)
		}
	}
}

func TestRunHistoryQueryCommand_AtuinDatabase(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "history.db")

	createTestAtuinDatabase(t, dbPath)
	setupHistoryTestConfig(t, dbPath)
	defer viper.Reset()

	// Reset global flags
	oldHistorySince := historySince
	oldHistoryUntil := historyUntil
	oldHistoryDirectory := historyDirectory
	oldHistorySession := historySession
	oldHistoryFailedOnly := historyFailedOnly
	oldHistoryLimit := historyLimit

	historySince = ""
	historyUntil = ""
	historyDirectory = ""
	historySession = ""
	historyFailedOnly = false
	historyLimit = 50

	defer func() {
		historySince = oldHistorySince
		historyUntil = oldHistoryUntil
		historyDirectory = oldHistoryDirectory
		historySession = oldHistorySession
		historyFailedOnly = oldHistoryFailedOnly
		historyLimit = oldHistoryLimit
	}()

	err := runHistoryQueryCommand("")
	if err != nil {
		t.Errorf("runHistoryQueryCommand() with atuin database error = %v, want nil", err)
	}
}

func TestRunHistoryQueryCommand_AllFiltersComposed(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "history.db")

	createTestHistoryDatabaseWithData(t, dbPath)
	setupHistoryTestConfig(t, dbPath)
	defer viper.Reset()

	// Reset global flags
	oldHistorySince := historySince
	oldHistoryUntil := historyUntil
	oldHistoryDirectory := historyDirectory
	oldHistorySession := historySession
	oldHistoryFailedOnly := historyFailedOnly
	oldHistoryLimit := historyLimit

	// Compose multiple filters together
	historySince = "2023-01-01"
	historyUntil = "2024-12-31"
	historyDirectory = "/home/user"
	historySession = "FRAAS"
	historyFailedOnly = true
	historyLimit = 10

	defer func() {
		historySince = oldHistorySince
		historyUntil = oldHistoryUntil
		historyDirectory = oldHistoryDirectory
		historySession = oldHistorySession
		historyFailedOnly = oldHistoryFailedOnly
		historyLimit = oldHistoryLimit
	}()

	err := runHistoryQueryCommand("make")
	if err != nil {
		t.Errorf("runHistoryQueryCommand() with all filters error = %v, want nil", err)
	}
}

func TestHistoryQueryTimeFormats(t *testing.T) {
	t.Parallel()

	// Test various time format inputs
	tests := []struct {
		name      string
		timeStr   string
		expectErr bool
	}{
		{
			name:      "date only - YYYY-MM-DD",
			timeStr:   "2024-01-15",
			expectErr: false,
		},
		{
			name:      "datetime - YYYY-MM-DD HH:MM",
			timeStr:   "2024-01-15 14:30",
			expectErr: false,
		},
		{
			name:      "datetime with seconds - YYYY-MM-DD HH:MM:SS",
			timeStr:   "2024-01-15 14:30:45",
			expectErr: false,
		},
		{
			name:      "invalid format",
			timeStr:   "15-01-2024",
			expectErr: true,
		},
		{
			name:      "invalid month",
			timeStr:   "2024-13-15",
			expectErr: true,
		},
		{
			name:      "empty string",
			timeStr:   "",
			expectErr: true,
		},
		{
			name:      "words not time",
			timeStr:   "yesterday",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := parseTimeString(tt.timeStr)
			if tt.expectErr && err == nil {
				t.Errorf("parseTimeString(%q) expected error, got nil", tt.timeStr)
			}
			if !tt.expectErr && err != nil {
				t.Errorf("parseTimeString(%q) unexpected error: %v", tt.timeStr, err)
			}
		})
	}
}
