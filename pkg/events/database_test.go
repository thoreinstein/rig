package events

import (
	"encoding/json"
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

func TestBackfillTicket(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	dataPath := filepath.Join(tmpDir, "data")
	dm, err := NewDatabaseManager(dataPath, "Rig Bot", "rig@localhost", true)
	if err != nil {
		t.Fatalf("failed to create db manager: %v", err)
	}
	defer dm.Close()

	if err := dm.InitDatabase(); err != nil {
		t.Fatalf("failed to init db: %v", err)
	}

	ctx := t.Context()
	correlationID := "test-backfill-123"

	// Insert row without metadata
	_, err = dm.db.ExecContext(ctx, "INSERT INTO workflow_events (id, correlation_id, step, status, message) VALUES ('id1', ?, 'preflight', 'STARTED', '')", correlationID)
	if err != nil {
		t.Fatalf("failed to insert initial event: %v", err)
	}

	// Backfill ticket
	ticketID := "PROJ-123"
	if err := dm.BackfillTicket(ctx, correlationID, ticketID); err != nil {
		t.Fatalf("BackfillTicket failed: %v", err)
	}

	// Verify metadata
	var metadataStr string
	err = dm.db.QueryRow("SELECT metadata FROM workflow_events WHERE correlation_id = ?", correlationID).Scan(&metadataStr)
	if err != nil {
		t.Fatalf("failed to query metadata: %v", err)
	}

	var metadata map[string]string
	if err := json.Unmarshal([]byte(metadataStr), &metadata); err != nil {
		t.Fatalf("failed to unmarshal metadata: %v", err)
	}

	if metadata["ticket"] != ticketID {
		t.Errorf("expected ticket %q, got %q", ticketID, metadata["ticket"])
	}
}

func TestBackfillTicket_Idempotent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	dataPath := filepath.Join(tmpDir, "data")
	dm, err := NewDatabaseManager(dataPath, "Rig Bot", "rig@localhost", true)
	if err != nil {
		t.Fatalf("failed to create db manager: %v", err)
	}
	defer dm.Close()

	if err := dm.InitDatabase(); err != nil {
		t.Fatalf("failed to init db: %v", err)
	}

	ctx := t.Context()
	correlationID := "test-backfill-idempotent"

	// Insert row with existing metadata (simulates a previously tagged event)
	existingMeta := `{"ticket":"ORIG-999","custom":"data"}`
	_, err = dm.db.ExecContext(ctx,
		"INSERT INTO workflow_events (id, correlation_id, step, status, message, metadata) VALUES ('id-idem', ?, 'preflight', 'STARTED', '', ?)",
		correlationID, existingMeta)
	if err != nil {
		t.Fatalf("failed to insert event: %v", err)
	}

	// Backfill should NOT overwrite existing metadata (WHERE metadata IS NULL)
	if err := dm.BackfillTicket(ctx, correlationID, "NEW-456"); err != nil {
		t.Fatalf("BackfillTicket failed: %v", err)
	}

	// Verify original metadata is preserved
	var metadataStr string
	err = dm.db.QueryRow("SELECT metadata FROM workflow_events WHERE id = 'id-idem'").Scan(&metadataStr)
	if err != nil {
		t.Fatalf("failed to query metadata: %v", err)
	}

	var metadata map[string]string
	if err := json.Unmarshal([]byte(metadataStr), &metadata); err != nil {
		t.Fatalf("failed to unmarshal metadata: %v", err)
	}

	if metadata["ticket"] != "ORIG-999" {
		t.Errorf("backfill overwrote existing metadata: ticket = %q, want %q", metadata["ticket"], "ORIG-999")
	}
	if metadata["custom"] != "data" {
		t.Errorf("backfill lost custom metadata: custom = %q, want %q", metadata["custom"], "data")
	}
}
