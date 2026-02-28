package orchestration

import (
	"context"
	"sync"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type MockStore struct {
	mu            sync.Mutex
	workflows     map[string]*Workflow
	executions    map[string]*Execution
	nodes         map[string][]*Node
	edges         map[string][]*Edge
	nodeStates    map[string][]*NodeState
	updateExecErr error
	updateNodeErr error
}

func (m *MockStore) GetWorkflow(ctx context.Context, id string) (*Workflow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if w, ok := m.workflows[id]; ok {
		return w, nil
	}
	return nil, errors.New("not found")
}

func (m *MockStore) GetExecution(ctx context.Context, id string) (*Execution, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e, ok := m.executions[id]; ok {
		return e, nil
	}
	return nil, errors.New("not found")
}

func (m *MockStore) GetNodesByWorkflow(ctx context.Context, workflowID string, version int) ([]*Node, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.nodes[workflowID], nil
}

func (m *MockStore) GetEdgesByWorkflow(ctx context.Context, workflowID string, version int) ([]*Edge, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.edges[workflowID], nil
}

func (m *MockStore) GetNodeStatesByExecution(ctx context.Context, executionID string) ([]*NodeState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.nodeStates[executionID], nil
}

func (m *MockStore) UpdateExecutionStatus(ctx context.Context, id string, status ExecutionStatus, errMsg string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.updateExecErr != nil {
		return m.updateExecErr
	}
	if e, ok := m.executions[id]; ok {
		if !isValidExecutionTransition(e.Status, status) {
			return errors.Errorf("invalid execution status transition from %s to %s", e.Status, status)
		}
		e.Status = status
		e.Error = errMsg
		return nil
	}
	return errors.New("not found")
}

func (m *MockStore) UpdateNodeStatus(ctx context.Context, id string, status NodeStatus, result []byte, errMsg string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.updateNodeErr != nil {
		return m.updateNodeErr
	}
	for _, states := range m.nodeStates {
		for _, s := range states {
			if s.ID == id {
				if !isValidNodeTransition(s.Status, status) {
					return errors.Errorf("invalid node status transition from %s to %s", s.Status, status)
				}
				s.Status = status
				s.Result = result
				s.Error = errMsg
				return nil
			}
		}
	}
	return errors.New("not found")
}

func TestOrchestrator_Execute_Mock(t *testing.T) {
	ctx := t.Context()

	t.Run("mock success with data passing", func(t *testing.T) {
		wID := "w1"
		eID := "e1"

		nodes := []*Node{
			{ID: "n1", Name: "hello", Type: "hello"},
			{ID: "n2", Name: "world", Type: "world"},
		}
		edges := []*Edge{
			{SourceNodeID: "n1", TargetNodeID: "n2"},
		}

		store := &MockStore{
			workflows: map[string]*Workflow{
				wID: {ID: wID, Version: 1},
			},
			executions: map[string]*Execution{
				eID: {ID: eID, WorkflowID: wID, WorkflowVersion: 1, Status: ExecutionStatusPending},
			},
			nodes: map[string][]*Node{wID: nodes},
			edges: map[string][]*Edge{wID: edges},
			nodeStates: map[string][]*NodeState{
				eID: {
					{ID: "s1", NodeID: "n1", ExecutionID: eID, Status: NodeStatusPending},
					{ID: "s2", NodeID: "n2", ExecutionID: eID, Status: NodeStatusPending},
				},
			},
		}

		o := NewOrchestrator(store)
		err := o.Execute(ctx, eID)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		if store.executions[eID].Status != ExecutionStatusSuccess {
			t.Errorf("Expected Success, got %s", store.executions[eID].Status)
		}
	})
}

func TestOrchestrator_Execute_MockFailureSkip(t *testing.T) {
	ctx := t.Context()

	t.Run("failure propagation and skip", func(t *testing.T) {
		wID := "w-fail"
		eID := "e-fail"

		// A (fail) -> B (should be skipped)
		nodes := []*Node{
			{ID: "n1", Name: "fail-node", Type: "fail"},
			{ID: "n2", Name: "skip-node", Type: "world"},
		}
		edges := []*Edge{
			{SourceNodeID: "n1", TargetNodeID: "n2"},
		}

		store := &MockStore{
			workflows: map[string]*Workflow{
				wID: {ID: wID, Version: 1},
			},
			executions: map[string]*Execution{
				eID: {ID: eID, WorkflowID: wID, WorkflowVersion: 1, Status: ExecutionStatusPending},
			},
			nodes: map[string][]*Node{wID: nodes},
			edges: map[string][]*Edge{wID: edges},
			nodeStates: map[string][]*NodeState{
				eID: {
					{ID: "s1", NodeID: "n1", ExecutionID: eID, Status: NodeStatusPending},
					{ID: "s2", NodeID: "n2", ExecutionID: eID, Status: NodeStatusPending},
				},
			},
		}

		o := NewOrchestrator(store)
		err := o.Execute(ctx, eID)
		if err == nil {
			t.Fatal("Expected Execute to return an error")
		}

		if store.executions[eID].Status != ExecutionStatusFailed {
			t.Errorf("Expected execution status FAILED, got %s", store.executions[eID].Status)
		}

		// Verify the failed node is FAILED and the downstream node is SKIPPED
		for _, ns := range store.nodeStates[eID] {
			switch ns.NodeID {
			case "n1":
				if ns.Status != NodeStatusFailed {
					t.Errorf("Node n1 expected FAILED, got %s", ns.Status)
				}
			case "n2":
				if ns.Status != NodeStatusSkipped {
					t.Errorf("Node n2 expected SKIPPED, got %s", ns.Status)
				}
			}
		}
	})
}

func TestOrchestrator_Execute_MockDiamond(t *testing.T) {
	ctx := t.Context()

	t.Run("diamond DAG with concurrent middle nodes", func(t *testing.T) {
		wID := "w-diamond"
		eID := "e-diamond"

		// A -> B, A -> C, B -> D, C -> D
		nodes := []*Node{
			{ID: "A", Name: "root", Type: "hello"},
			{ID: "B", Name: "left", Type: "hello"},
			{ID: "C", Name: "right", Type: "world"},
			{ID: "D", Name: "join", Type: "hello"},
		}
		edges := []*Edge{
			{SourceNodeID: "A", TargetNodeID: "B"},
			{SourceNodeID: "A", TargetNodeID: "C"},
			{SourceNodeID: "B", TargetNodeID: "D"},
			{SourceNodeID: "C", TargetNodeID: "D"},
		}

		store := &MockStore{
			workflows: map[string]*Workflow{
				wID: {ID: wID, Version: 1},
			},
			executions: map[string]*Execution{
				eID: {ID: eID, WorkflowID: wID, WorkflowVersion: 1, Status: ExecutionStatusPending},
			},
			nodes: map[string][]*Node{wID: nodes},
			edges: map[string][]*Edge{wID: edges},
			nodeStates: map[string][]*NodeState{
				eID: {
					{ID: "sA", NodeID: "A", ExecutionID: eID, Status: NodeStatusPending},
					{ID: "sB", NodeID: "B", ExecutionID: eID, Status: NodeStatusPending},
					{ID: "sC", NodeID: "C", ExecutionID: eID, Status: NodeStatusPending},
					{ID: "sD", NodeID: "D", ExecutionID: eID, Status: NodeStatusPending},
				},
			},
		}

		o := NewOrchestrator(store)
		err := o.Execute(ctx, eID)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		if store.executions[eID].Status != ExecutionStatusSuccess {
			t.Errorf("Expected execution status SUCCESS, got %s", store.executions[eID].Status)
		}

		// All four nodes should be SUCCESS
		for _, ns := range store.nodeStates[eID] {
			if ns.Status != NodeStatusSuccess {
				t.Errorf("Node %s expected SUCCESS, got %s", ns.NodeID, ns.Status)
			}
		}
	})
}

func TestHelloWorldWorkflow(t *testing.T) {
	dm := skipWithoutDolt(t)
	defer dm.Close()
	ctx := t.Context()

	if err := dm.InitSchema(); err != nil {
		t.Fatalf("InitSchema() failed: %v", err)
	}

	o := NewOrchestrator(dm)

	w := &Workflow{Name: "hello-world-dag", Status: WorkflowStatusActive}
	nodes := []*Node{
		{ID: "A", Name: "PrintHello", Type: "hello"},
		{ID: "B", Name: "PrintWorld", Type: "world"},
	}
	edges := []*Edge{
		{ID: "A-B", SourceNodeID: "A", TargetNodeID: "B"},
	}

	if err := dm.SaveWorkflowDefinition(ctx, w, nodes, edges); err != nil {
		t.Fatalf("SaveWorkflowDefinition() failed: %v", err)
	}

	exec, err := dm.CreateExecutionWithInitialStates(ctx, w.ID, w.Version)
	if err != nil {
		t.Fatalf("CreateExecutionWithInitialStates() failed: %v", err)
	}

	if err := o.Execute(ctx, exec.ID); err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}

	// Persist checks
	finalExec, err := dm.GetExecution(ctx, exec.ID)
	if err != nil {
		t.Fatalf("GetExecution() failed: %v", err)
	}
	if finalExec.Status != ExecutionStatusSuccess {
		t.Errorf("Expected SUCCESS state, got %s", finalExec.Status)
	}
}

