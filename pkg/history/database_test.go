package history

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func TestNewDatabaseManager(t *testing.T) {
	dm := NewDatabaseManager("/path/to/db", true)

	if dm.DatabasePath != "/path/to/db" {
		t.Errorf("DatabasePath = %q, want %q", dm.DatabasePath, "/path/to/db")
	}
	if !dm.Verbose {
		t.Error("Verbose should be true")
	}
}

func TestIsAvailable_NonExistent(t *testing.T) {
	dm := NewDatabaseManager("/nonexistent/path/to/db.sqlite", false)

	if dm.IsAvailable() {
		t.Error("IsAvailable() should return false for non-existent database")
	}
}

func TestIsAvailable_ValidZshHistdb(t *testing.T) {
	// Create a temporary database with zsh-histdb schema
	tmpDir := t.TempDir()

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}

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
	db.Close()

	dm := NewDatabaseManager(dbPath, false)
	if !dm.IsAvailable() {
		t.Error("IsAvailable() should return true for valid zsh-histdb database")
	}
}

func TestIsAvailable_ValidAtuin(t *testing.T) {
	// Create a temporary database with atuin schema
	tmpDir := t.TempDir()

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}

	// Create atuin schema
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
	`)
	if err != nil {
		t.Fatalf("Failed to create schema: %v", err)
	}
	db.Close()

	dm := NewDatabaseManager(dbPath, false)
	if !dm.IsAvailable() {
		t.Error("IsAvailable() should return true for valid atuin database")
	}
}

func TestDetectSchema_ZshHistdb(t *testing.T) {
	tmpDir := t.TempDir()

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create zsh-histdb schema
	_, err = db.Exec(`
		CREATE TABLE commands (id INTEGER PRIMARY KEY);
		CREATE TABLE places (id INTEGER PRIMARY KEY);
		CREATE TABLE sessions (id INTEGER PRIMARY KEY);
	`)
	if err != nil {
		t.Fatalf("Failed to create schema: %v", err)
	}

	dm := NewDatabaseManager(dbPath, false)
	schema, err := dm.detectSchema(db)
	if err != nil {
		t.Fatalf("detectSchema() error: %v", err)
	}
	if schema != SchemaZshHistdb {
		t.Errorf("detectSchema() = %v, want %v", schema, SchemaZshHistdb)
	}
}

func TestDetectSchema_Atuin(t *testing.T) {
	tmpDir := t.TempDir()

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create atuin schema
	_, err = db.Exec(`CREATE TABLE history (id INTEGER PRIMARY KEY);`)
	if err != nil {
		t.Fatalf("Failed to create schema: %v", err)
	}

	dm := NewDatabaseManager(dbPath, false)
	schema, err := dm.detectSchema(db)
	if err != nil {
		t.Fatalf("detectSchema() error: %v", err)
	}
	if schema != SchemaAtuin {
		t.Errorf("detectSchema() = %v, want %v", schema, SchemaAtuin)
	}
}

func TestDetectSchema_Unknown(t *testing.T) {
	tmpDir := t.TempDir()

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create unrecognized schema
	_, err = db.Exec(`CREATE TABLE other_table (id INTEGER PRIMARY KEY);`)
	if err != nil {
		t.Fatalf("Failed to create schema: %v", err)
	}

	dm := NewDatabaseManager(dbPath, false)
	schema, err := dm.detectSchema(db)
	if err == nil {
		t.Error("detectSchema() expected error for unknown schema")
	}
	if schema != SchemaUnknown {
		t.Errorf("detectSchema() = %v, want %v", schema, SchemaUnknown)
	}
}

func TestBuildZshHistdbQuery_BasicOptions(t *testing.T) {
	dm := NewDatabaseManager("", false)

	tests := []struct {
		name           string
		options        QueryOptions
		expectContains []string
		expectArgs     int
	}{
		{
			name:           "empty options",
			options:        QueryOptions{},
			expectContains: []string{"SELECT", "FROM commands", "ORDER BY"},
			expectArgs:     0,
		},
		{
			name: "with pattern",
			options: QueryOptions{
				Pattern: "git",
			},
			expectContains: []string{"argv LIKE"},
			expectArgs:     1,
		},
		{
			name: "with directory",
			options: QueryOptions{
				Directory: "/home/user",
			},
			expectContains: []string{"p.dir LIKE"},
			expectArgs:     1,
		},
		{
			name: "with limit",
			options: QueryOptions{
				Limit: 100,
			},
			expectContains: []string{"LIMIT"},
			expectArgs:     1,
		},
		{
			name: "with ticket",
			options: QueryOptions{
				Ticket: "FRAAS-123",
			},
			expectContains: []string{"session LIKE", "argv LIKE"},
			expectArgs:     2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, args := dm.buildZshHistdbQuery(tt.options)

			for _, expected := range tt.expectContains {
				if !containsString(query, expected) {
					t.Errorf("query missing expected string %q: %s", expected, query)
				}
			}

			if len(args) != tt.expectArgs {
				t.Errorf("got %d args, want %d", len(args), tt.expectArgs)
			}
		})
	}
}

func TestBuildZshHistdbQuery_TimeFilters(t *testing.T) {
	dm := NewDatabaseManager("", false)

	now := time.Now()
	yesterday := now.Add(-24 * time.Hour)

	options := QueryOptions{
		Since: &yesterday,
		Until: &now,
	}

	query, args := dm.buildZshHistdbQuery(options)

	if !containsString(query, "start_time >=") {
		t.Error("query missing Since filter")
	}
	if !containsString(query, "start_time <=") {
		t.Error("query missing Until filter")
	}
	if len(args) != 2 {
		t.Errorf("got %d args, want 2", len(args))
	}
}

func TestBuildZshHistdbQuery_ExitCode(t *testing.T) {
	dm := NewDatabaseManager("", false)

	exitCode := 1
	options := QueryOptions{
		ExitCode: &exitCode,
	}

	query, args := dm.buildZshHistdbQuery(options)

	if !containsString(query, "exit_status =") {
		t.Error("query missing ExitCode filter")
	}
	if len(args) != 1 {
		t.Errorf("got %d args, want 1", len(args))
	}
	if args[0] != 1 {
		t.Errorf("exit code arg = %v, want 1", args[0])
	}
}

func TestBuildAtuinQuery_BasicOptions(t *testing.T) {
	dm := NewDatabaseManager("", false)

	tests := []struct {
		name           string
		options        QueryOptions
		expectContains []string
		expectArgs     int
	}{
		{
			name:           "empty options",
			options:        QueryOptions{},
			expectContains: []string{"SELECT", "FROM history", "ORDER BY"},
			expectArgs:     0,
		},
		{
			name: "with pattern",
			options: QueryOptions{
				Pattern: "docker",
			},
			expectContains: []string{"command LIKE"},
			expectArgs:     1,
		},
		{
			name: "with directory",
			options: QueryOptions{
				Directory: "/home/user",
			},
			expectContains: []string{"cwd LIKE"},
			expectArgs:     1,
		},
		{
			name: "with session",
			options: QueryOptions{
				Session: "abc123",
			},
			expectContains: []string{"session LIKE"},
			expectArgs:     1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, args := dm.buildAtuinQuery(tt.options)

			for _, expected := range tt.expectContains {
				if !containsString(query, expected) {
					t.Errorf("query missing expected string %q: %s", expected, query)
				}
			}

			if len(args) != tt.expectArgs {
				t.Errorf("got %d args, want %d", len(args), tt.expectArgs)
			}
		})
	}
}

func TestBuildQuery_SchemaRouting(t *testing.T) {
	dm := NewDatabaseManager("", false)

	options := QueryOptions{Pattern: "test"}

	// Test zsh-histdb routing
	query, _ := dm.buildQuery(SchemaZshHistdb, options)
	if !containsString(query, "argv LIKE") {
		t.Error("zsh-histdb query should use 'argv' column")
	}

	// Test atuin routing
	query, _ = dm.buildQuery(SchemaAtuin, options)
	if !containsString(query, "command LIKE") {
		t.Error("atuin query should use 'command' column")
	}

	// Test unknown schema fallback
	query, _ = dm.buildQuery(SchemaUnknown, options)
	if !containsString(query, "SELECT 1") {
		t.Error("unknown schema should return fallback query")
	}
}

func TestQueryCommands_ZshHistdb(t *testing.T) {
	tmpDir := t.TempDir()

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}

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
		INSERT INTO sessions (id, session) VALUES (1, 'FRAAS-123');
		INSERT INTO commands (argv, start_time, duration, exit_status, place_id, session_id, hostname)
		VALUES ('git status', 1700000000, 100, 0, 1, 1, 'localhost');
		INSERT INTO commands (argv, start_time, duration, exit_status, place_id, session_id, hostname)
		VALUES ('git commit -m "test"', 1700000100, 200, 0, 1, 1, 'localhost');
		INSERT INTO commands (argv, start_time, duration, exit_status, place_id, session_id, hostname)
		VALUES ('make build', 1700000200, 5000, 1, 1, 1, 'localhost');
	`)
	if err != nil {
		t.Fatalf("Failed to setup test data: %v", err)
	}
	db.Close()

	dm := NewDatabaseManager(dbPath, false)

	// Test basic query
	commands, err := dm.QueryCommands(QueryOptions{})
	if err != nil {
		t.Fatalf("QueryCommands() error: %v", err)
	}
	if len(commands) != 3 {
		t.Errorf("QueryCommands() returned %d commands, want 3", len(commands))
	}

	// Test pattern filter
	commands, err = dm.QueryCommands(QueryOptions{Pattern: "git"})
	if err != nil {
		t.Fatalf("QueryCommands() error: %v", err)
	}
	if len(commands) != 2 {
		t.Errorf("QueryCommands() with pattern returned %d commands, want 2", len(commands))
	}

	// Test limit
	commands, err = dm.QueryCommands(QueryOptions{Limit: 1})
	if err != nil {
		t.Fatalf("QueryCommands() error: %v", err)
	}
	if len(commands) != 1 {
		t.Errorf("QueryCommands() with limit returned %d commands, want 1", len(commands))
	}
}

