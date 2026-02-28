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

	if err := dm.InitSchema(); err != nil {
		t.Fatalf("InitSchema() failed: %v", err)
	}

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

	if err := dm.InitSchema(); err != nil {
		t.Fatalf("InitSchema() failed: %v", err)
	}

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

func TestNodeHistoricalVersioning(t *testing.T) {
	dm := skipWithoutDolt(t)
	defer dm.Close()
	ctx := t.Context()

	if err := dm.InitSchema(); err != nil {
		t.Fatalf("InitSchema() failed: %v", err)
	}

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
	dm := skipWithoutDolt(t)
	defer dm.Close()
	ctx := t.Context()

	if err := dm.InitSchema(); err != nil {
		t.Fatalf("InitSchema() failed: %v", err)
	}

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
