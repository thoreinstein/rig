package history

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestQueryCommands_WithNewFilters(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
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
		CREATE TABLE places (id INTEGER PRIMARY KEY, dir TEXT);
		CREATE TABLE sessions (id INTEGER PRIMARY KEY, session TEXT);

		INSERT INTO places (id, dir) VALUES (1, '/home');
		INSERT INTO sessions (id, session) VALUES (1, 'session1');
		INSERT INTO sessions (id, session) VALUES (2, 'session2');

		-- Cmd 1: Short duration, session1
		INSERT INTO commands (argv, start_time, duration, exit_status, place_id, session_id)
		VALUES ('ls', 100, 1, 0, 1, 1);

		-- Cmd 2: Long duration (5s), session1
		INSERT INTO commands (argv, start_time, duration, exit_status, place_id, session_id)
		VALUES ('sleep 5', 200, 5, 0, 1, 1);

		-- Cmd 3: Short duration, session2
		INSERT INTO commands (argv, start_time, duration, exit_status, place_id, session_id)
		VALUES ('echo hi', 300, 1, 0, 1, 2);
	`)
	if err != nil {
		t.Fatalf("Failed to setup test data: %v", err)
	}

	dm := NewDatabaseManager(dbPath, false)

	// Test MinDuration
	cmds, err := dm.QueryCommands(QueryOptions{MinDuration: 2 * time.Second})
	if err != nil {
		t.Fatalf("QueryCommands error: %v", err)
	}
	if len(cmds) != 1 {
		t.Errorf("Expected 1 command with duration >= 2s, got %d", len(cmds))
	}
	if len(cmds) > 0 && cmds[0].Command != "sleep 5" {
		t.Errorf("Expected 'sleep 5', got %q", cmds[0].Command)
	}

	// Test SessionID
	cmds, err = dm.QueryCommands(QueryOptions{SessionID: "session2"})
	if err != nil {
		t.Fatalf("QueryCommands error: %v", err)
	}
	if len(cmds) != 1 {
		t.Errorf("Expected 1 command in session2, got %d", len(cmds))
	}
	if len(cmds) > 0 && cmds[0].Command != "echo hi" {
		t.Errorf("Expected 'echo hi', got %q", cmds[0].Command)
	}
}