func TestOrchestrator_Execute_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel() // Pre-cancel to test cancellation handling

	wID := "w-cancel"
	eID := "e-cancel"

	nodes := []*Node{
		{ID: "n1", Name: "hello", Type: "hello"},
		{ID: "n2", Name: "world", Type: "world"},
	}
	edges := []*Edge{
		{SourceNodeID: "n1", TargetNodeID: "n2"},
	}

	store := &MockStore{
		workflows: map[string]*Workflow{
			wID: {ID: wID, Version: 1},
		},
		executions: map[string]*Execution{
			eID: {ID: eID, WorkflowID: wID, WorkflowVersion: 1, Status: ExecutionStatusPending},
		},
		nodes: map[string][]*Node{wID: nodes},
		edges: map[string][]*Edge{wID: edges},
		nodeStates: map[string][]*NodeState{
			eID: {
				{ID: "s1", NodeID: "n1", ExecutionID: eID, Status: NodeStatusPending},
				{ID: "s2", NodeID: "n2", ExecutionID: eID, Status: NodeStatusPending},
			},
		},
	}

	o := NewOrchestrator(store)
	err := o.Execute(ctx, eID)
	if err == nil {
		t.Fatal("Expected Execute to return an error on cancelled context")
	}

	store.mu.Lock()
	defer store.mu.Unlock()

	if store.executions[eID].Status != ExecutionStatusFailed {
		t.Errorf("Expected execution status FAILED, got %s", store.executions[eID].Status)
	}

	// All nodes should be SKIPPED since context was cancelled before any launched
	for _, ns := range store.nodeStates[eID] {
		if ns.Status != NodeStatusSkipped {
			t.Errorf("Node %s expected SKIPPED, got %s", ns.NodeID, ns.Status)
		}
	}
}

