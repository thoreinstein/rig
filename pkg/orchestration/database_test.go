package orchestration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func setupTestDB(t *testing.T) *DatabaseManager {
	// Clear GIT_* env vars to avoid interference from pre-commit/environment
	for _, env := range os.Environ() {
		if i := strings.Index(env, "="); i > 0 && len(env) > 4 && env[:4] == "GIT_" {
			t.Setenv(env[:i], "")
		}
	}

	tmpDir := t.TempDir()
	dm, err := NewDatabaseManager(tmpDir, "Test User", "test@localhost", true)
	if err != nil {
		t.Fatalf("Failed to create test database manager: %v", err)
	}

	if err := dm.InitDatabase(); err != nil {
		t.Fatalf("Failed to initialize test database: %v", err)
	}

	return dm
}

func TestInitDatabase(t *testing.T) {
	dm := setupTestDB(t)
	defer dm.Close()
	// setupTestDB already calls InitDatabase and fails on error
}

func TestWorkflowCRUD(t *testing.T) {
	dm := setupTestDB(t)
	defer dm.Close()
	ctx := t.Context()

	workflowName := "test-workflow-" + uuid.New().String()
	w := &Workflow{
		Name:        workflowName,
		Description: "Test Description",
		Version:     1,
		Status:      WorkflowStatusDraft,
	}

	// Test Create
	if err := dm.CreateWorkflow(ctx, w); err != nil {
		t.Fatalf("CreateWorkflow() failed: %v", err)
	}

	// Test Get
	retrieved, err := dm.GetWorkflow(ctx, w.ID)
	if err != nil {
		t.Fatalf("GetWorkflow() failed: %v", err)
	}
	if retrieved.Name != w.Name {
		t.Errorf("Retrieved name = %q, want %q", retrieved.Name, w.Name)
	}

	// Test Update
	retrieved.Description = "Updated Description"
	if err := dm.UpdateWorkflow(ctx, retrieved); err != nil {
		t.Fatalf("UpdateWorkflow() failed: %v", err)
	}

	updated, err := dm.GetWorkflow(ctx, w.ID)
	if err != nil {
		t.Fatalf("GetWorkflow() after update failed: %v", err)
	}
	if updated.Version != 2 {
		t.Errorf("Updated version = %d, want 2", updated.Version)
	}

	// Test List
	workflows, err := dm.ListWorkflows(ctx)
	if err != nil {
		t.Fatalf("ListWorkflows() failed: %v", err)
	}
	found := false
	for _, wf := range workflows {
		if wf.ID == w.ID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Workflow %s not found in list", w.ID)
	}
}

func TestSaveWorkflowDefinition(t *testing.T) {
	dm := setupTestDB(t)
	defer dm.Close()
	ctx := t.Context()

	w := &Workflow{
		Name:   "def-test-" + uuid.New().String(),
		Status: WorkflowStatusActive,
	}

	nodes := []*Node{
		{Name: "node1", Type: "task"},
		{Name: "node2", Type: "task"},
	}

	if err := dm.SaveWorkflowDefinition(ctx, w, nodes, nil); err != nil {
		t.Fatalf("SaveWorkflowDefinition() failed: %v", err)
	}

	// Verify nodes for version 1
	dbNodes, err := dm.GetNodesByWorkflow(ctx, w.ID, 1)
	if err != nil {
		t.Fatalf("GetNodesByWorkflow() failed: %v", err)
	}
	if len(dbNodes) != 2 {
		t.Errorf("Got %d nodes, want 2", len(dbNodes))
	}

	// Test Dolt Log
	log, err := dm.DoltLog(ctx, 5)
	if err != nil {
		t.Fatalf("DoltLog() failed: %v", err)
	}
	if len(log) == 0 {
		t.Error("Dolt log is empty, expected commits")
	}
}

