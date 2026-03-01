package events

import (
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

	if err := dm.InitDatabase(dataPath); err != nil {
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
	expectedMsg := "Workflow " + correlationID + " completed"
	if commitMsg != expectedMsg {
		t.Errorf("expected commit message %q, got %q", expectedMsg, commitMsg)
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
	if err := logger.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
}