func TestQueryCommands_Atuin(t *testing.T) {
	tmpDir := t.TempDir()

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}

	// Create atuin schema with test data
	// Note: atuin uses nanoseconds for timestamp
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
	db.Close()

	dm := NewDatabaseManager(dbPath, false)

	commands, err := dm.QueryCommands(QueryOptions{})
	if err != nil {
		t.Fatalf("QueryCommands() error: %v", err)
	}
	if len(commands) != 2 {
		t.Errorf("QueryCommands() returned %d commands, want 2", len(commands))
	}

	// Verify command data was parsed correctly
	if commands[0].Command != "ls -la" {
		t.Errorf("First command = %q, want %q", commands[0].Command, "ls -la")
	}
	if commands[0].Directory != "/home/user" {
		t.Errorf("First command directory = %q, want %q", commands[0].Directory, "/home/user")
	}
}

func TestQueryCommands_EmptyResult(t *testing.T) {
	tmpDir := t.TempDir()

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}

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
	`)
	if err != nil {
		t.Fatalf("Failed to create schema: %v", err)
	}
	db.Close()

	dm := NewDatabaseManager(dbPath, false)

	commands, err := dm.QueryCommands(QueryOptions{})
	if err != nil {
		t.Fatalf("QueryCommands() error: %v", err)
	}
	if len(commands) != 0 {
		t.Errorf("QueryCommands() returned %d commands, want 0", len(commands))
	}
}

func TestQueryCommands_NotAvailable(t *testing.T) {
	dm := NewDatabaseManager("/nonexistent/db.sqlite", false)

	_, err := dm.QueryCommands(QueryOptions{})
	if err == nil {
		t.Error("QueryCommands() expected error for unavailable database")
	}
}

func TestGetDatabaseInfo_NonExistent(t *testing.T) {
	dm := NewDatabaseManager("/nonexistent/path/db.sqlite", false)

	info, err := dm.GetDatabaseInfo()
	if err != nil {
		t.Fatalf("GetDatabaseInfo() error: %v", err)
	}

	if info["exists"].(bool) {
		t.Error("expected exists=false for non-existent database")
	}
}

func TestGetDatabaseInfo_ValidDatabase(t *testing.T) {
	tmpDir := t.TempDir()

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}

	_, err = db.Exec(`
		CREATE TABLE history (
			id INTEGER PRIMARY KEY,
			command TEXT
		);
		INSERT INTO history (command) VALUES ('test1');
		INSERT INTO history (command) VALUES ('test2');
	`)
	if err != nil {
		t.Fatalf("Failed to setup test data: %v", err)
	}
	db.Close()

	dm := NewDatabaseManager(dbPath, false)

	info, err := dm.GetDatabaseInfo()
	if err != nil {
		t.Fatalf("GetDatabaseInfo() error: %v", err)
	}

	if !info["exists"].(bool) {
		t.Error("expected exists=true for existing database")
	}
	if info["schema"] != string(SchemaAtuin) {
		t.Errorf("schema = %v, want %v", info["schema"], SchemaAtuin)
	}
	if info["command_count"].(int64) != 2 {
		t.Errorf("command_count = %v, want 2", info["command_count"])
	}
}

// Helper function to check if a string contains a substring
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStringHelper(s, substr))
}

func containsStringHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