func TestExecutionLifecycle(t *testing.T) {
	dm := setupTestDB(t)
	defer dm.Close()
	ctx := t.Context()

	// Need a workflow first
	w := &Workflow{Name: "exec-test-" + uuid.New().String()}
	if err := dm.CreateWorkflow(ctx, w); err != nil {
		t.Fatalf("CreateWorkflow() failed: %v", err)
	}

	exec := &Execution{
		WorkflowID:      w.ID,
		WorkflowVersion: w.Version,
	}

	if err := dm.CreateExecution(ctx, exec); err != nil {
		t.Fatalf("CreateExecution() failed: %v", err)
	}

	if err := dm.UpdateExecutionStatus(ctx, exec.ID, ExecutionStatusRunning, ""); err != nil {
		t.Fatalf("UpdateExecutionStatus(RUNNING) failed: %v", err)
	}

	updated, err := dm.GetExecution(ctx, exec.ID)
	if err != nil {
		t.Fatalf("GetExecution() failed: %v", err)
	}
	if updated.Status != ExecutionStatusRunning || updated.StartedAt == nil {
		t.Errorf("Execution not properly transitioned to RUNNING")
	}

	if err := dm.UpdateExecutionStatus(ctx, exec.ID, ExecutionStatusSuccess, ""); err != nil {
		t.Fatalf("UpdateExecutionStatus(SUCCESS) failed: %v", err)
	}

	updated, err = dm.GetExecution(ctx, exec.ID)
	if err != nil {
		t.Fatalf("GetExecution() failed: %v", err)
	}
	if updated.Status != ExecutionStatusSuccess || updated.CompletedAt == nil {
		t.Errorf("Execution not properly transitioned to SUCCESS")
	}
}

func TestBackwardCompatibilityGuard(t *testing.T) {
	dm := setupTestDB(t)
	defer dm.Close()
	ctx := t.Context()

	w := &Workflow{Name: "guard-test-" + uuid.New().String()}
	if err := dm.CreateWorkflow(ctx, w); err != nil {
		t.Fatalf("CreateWorkflow() failed: %v", err)
	}

	// Create an active execution
	exec := &Execution{
		WorkflowID:      w.ID,
		WorkflowVersion: w.Version,
		Status:          ExecutionStatusPending,
	}
	if err := dm.CreateExecution(ctx, exec); err != nil {
		t.Fatalf("CreateExecution() failed: %v", err)
	}
	// Force it to RUNNING since CreateExecution defaults to PENDING (which is also active)
	if err := dm.UpdateExecutionStatus(ctx, exec.ID, ExecutionStatusRunning, ""); err != nil {
		t.Fatalf("UpdateExecutionStatus(RUNNING) failed: %v", err)
	}

	// Try to update workflow
	err := dm.UpdateWorkflow(ctx, w)
	if err == nil {
		t.Error("UpdateWorkflow should have failed due to active execution")
	}

	// Try to save definition
	err = dm.SaveWorkflowDefinition(ctx, w, nil, nil)
	if err == nil {
		t.Error("SaveWorkflowDefinition should have failed due to active execution")
	}
}

func TestWorkflowUpdateMerging(t *testing.T) {
	dm := setupTestDB(t)
	defer dm.Close()
	ctx := t.Context()

	w := &Workflow{
		Name:   "merge-test-" + uuid.New().String(),
		Status: WorkflowStatusActive,
	}
	if err := dm.CreateWorkflow(ctx, w); err != nil {
		t.Fatalf("CreateWorkflow failed: %v", err)
	}

	// 1. Update with zero status and stale version
	update := &Workflow{
		ID:          w.ID,
		Name:        w.Name,
		Description: "Updated description",
		Version:     0,  // Stale/zero version
		Status:      "", // Zero status (should merge)
	}

	if err := dm.UpdateWorkflow(ctx, update); err != nil {
		t.Fatalf("UpdateWorkflow failed: %v", err)
	}

	updated, err := dm.GetWorkflow(ctx, w.ID)
	if err != nil {
		t.Fatalf("GetWorkflow failed: %v", err)
	}

	if updated.Version != 2 {
		t.Errorf("Expected version 2, got %d", updated.Version)
	}
	if updated.Status != WorkflowStatusActive {
		t.Errorf("Expected status %s, got %s", WorkflowStatusActive, updated.Status)
	}
	if updated.Description != "Updated description" {
		t.Errorf("Expected updated description, got %s", updated.Description)
	}
}

func TestWorkflowConcurrency(t *testing.T) {
	t.Skip("skipping known concurrency issue in embedded dolt - to be addressed in rig-ytn.7")
}

