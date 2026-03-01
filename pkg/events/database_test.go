package events

import (
	"path/filepath"
	"testing"
)

func TestDatabaseManager(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()

	dataPath := filepath.Join(tmpDir, "data")
	dm, err := NewDatabaseManager(dataPath, "Test User", "test@example.com", true)
	if err != nil {
		t.Fatalf("NewDatabaseManager failed: %v", err)
	}
	defer dm.Close()

	if err := dm.InitDatabase(); err != nil {
		t.Fatalf("InitDatabase failed: %v", err)
	}

	// Verify table exists
	var tableName string
	err = dm.db.QueryRow("SELECT table_name FROM information_schema.tables WHERE table_schema = 'rig_events' AND table_name = 'workflow_events'").Scan(&tableName)
	if err != nil {
		t.Fatalf("failed to find table workflow_events: %v", err)
	}
	if tableName != "workflow_events" {
		t.Errorf("expected table workflow_events, got %s", tableName)
	}
}
