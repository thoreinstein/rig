package events

import (
	"path/filepath"
	"testing"
)

func TestReader_QueryEventsByTicket(t *testing.T) {
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
	ticketID := "PROJ-READER-123"
	logger.SetTicket(ticketID)

	// Log some events
	if err := logger.LogStepStarted(ctx, "wf1", "preflight"); err != nil {
		t.Fatalf("LogStepStarted failed: %v", err)
	}
	if err := logger.LogStepCompleted(ctx, "wf1", "preflight"); err != nil {
		t.Fatalf("LogStepCompleted failed: %v", err)
	}

	// Query by ticket
	events, err := dm.QueryEventsByTicket(ctx, ticketID)
	if err != nil {
		t.Fatalf("QueryEventsByTicket failed: %v", err)
	}

	if len(events) != 2 {
		t.Errorf("expected 2 events, got %d", len(events))
	}

	for _, e := range events {
		if e.CorrelationID != "wf1" {
			t.Errorf("expected correlation_id 'wf1', got %q", e.CorrelationID)
		}
	}

	// Query non-existent ticket
	events, err = dm.QueryEventsByTicket(ctx, "NON-EXISTENT")
	if err != nil {
		t.Fatalf("QueryEventsByTicket failed: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}
}

func TestReader_QueryDiffForCorrelation(t *testing.T) {
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
	correlationID := "diff-test-123"

	// 1. Log steps and complete workflow (creates commit)
	if err := logger.LogStepStarted(ctx, correlationID, "preflight"); err != nil {
		t.Fatalf("LogStepStarted failed: %v", err)
	}
	if err := logger.LogWorkflowCompleted(ctx, correlationID); err != nil {
		t.Fatalf("LogWorkflowCompleted failed: %v", err)
	}

	// 2. Query diffs
	diffs, err := dm.QueryDiffForCorrelation(ctx, correlationID)
	if err != nil {
		t.Fatalf("QueryDiffForCorrelation failed: %v", err)
	}

	// The completion commit includes all uncommitted rows (preflight + workflow completion).
	// On Dolt's first commit, dolt_diff compares against an empty base, so all inserts appear.
	if len(diffs) == 0 {
		t.Error("expected at least one diff entry from the workflow completion commit")
	} else {
		foundCompletion := false
		for _, d := range diffs {
			if d.Step == "workflow" && d.Status == "COMPLETED" {
				foundCompletion = true
			}
		}
		if !foundCompletion {
			t.Errorf("expected workflow completion event in diffs")
		}
	}
}

func TestReader_QueryDiffForCorrelation_NoCommit(t *testing.T) {
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

	// Query with a correlation ID that has no matching commit
	diffs, err := dm.QueryDiffForCorrelation(ctx, "nonexistent-correlation")
	if err != nil {
		t.Fatalf("QueryDiffForCorrelation should return nil error for missing commit, got: %v", err)
	}
	if diffs != nil {
		t.Errorf("expected nil diffs for missing commit, got %d entries", len(diffs))
	}
}