func TestNodeHistoricalVersioning(t *testing.T) {
	dm := setupTestDB(t)
	defer dm.Close()
	ctx := t.Context()

	w := &Workflow{
		Name:   "history-test-" + uuid.New().String(),
		Status: WorkflowStatusActive,
	}

	// Version 1
	nodes1 := []*Node{{Name: "node-v1", Type: "task"}}
	if err := dm.SaveWorkflowDefinition(ctx, w, nodes1, nil); err != nil {
		t.Fatalf("SaveWorkflowDefinition V1 failed: %v", err)
	}

	// Version 2
	nodes2 := []*Node{{Name: "node-v2", Type: "task"}}
	if err := dm.SaveWorkflowDefinition(ctx, w, nodes2, nil); err != nil {
		t.Fatalf("SaveWorkflowDefinition V2 failed: %v", err)
	}

	// Verify Version 1 still has its node
	dbNodes1, err := dm.GetNodesByWorkflow(ctx, w.ID, 1)
	if err != nil {
		t.Fatalf("GetNodesByWorkflow V1 failed: %v", err)
	}
	if len(dbNodes1) != 1 || dbNodes1[0].Name != "node-v1" {
		t.Errorf("Expected node-v1 for version 1, got %v", dbNodes1)
	}

	// Verify Version 2 has its node
	dbNodes2, err := dm.GetNodesByWorkflow(ctx, w.ID, 2)
	if err != nil {
		t.Fatalf("GetNodesByWorkflow V2 failed: %v", err)
	}
	if len(dbNodes2) != 1 || dbNodes2[0].Name != "node-v2" {
		t.Errorf("Expected node-v2 for version 2, got %v", dbNodes2)
	}
}

func TestIdempotentRecovery(t *testing.T) {
	dm := setupTestDB(t)
	defer dm.Close()
	ctx := t.Context()

	w := &Workflow{Name: "recovery-test-" + uuid.New().String()}
	if err := dm.CreateWorkflow(ctx, w); err != nil {
		t.Fatalf("CreateWorkflow failed: %v", err)
	}

	exec := &Execution{
		WorkflowID:      w.ID,
		WorkflowVersion: w.Version,
		Status:          ExecutionStatusPending,
	}
	if err := dm.CreateExecution(ctx, exec); err != nil {
		t.Fatalf("CreateExecution failed: %v", err)
	}

	// 1. Initial Transition to RUNNING
	if err := dm.UpdateExecutionStatus(ctx, exec.ID, ExecutionStatusRunning, ""); err != nil {
		t.Fatalf("First UpdateExecutionStatus(RUNNING) failed: %v", err)
	}

	initial, err := dm.GetExecution(ctx, exec.ID)
	if err != nil {
		t.Fatalf("GetExecution failed: %v", err)
	}
	if initial.StartedAt == nil {
		t.Fatal("StartedAt should be set")
	}
	firstStart := *initial.StartedAt

	// 2. Idempotent Transition to RUNNING
	if err := dm.UpdateExecutionStatus(ctx, exec.ID, ExecutionStatusRunning, ""); err != nil {
		t.Fatalf("Second UpdateExecutionStatus(RUNNING) failed: %v", err)
	}

	recovered, err := dm.GetExecution(ctx, exec.ID)
	if err != nil {
		t.Fatalf("GetExecution failed: %v", err)
	}
	if recovered.StartedAt == nil || !recovered.StartedAt.Equal(firstStart) {
		t.Errorf("StartedAt was not preserved: got %v, want %v", recovered.StartedAt, firstStart)
	}

	// 3. Node Idempotent Transition
	node := &Node{
		WorkflowID:      w.ID,
		WorkflowVersion: w.Version,
		Name:            "idempotent-test-node",
		Type:            "task",
	}
	if err := dm.CreateNode(ctx, node); err != nil {
		t.Fatalf("CreateNode failed: %v", err)
	}

	execWithStates, err := dm.CreateExecutionWithInitialStates(ctx, w.ID, w.Version)
	if err != nil {
		t.Fatalf("CreateExecutionWithInitialStates failed: %v", err)
	}
	states, err := dm.GetNodeStatesByExecution(ctx, execWithStates.ID)
	if err != nil {
		t.Fatalf("GetNodeStatesByExecution failed: %v", err)
	}
	ns := states[0]

	if err := dm.UpdateNodeStatus(ctx, ns.ID, NodeStatusRunning, nil, ""); err != nil {
		t.Fatalf("First UpdateNodeStatus(RUNNING) failed: %v", err)
	}

	nsInitial, err := dm.GetNodeStatesByExecution(ctx, execWithStates.ID)
	if err != nil {
		t.Fatalf("GetNodeStatesByExecution failed: %v", err)
	}
	if nsInitial[0].StartedAt == nil {
		t.Fatal("Expected StartedAt to be set after first RUNNING transition, got nil")
	}
	nsFirstStart := *nsInitial[0].StartedAt

	if err := dm.UpdateNodeStatus(ctx, ns.ID, NodeStatusRunning, nil, ""); err != nil {
		t.Fatalf("Second UpdateNodeStatus(RUNNING) failed: %v", err)
	}

	nsRecovered, err := dm.GetNodeStatesByExecution(ctx, execWithStates.ID)
	if err != nil {
		t.Fatalf("GetNodeStatesByExecution failed: %v", err)
	}
	if nsRecovered[0].StartedAt == nil || !nsRecovered[0].StartedAt.Equal(nsFirstStart) {
		t.Errorf("Node StartedAt was not preserved: got %v, want %v", nsRecovered[0].StartedAt, nsFirstStart)
	}
}

