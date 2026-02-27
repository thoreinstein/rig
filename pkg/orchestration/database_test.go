package orchestration

import (
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"
)

func skipWithoutDolt(t *testing.T) *DatabaseManager {
	if testing.Short() {
		t.Skip("skipping integration test (short mode)")
	}

	dsn := os.Getenv("RIG_TEST_DOLT_DSN")
	if dsn == "" {
		t.Skip("skipping integration test (RIG_TEST_DOLT_DSN not set)")
	}

	dm, err := NewDatabaseManager(dsn, true)
	if err != nil {
		t.Fatalf("Failed to connect to Dolt: %v", err)
	}

	return dm
}

func TestInitSchema(t *testing.T) {
	dm := skipWithoutDolt(t)
	defer dm.Close()

	if err := dm.InitSchema(); err != nil {
		t.Fatalf("InitSchema() failed: %v", err)
	}
}

func TestWorkflowCRUD(t *testing.T) {
	dm := skipWithoutDolt(t)
	defer dm.Close()
	ctx := t.Context()

	if err := dm.InitSchema(); err != nil {
		t.Fatalf("InitSchema() failed: %v", err)
	}

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
	dm := skipWithoutDolt(t)
	defer dm.Close()
	ctx := t.Context()

	if err := dm.InitSchema(); err != nil {
		t.Fatalf("InitSchema() failed: %v", err)
	}

	w := &Workflow{
		Name:   "def-test-" + uuid.New().String(),
		Status: WorkflowStatusActive,
	}

	nodes := []*Node{
		{Name: "node1", Type: "task"},
		{Name: "node2", Type: "task"},
	}

	// We'll set IDs after creation to satisfy FKs if needed,
	// but SaveWorkflowDefinition handles it.

	if err := dm.SaveWorkflowDefinition(ctx, w, nodes, nil); err != nil {
		t.Fatalf("SaveWorkflowDefinition() failed: %v", err)
	}

	// Verify nodes
	dbNodes, err := dm.GetNodesByWorkflow(ctx, w.ID)
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

func TestUpdateWorkflowDefinition(t *testing.T) {
	dm := skipWithoutDolt(t)
	defer dm.Close()
	ctx := t.Context()

	if err := dm.InitSchema(); err != nil {
		t.Fatalf("InitSchema() failed: %v", err)
	}

	// Initial workflow definition
	w := &Workflow{
		Name:   "def-update-test-" + uuid.New().String(),
		Status: WorkflowStatusActive,
	}

	initialNodes := []*Node{
		{Name: "node1", Type: "task"},
		{Name: "node2", Type: "task"},
	}

	if err := dm.SaveWorkflowDefinition(ctx, w, initialNodes, nil); err != nil {
		t.Fatalf("initial SaveWorkflowDefinition() failed: %v", err)
	}

	// Updated definition with overlapping node name "node1" and a new node "node3".
	// This should update the existing node1 record rather than violating a UNIQUE
	// constraint on (workflow_id, name).
	updatedNodes := []*Node{
		{Name: "node1", Type: "updated-task"},
		{Name: "node3", Type: "task"},
	}

	if err := dm.SaveWorkflowDefinition(ctx, w, updatedNodes, nil); err != nil {
		t.Fatalf("updated SaveWorkflowDefinition() failed: %v", err)
	}

	dbNodes, err := dm.GetNodesByWorkflow(ctx, w.ID)
	if err != nil {
		t.Fatalf("GetNodesByWorkflow() after update failed: %v", err)
	}

	if len(dbNodes) != 2 {
		t.Fatalf("Got %d nodes after update, want 2", len(dbNodes))
	}

	nodeByName := make(map[string]*Node, len(dbNodes))
	for _, n := range dbNodes {
		nodeByName[n.Name] = n
	}

	if n, ok := nodeByName["node1"]; !ok {
		t.Fatalf("Updated nodes missing 'node1'")
	} else if n.Type != "updated-task" {
		t.Errorf("node1 type = %q, want %q", n.Type, "updated-task")
	}

	if _, ok := nodeByName["node3"]; !ok {
		t.Fatalf("Updated nodes missing 'node3'")
	}
}
func TestExecutionLifecycle(t *testing.T) {
	dm := skipWithoutDolt(t)
	defer dm.Close()
	ctx := t.Context()

	if err := dm.InitSchema(); err != nil {
		t.Fatalf("InitSchema() failed: %v", err)
	}

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
	dm := skipWithoutDolt(t)
	defer dm.Close()
	ctx := t.Context()

	if err := dm.InitSchema(); err != nil {
		t.Fatalf("InitSchema() failed: %v", err)
	}

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
	dm := skipWithoutDolt(t)
	defer dm.Close()
	ctx := t.Context()

	_ = dm.InitSchema()

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
	dm := skipWithoutDolt(t)
	defer dm.Close()
	ctx := t.Context()

	_ = dm.InitSchema()

	w := &Workflow{
		Name:   "concurrency-test-" + uuid.New().String(),
		Status: WorkflowStatusActive,
	}
	if err := dm.CreateWorkflow(ctx, w); err != nil {
		t.Fatalf("CreateWorkflow failed: %v", err)
	}

	const workers = 10
	errChan := make(chan error, workers)

	// 1. Test Concurrent Version Increments
	for i := range workers {
		go func(idx int) {
			update := &Workflow{
				ID:          w.ID,
				Name:        w.Name,
				Description: fmt.Sprintf("Update from worker %d", idx),
			}
			errChan <- dm.UpdateWorkflow(ctx, update)
		}(i)
	}

	for range workers {
		if err := <-errChan; err != nil {
			t.Errorf("Concurrent UpdateWorkflow failed: %v", err)
		}
	}

	final, err := dm.GetWorkflow(ctx, w.ID)
	if err != nil {
		t.Fatalf("GetWorkflow failed: %v", err)
	}

	expectedVersion := 1 + workers
	if final.Version != expectedVersion {
		t.Errorf("Final version = %d, want %d (concurrency collapse!)", final.Version, expectedVersion)
	}

	// 2. Test Concurrent Execution vs Definition Update
	// This is non-deterministic but we should never get a successful update
	// if an execution is currently PENDING/RUNNING.

	// We'll create a PENDING execution and then try to update.
	exec := &Execution{
		WorkflowID:      w.ID,
		WorkflowVersion: final.Version,
		Status:          ExecutionStatusPending,
	}
	if err := dm.CreateExecution(ctx, exec); err != nil {
		t.Fatalf("CreateExecution failed: %v", err)
	}

	// This update MUST fail because of the PENDING execution
	err = dm.UpdateWorkflow(ctx, final)
	if err == nil {
		t.Error("UpdateWorkflow should have failed due to active PENDING execution")
	}
}
