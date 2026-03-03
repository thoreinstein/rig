package events

import (
	"path/filepath"
	"testing"
	"time"
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
	events, err := dm.QueryEventsByTicket(ctx, ticketID, time.Time{}, time.Time{})
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
	events, err = dm.QueryEventsByTicket(ctx, "NON-EXISTENT", time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("QueryEventsByTicket failed: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}
}

func TestReader_QueryEventsByTicketAndTimeRange(t *testing.T) {
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
	ticketID := "PROJ-RANGE-123"

	// Log events across a time window
	t1 := time.Now().Add(-10 * time.Minute)
	t2 := time.Now().Add(-5 * time.Minute)
	t3 := time.Now()

	// Use internal DB connection directly to inject specific timestamps
	metadata := `{"ticket":"` + ticketID + `"}`
	_, err = dm.db.ExecContext(ctx, "INSERT INTO workflow_events (id, correlation_id, step, status, message, metadata, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		"ev1", "corr1", "step1", "STARTED", "msg1", metadata, t1)
	if err != nil {
		t.Fatalf("failed to insert t1 event: %v", err)
	}
	_, err = dm.db.ExecContext(ctx, "INSERT INTO workflow_events (id, correlation_id, step, status, message, metadata, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		"ev2", "corr1", "step1", "COMPLETED", "msg2", metadata, t2)
	if err != nil {
		t.Fatalf("failed to insert t2 event: %v", err)
	}
	_, err = dm.db.ExecContext(ctx, "INSERT INTO workflow_events (id, correlation_id, step, status, message, metadata, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		"ev3", "corr1", "step2", "STARTED", "msg3", metadata, t3)
	if err != nil {
		t.Fatalf("failed to insert t3 event: %v", err)
	}

	// 1. Query ticket covering all
	events, err := dm.QueryEventsByTicket(ctx, ticketID, time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("QueryEventsByTicket failed: %v", err)
	}
	if len(events) != 3 {
		t.Errorf("expected 3 events, got %d", len(events))
	}

	// 2. Query ticket with range covering middle only
	events, err = dm.QueryEventsByTicket(ctx, ticketID, t2.Add(-1*time.Minute), t2.Add(1*time.Minute))
	if err != nil {
		t.Fatalf("QueryEventsByTicket failed: %v", err)
	}
	if len(events) != 1 {
		t.Errorf("expected 1 event, got %d", len(events))
	} else if events[0].ID != "ev2" {
		t.Errorf("expected ev2, got %s", events[0].ID)
	}

	// 3. Query ticket with range covering none
	events, err = dm.QueryEventsByTicket(ctx, ticketID, t1.Add(-10*time.Minute), t1.Add(-5*time.Minute))
	if err != nil {
		t.Fatalf("QueryEventsByTicket failed: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}
}

func TestReader_QueryEventsByTimeRange(t *testing.T) {
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

	// Log events across a time window
	t1 := time.Now().Add(-10 * time.Minute)
	t2 := time.Now().Add(-5 * time.Minute)
	t3 := time.Now()

	// Use internal DB connection directly to inject specific timestamps
	_, err = dm.db.ExecContext(ctx, "INSERT INTO workflow_events (id, correlation_id, step, status, message, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		"ev1", "corr1", "step1", "STARTED", "msg1", t1)
	if err != nil {
		t.Fatalf("failed to insert t1 event: %v", err)
	}
	_, err = dm.db.ExecContext(ctx, "INSERT INTO workflow_events (id, correlation_id, step, status, message, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		"ev2", "corr1", "step1", "COMPLETED", "msg2", t2)
	if err != nil {
		t.Fatalf("failed to insert t2 event: %v", err)
	}
	_, err = dm.db.ExecContext(ctx, "INSERT INTO workflow_events (id, correlation_id, step, status, message, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		"ev3", "corr1", "step2", "STARTED", "msg3", t3)
	if err != nil {
		t.Fatalf("failed to insert t3 event: %v", err)
	}

	// 1. Query range covering all
	events, err := dm.QueryEventsByTimeRange(ctx, t1.Add(-1*time.Minute), t3.Add(1*time.Minute))
	if err != nil {
		t.Fatalf("QueryEventsByTimeRange failed: %v", err)
	}
	if len(events) != 3 {
		t.Errorf("expected 3 events, got %d", len(events))
	}

	// 2. Query range covering middle only
	events, err = dm.QueryEventsByTimeRange(ctx, t2.Add(-1*time.Minute), t2.Add(1*time.Minute))
	if err != nil {
		t.Fatalf("QueryEventsByTimeRange failed: %v", err)
	}
	if len(events) != 1 {
		t.Errorf("expected 1 event, got %d", len(events))
	} else if events[0].ID != "ev2" {
		t.Errorf("expected ev2, got %s", events[0].ID)
	}

	// 3. Query range covering none
	events, err = dm.QueryEventsByTimeRange(ctx, t1.Add(-10*time.Minute), t1.Add(-5*time.Minute))
	if err != nil {
		t.Fatalf("QueryEventsByTimeRange failed: %v", err)
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