func TestMigrationIdempotency(t *testing.T) {
	dm := setupTestDB(t)
	defer dm.Close()
	ctx := t.Context()

	// Running Migrate again should be a no-op
	if err := dm.Migrate(ctx); err != nil {
		t.Fatalf("Migrate() failed on second run: %v", err)
	}

	// Derive expected version from defined migrations
	migrations := AllMigrations()
	if len(migrations) == 0 {
		t.Fatal("No migrations defined; cannot determine expected schema version")
	}
	expectedVersion := migrations[len(migrations)-1].Version

	var currentVersion int
	err := dm.db.QueryRowContext(ctx, "SELECT MAX(version) FROM schema_migrations").Scan(&currentVersion)
	if err != nil {
		t.Fatalf("Failed to query migration version: %v", err)
	}
	if currentVersion != expectedVersion {
		t.Errorf("Expected migration version %d, got %d", expectedVersion, currentVersion)
	}
}

func TestPruneExecutions(t *testing.T) {
	dm := setupTestDB(t)
	defer dm.Close()
	ctx := t.Context()

	// 1. Setup workflows and nodes
	w := &Workflow{Name: "prune-test"}
	if err := dm.CreateWorkflow(ctx, w); err != nil {
		t.Fatal(err)
	}
	node1 := &Node{WorkflowID: w.ID, WorkflowVersion: 1, Name: "n1", Type: "task"}
	if err := dm.CreateNode(ctx, node1); err != nil {
		t.Fatal(err)
	}
	node2 := &Node{WorkflowID: w.ID, WorkflowVersion: 1, Name: "n2", Type: "task"}
	if err := dm.CreateNode(ctx, node2); err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	oldTime := now.Add(-48 * time.Hour)
	cutoff := now.Add(-24 * time.Hour)

	// 2. Create executions with various statuses and timestamps
	execs := []struct {
		id     string
		status ExecutionStatus
		ts     time.Time
	}{
		{"old-success", ExecutionStatusSuccess, oldTime},
		{"old-failed", ExecutionStatusFailed, oldTime},
		{"old-running", ExecutionStatusRunning, oldTime}, // Should NOT be pruned (not terminal)
		{"new-success", ExecutionStatusSuccess, now},     // Should NOT be pruned (too recent)
	}

	for _, ex := range execs {
		query := `INSERT INTO executions (id, workflow_id, workflow_version, status, created_at) VALUES (?, ?, ?, ?, ?)`
		if _, err := dm.db.ExecContext(ctx, query, ex.id, w.ID, w.Version, ex.status, ex.ts); err != nil {
			t.Fatalf("failed to insert execution %s: %v", ex.id, err)
		}

		// Insert 2 node states per execution
		for _, n := range []*Node{node1, node2} {
			nsID := uuid.New().String()
			nsQuery := `INSERT INTO node_states (id, execution_id, node_id, status, created_at) VALUES (?, ?, ?, ?, ?)`
			if _, err := dm.db.ExecContext(ctx, nsQuery, nsID, ex.id, n.ID, NodeStatusSuccess, ex.ts); err != nil {
				t.Fatalf("failed to insert node state for %s: %v", ex.id, err)
			}
		}
	}

	// 3. Test Dry Run (should find 2 executions, 4 node states)
	res, err := dm.PruneExecutions(ctx, cutoff, true)
	if err != nil {
		t.Fatalf("PruneExecutions(dryRun=true) failed: %v", err)
	}
	if res.ExecutionsPruned != 2 {
		t.Errorf("Dry run: expected 2 executions, got %d", res.ExecutionsPruned)
	}
	if res.NodeStatesPruned != 4 {
		t.Errorf("Dry run: expected 4 node states, got %d", res.NodeStatesPruned)
	}

	// Verify nothing deleted
	var count int
	if err := dm.db.QueryRow("SELECT COUNT(*) FROM executions").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 4 {
		t.Errorf("Dry run deleted executions: count = %d, want 4", count)
	}

	// 4. Perform actual Prune
	res, err = dm.PruneExecutions(ctx, cutoff, false)
	if err != nil {
		t.Fatalf("PruneExecutions(dryRun=false) failed: %v", err)
	}
	if res.ExecutionsPruned != 2 {
		t.Errorf("Actual prune: expected 2 executions, got %d", res.ExecutionsPruned)
	}

	// Verify results
	if err := dm.db.QueryRow("SELECT COUNT(*) FROM executions").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("Actual prune failed: executions count = %d, want 2", count)
	}

	if err := dm.db.QueryRow("SELECT COUNT(*) FROM node_states").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 4 {
		t.Errorf("Actual prune failed: node_states count = %d, want 4 (2 remaining execs * 2 states each)", count)
	}

	// Verify correct executions remain
	rows, _ := dm.db.Query("SELECT id FROM executions")
	defer rows.Close()
	remaining := make(map[string]bool)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		remaining[id] = true
	}
	if !remaining["old-running"] || !remaining["new-success"] {
		t.Errorf("Wrong executions remain: %v", remaining)
	}
}