func TestOrchestrator_Execute_PanicRecovery(t *testing.T) {
	ctx := t.Context()

	wID := "w-panic"
	eID := "e-panic"

	// A (panic) -> B (should be skipped)
	nodes := []*Node{
		{ID: "n1", Name: "panic-node", Type: "panic"},
		{ID: "n2", Name: "skip-node", Type: "world"},
	}
	edges := []*Edge{
		{SourceNodeID: "n1", TargetNodeID: "n2"},
	}

	store := &MockStore{
		workflows: map[string]*Workflow{
			wID: {ID: wID, Version: 1},
		},
		executions: map[string]*Execution{
			eID: {ID: eID, WorkflowID: wID, WorkflowVersion: 1, Status: ExecutionStatusPending},
		},
		nodes: map[string][]*Node{wID: nodes},
		edges: map[string][]*Edge{wID: edges},
		nodeStates: map[string][]*NodeState{
			eID: {
				{ID: "s1", NodeID: "n1", ExecutionID: eID, Status: NodeStatusPending},
				{ID: "s2", NodeID: "n2", ExecutionID: eID, Status: NodeStatusPending},
			},
		},
	}

	o := NewOrchestrator(store)
	err := o.Execute(ctx, eID)
	if err == nil {
		t.Fatal("Expected Execute to return an error on panic")
	}

	store.mu.Lock()
	defer store.mu.Unlock()

	if store.executions[eID].Status != ExecutionStatusFailed {
		t.Errorf("Expected execution status FAILED, got %s", store.executions[eID].Status)
	}

	// Panicked node should be FAILED, downstream should be SKIPPED
	for _, ns := range store.nodeStates[eID] {
		switch ns.NodeID {
		case "n1":
			if ns.Status != NodeStatusFailed {
				t.Errorf("Node n1 expected FAILED, got %s", ns.Status)
			}
		case "n2":
			if ns.Status != NodeStatusSkipped {
				t.Errorf("Node n2 expected SKIPPED, got %s", ns.Status)
			}
		}
	}
}

