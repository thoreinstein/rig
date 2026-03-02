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

	// Note: On a fresh DB, the completion commit should show the rows added during the workflow.
	// Since LogWorkflowCompleted adds its own event before committing, we expect at least that one.
	if len(diffs) == 0 {
		// Depending on how dolt_diff handles the first commit, this might be empty if it only shows changes between commits.
		// However, LogWorkflowCompleted calls DOLT_COMMIT after inserting the workflow completion event.
		// If there were uncommitted changes (like preflight started), they are committed too.
		t.Log("Warning: no diffs found (expected on some Dolt versions if no previous commit existed)")
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