func TestDoltGC_Orchestration(t *testing.T) {
	dm := setupTestDB(t)
	defer dm.Close()
	ctx := t.Context()

	if err := dm.DoltGC(ctx); err != nil {
		t.Errorf("DoltGC failed: %v", err)
	}
}

func TestExportExecutions(t *testing.T) {
	dm := setupTestDB(t)
	defer dm.Close()
	ctx := t.Context()

	tmpDir := t.TempDir()
	archiveDir := filepath.Join(tmpDir, "archive")

	// 1. Setup workflow and node
	w := &Workflow{Name: "export-test"}
	if err := dm.CreateWorkflow(ctx, w); err != nil {
		t.Fatal(err)
	}
	node := &Node{WorkflowID: w.ID, WorkflowVersion: 1, Name: "n1", Type: "task"}
	if err := dm.CreateNode(ctx, node); err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	oldTime := now.Add(-48 * time.Hour)
	cutoff := now.Add(-24 * time.Hour)

	// 2. Insert 1 old success, 1 old running, 1 new success
	execs := []struct {
		id     string
		status ExecutionStatus
		ts     time.Time
	}{
		{"old-success", ExecutionStatusSuccess, oldTime},
		{"old-running", ExecutionStatusRunning, oldTime},
		{"new-success", ExecutionStatusSuccess, now},
	}

	for _, ex := range execs {
		query := `INSERT INTO executions (id, workflow_id, workflow_version, status, created_at) VALUES (?, ?, ?, ?, ?)`
		if _, err := dm.db.ExecContext(ctx, query, ex.id, w.ID, w.Version, ex.status, ex.ts); err != nil {
			t.Fatal(err)
		}
		nsID := uuid.New().String()
		nsQuery := `INSERT INTO node_states (id, execution_id, node_id, status, created_at) VALUES (?, ?, ?, ?, ?)`
		if _, err := dm.db.ExecContext(ctx, nsQuery, nsID, ex.id, node.ID, NodeStatusSuccess, ex.ts); err != nil {
			t.Fatal(err)
		}
	}

	// 3. Export
	count, path, err := dm.ExportExecutionsBeforeCutoff(ctx, cutoff, archiveDir)
	if err != nil {
		t.Fatalf("ExportExecutionsBeforeCutoff failed: %v", err)
	}

	if count != 1 {
		t.Errorf("expected 1 execution exported, got %d", count)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("archive file does not exist at %s", path)
	}

	// 4. Verify content
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var exported []*ExecutionArchive
	if err := json.Unmarshal(data, &exported); err != nil {
		t.Fatal(err)
	}

	if len(exported) != 1 {
		t.Errorf("expected 1 archive entry, got %d", len(exported))
	}

	if exported[0].Execution.ID != "old-success" {
		t.Errorf("wrong execution archived: got %s, want old-success", exported[0].Execution.ID)
	}

	if len(exported[0].NodeStates) != 1 {
		t.Errorf("expected 1 node state archived, got %d", len(exported[0].NodeStates))
	}
}