func TestValidateWorkflow(t *testing.T) {
	tests := []struct {
		name    string
		nodes   []*Node
		edges   []*Edge
		wantErr bool
	}{
		{
			name: "valid linear DAG",
			nodes: []*Node{
				{ID: "1"},
				{ID: "2"},
				{ID: "3"},
			},
			edges: []*Edge{
				{SourceNodeID: "1", TargetNodeID: "2"},
				{SourceNodeID: "2", TargetNodeID: "3"},
			},
			wantErr: false,
		},
		{
			name: "valid branching DAG",
			nodes: []*Node{
				{ID: "1"},
				{ID: "2"},
				{ID: "3"},
				{ID: "4"},
			},
			edges: []*Edge{
				{SourceNodeID: "1", TargetNodeID: "2"},
				{SourceNodeID: "1", TargetNodeID: "3"},
				{SourceNodeID: "2", TargetNodeID: "4"},
				{SourceNodeID: "3", TargetNodeID: "4"},
			},
			wantErr: false,
		},
		{
			name: "simple cycle",
			nodes: []*Node{
				{ID: "1"},
				{ID: "2"},
			},
			edges: []*Edge{
				{SourceNodeID: "1", TargetNodeID: "2"},
				{SourceNodeID: "2", TargetNodeID: "1"},
			},
			wantErr: true,
		},
		{
			name: "self cycle",
			nodes: []*Node{
				{ID: "1"},
			},
			edges: []*Edge{
				{SourceNodeID: "1", TargetNodeID: "1"},
			},
			wantErr: true,
		},
		{
			name: "disconnected valid components",
			nodes: []*Node{
				{ID: "1"},
				{ID: "2"},
				{ID: "3"},
				{ID: "4"},
			},
			edges: []*Edge{
				{SourceNodeID: "1", TargetNodeID: "2"},
				{SourceNodeID: "3", TargetNodeID: "4"},
			},
			wantErr: false,
		},
		{
			name: "complex cycle",
			nodes: []*Node{
				{ID: "1"},
				{ID: "2"},
				{ID: "3"},
				{ID: "4"},
			},
			edges: []*Edge{
				{SourceNodeID: "1", TargetNodeID: "2"},
				{SourceNodeID: "2", TargetNodeID: "3"},
				{SourceNodeID: "3", TargetNodeID: "4"},
				{SourceNodeID: "4", TargetNodeID: "2"},
			},
			wantErr: true,
		},
		{
			name: "edge references non-existent node",
			nodes: []*Node{
				{ID: "1"},
			},
			edges: []*Edge{
				{SourceNodeID: "1", TargetNodeID: "2"},
			},
			wantErr: true,
		},
		{
			name: "empty node ID",
			nodes: []*Node{
				{ID: ""},
			},
			edges:   nil,
			wantErr: true,
		},
		{
			name: "duplicate node IDs",
			nodes: []*Node{
				{ID: "1"},
				{ID: "1"},
			},
			edges:   nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateWorkflow(tt.nodes, tt.edges); (err != nil) != tt.wantErr {
				t.Errorf("ValidateWorkflow() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOrchestrator_Execute(t *testing.T) {
	dm := skipWithoutDolt(t)
	defer dm.Close()
	ctx := t.Context()

	if err := dm.InitSchema(); err != nil {
		t.Fatalf("InitSchema() failed: %v", err)
	}

	o := NewOrchestrator(dm)

	t.Run("successful linear execution", func(t *testing.T) {
		w := &Workflow{Name: "success-test-" + uuid.New().String(), Status: WorkflowStatusActive}
		nodes := []*Node{
			{ID: "n1", Name: "hello", Type: "hello"},
			{ID: "n2", Name: "world", Type: "world"},
		}
		edges := []*Edge{
			{ID: "e1", SourceNodeID: "n1", TargetNodeID: "n2"},
		}

		if err := dm.SaveWorkflowDefinition(ctx, w, nodes, edges); err != nil {
			t.Fatalf("SaveWorkflowDefinition() failed: %v", err)
		}

		exec, err := dm.CreateExecutionWithInitialStates(ctx, w.ID, w.Version)
		if err != nil {
			t.Fatalf("CreateExecutionWithInitialStates() failed: %v", err)
		}

		if err := o.Execute(ctx, exec.ID); err != nil {
			t.Fatalf("Execute() failed: %v", err)
		}

		// Verify final status
		finalExec, err := dm.GetExecution(ctx, exec.ID)
		if err != nil {
			t.Fatalf("GetExecution() failed: %v", err)
		}
		if finalExec.Status != ExecutionStatusSuccess {
			t.Errorf("Expected status SUCCESS, got %s", finalExec.Status)
		}

		nodeStates, err := dm.GetNodeStatesByExecution(ctx, exec.ID)
		if err != nil {
			t.Fatalf("GetNodeStatesByExecution() failed: %v", err)
		}
		for _, ns := range nodeStates {
			if ns.Status != NodeStatusSuccess {
				t.Errorf("Node %s expected status SUCCESS, got %s", ns.NodeID, ns.Status)
			}
		}
	})

	t.Run("execution with failure and skip", func(t *testing.T) {
		w := &Workflow{Name: "fail-test-" + uuid.New().String(), Status: WorkflowStatusActive}
		nodes := []*Node{
			{ID: "f1", Name: "fail-node", Type: "fail"},
			{ID: "s1", Name: "skip-node", Type: "world"},
		}
		edges := []*Edge{
			{ID: "fe1", SourceNodeID: "f1", TargetNodeID: "s1"},
		}

		if err := dm.SaveWorkflowDefinition(ctx, w, nodes, edges); err != nil {
			t.Fatalf("SaveWorkflowDefinition() failed: %v", err)
		}
		exec, err := dm.CreateExecutionWithInitialStates(ctx, w.ID, w.Version)
		if err != nil {
			t.Fatalf("CreateExecutionWithInitialStates() failed: %v", err)
		}

		err = o.Execute(ctx, exec.ID)
		if err == nil {
			t.Error("Expected Execute() to fail")
		}

		finalExec, err := dm.GetExecution(ctx, exec.ID)
		if err != nil {
			t.Fatalf("GetExecution() failed: %v", err)
		}
		if finalExec.Status != ExecutionStatusFailed {
			t.Errorf("Expected status FAILED, got %s", finalExec.Status)
		}

		nodeStates, err := dm.GetNodeStatesByExecution(ctx, exec.ID)
		if err != nil {
			t.Fatalf("GetNodeStatesByExecution() failed: %v", err)
		}
		for _, ns := range nodeStates {
			if ns.NodeID == "f1" && ns.Status != NodeStatusFailed {
				t.Errorf("Node f1 expected status FAILED, got %s", ns.Status)
			}
			if ns.NodeID == "s1" && ns.Status != NodeStatusSkipped {
				t.Errorf("Node s1 expected status SKIPPED, got %s", ns.Status)
			}
		}
	})
}
