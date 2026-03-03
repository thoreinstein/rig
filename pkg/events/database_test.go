package events

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
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

func TestPruneEvents(t *testing.T) {
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
	now := time.Now()
	oldTime := now.Add(-48 * time.Hour)
	cutoff := now.Add(-24 * time.Hour)

	// 1. Insert 3 rows: 2 old, 1 new
	events := []struct {
		id string
		ts time.Time
	}{
		{"id1", oldTime},
		{"id2", oldTime.Add(time.Hour)},
		{"id3", now},
	}

	for _, e := range events {
		_, err := dm.db.ExecContext(ctx, "INSERT INTO workflow_events (id, correlation_id, step, status, created_at) VALUES (?, 'corr', 'step', 'COMPLETED', ?)", e.id, e.ts)
		if err != nil {
			t.Fatalf("failed to insert event %s: %v", e.id, err)
		}
	}

	// 2. Test Dry Run (should find 2 events)
	res, err := dm.PruneEvents(ctx, cutoff, true)
	if err != nil {
		t.Fatalf("PruneEvents(dryRun=true) failed: %v", err)
	}
	if res.EventsDeleted != 2 {
		t.Errorf("Dry run: expected 2 events, got %d", res.EventsDeleted)
	}

	// Verify no rows actually deleted
	var count int
	if err := dm.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM workflow_events").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Errorf("Dry run actually deleted rows: count = %d, want 3", count)
	}

	// 3. Perform actual Prune
	res, err = dm.PruneEvents(ctx, cutoff, false)
	if err != nil {
		t.Fatalf("PruneEvents(dryRun=false) failed: %v", err)
	}
	if res.EventsDeleted != 2 {
		t.Errorf("Actual prune: expected 2 events, got %d", res.EventsDeleted)
	}

	// Verify rows deleted
	if err := dm.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM workflow_events").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("Actual prune failed: count = %d, want 1", count)
	}

	// Verify correct row remains
	var remainingID string
	if err := dm.db.QueryRowContext(ctx, "SELECT id FROM workflow_events").Scan(&remainingID); err != nil {
		t.Fatal(err)
	}
	if remainingID != "id3" {
		t.Errorf("Wrong row remains: id = %s, want id3", remainingID)
	}
}

func TestDoltGC(t *testing.T) {
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
	if err := dm.DoltGC(ctx); err != nil {
		t.Errorf("DoltGC failed: %v", err)
	}
}
