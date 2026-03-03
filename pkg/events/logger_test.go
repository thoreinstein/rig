package events

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestDoltEventLogger(t *testing.T) {
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

	logger := NewDoltEventLogger(dm)
	ctx := t.Context()
	correlationID := "test-workflow-123"

	// Test logging steps
	if err := logger.LogStepStarted(ctx, correlationID, "preflight"); err != nil {
		t.Errorf("LogStepStarted failed: %v", err)
	}
	if err := logger.LogStepCompleted(ctx, correlationID, "preflight"); err != nil {
		t.Errorf("LogStepCompleted failed: %v", err)
	}
	if err := logger.LogStepFailed(ctx, correlationID, "merge", "conflict"); err != nil {
		t.Errorf("LogStepFailed failed: %v", err)
	}

	// Verify rows in database
	var count int
	err = dm.db.QueryRow("SELECT COUNT(*) FROM workflow_events WHERE correlation_id = ?", correlationID).Scan(&count)
	if err != nil {
		t.Fatalf("failed to query events: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 events, got %d", count)
	}

	// Test workflow completion (performs Dolt commit)
	if err := logger.LogWorkflowCompleted(ctx, correlationID); err != nil {
		t.Fatalf("LogWorkflowCompleted failed: %v", err)
	}

	// Verify commit exists in dolt_log
	var commitMsg string
	err = dm.db.QueryRow("SELECT message FROM dolt_log ORDER BY date DESC LIMIT 1").Scan(&commitMsg)
	if err != nil {
		t.Fatalf("failed to query dolt_log: %v", err)
	}
	expectedMsg := "events: Workflow " + correlationID + " completed"
	if commitMsg != expectedMsg {
		t.Errorf("expected commit message %q, got %q", expectedMsg, commitMsg)
	}
}

func TestDoltEventLogger_MetadataTicket(t *testing.T) {
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

	logger := NewDoltEventLogger(dm)
	ctx := t.Context()
	correlationID := "test-metadata-123"
	ticketID := "PROJ-123"

	// Set ticket and log
	logger.SetTicket(ticketID)
	if err := logger.LogStepStarted(ctx, correlationID, "preflight"); err != nil {
		t.Fatalf("LogStepStarted failed: %v", err)
	}

	// Verify metadata in database
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

func TestDoltEventLogger_NoTicketNullMetadata(t *testing.T) {
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

	logger := NewDoltEventLogger(dm)
	ctx := t.Context()
	correlationID := "test-no-ticket-123"

	// Log without calling SetTicket — metadata should be NULL
	if err := logger.LogStepStarted(ctx, correlationID, "preflight"); err != nil {
		t.Fatalf("LogStepStarted failed: %v", err)
	}

	var metadata *string
	err = dm.db.QueryRow("SELECT metadata FROM workflow_events WHERE correlation_id = ?", correlationID).Scan(&metadata)
	if err != nil {
		t.Fatalf("failed to query metadata: %v", err)
	}

	if metadata != nil {
		t.Errorf("expected NULL metadata when no ticket set, got %q", *metadata)
	}
}

func TestNoopEventLogger(t *testing.T) {
	logger := NoopEventLogger{}
	ctx := t.Context()
	cid := "any"

	if err := logger.LogStepStarted(ctx, cid, "step"); err != nil {
		t.Errorf("LogStepStarted failed: %v", err)
	}
	if err := logger.LogStepCompleted(ctx, cid, "step"); err != nil {
		t.Errorf("LogStepCompleted failed: %v", err)
	}
	if err := logger.LogStepFailed(ctx, cid, "step", "err"); err != nil {
		t.Errorf("LogStepFailed failed: %v", err)
	}
	if err := logger.LogWorkflowCompleted(ctx, cid); err != nil {
		t.Errorf("LogWorkflowCompleted failed: %v", err)
	}
	if err := logger.LogWorkflowFailed(ctx, cid, "error"); err != nil {
		t.Errorf("LogWorkflowFailed failed: %v", err)
	}
	if err := logger.CommitMilestone(ctx, "milestone message"); err != nil {
		t.Errorf("CommitMilestone failed: %v", err)
	}
	if err := logger.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
}
