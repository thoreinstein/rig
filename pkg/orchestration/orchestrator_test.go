package orchestration

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/cockroachdb/errors"
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

func TestOrchestrator_Execute_Recovery(t *testing.T) {
	ctx := t.Context()

	wID := "w1"
	eID := "e1"

	// Define a diamond graph: A -> {B, C} -> D
	nodes := []*Node{
		{ID: "A", WorkflowID: wID, WorkflowVersion: 1, Type: "hello"},
		{ID: "B", WorkflowID: wID, WorkflowVersion: 1, Type: "hello"},
		{ID: "C", WorkflowID: wID, WorkflowVersion: 1, Type: "hello"},
		{ID: "D", WorkflowID: wID, WorkflowVersion: 1, Type: "hello"},
	}
	edges := []*Edge{
		{ID: "e_ab", SourceNodeID: "A", TargetNodeID: "B", WorkflowID: wID, WorkflowVersion: 1},
		{ID: "e_ac", SourceNodeID: "A", TargetNodeID: "C", WorkflowID: wID, WorkflowVersion: 1},
		{ID: "e_bd", SourceNodeID: "B", TargetNodeID: "D", WorkflowID: wID, WorkflowVersion: 1},
		{ID: "e_cd", SourceNodeID: "C", TargetNodeID: "D", WorkflowID: wID, WorkflowVersion: 1},
	}

	tests := []struct {
		name             string
		initialStates    []*NodeState
		expectRun        map[string]bool                       // which nodes we expect the bridge to execute
		expectInputs     map[string]map[string]json.RawMessage // nodeID -> expected inputs map (keyed by source nodeID)
		expectStatus     ExecutionStatus
		expectNodeStatus map[string]NodeStatus // expected final status of every node
	}{
		{
			name: "Diamond partial resume - A success, resume B and C",
			initialStates: []*NodeState{
				{ID: "sA", NodeID: "A", ExecutionID: eID, Status: NodeStatusSuccess, Result: json.RawMessage(`{"output":"A"}`)},
				// B was RUNNING when the process crashed. Recovery does not mark
				// RUNNING nodes as "launched" — they must be re-executed because
				// the goroutine doing the work is gone.
				{ID: "sB", NodeID: "B", ExecutionID: eID, Status: NodeStatusRunning},
				{ID: "sC", NodeID: "C", ExecutionID: eID, Status: NodeStatusPending},
				{ID: "sD", NodeID: "D", ExecutionID: eID, Status: NodeStatusPending},
			},
			expectRun: map[string]bool{"B": true, "C": true, "D": true},
			expectInputs: map[string]map[string]json.RawMessage{
				"B": {"A": json.RawMessage(`{"output":"A"}`)}, // B receives restored A result
				"C": {"A": json.RawMessage(`{"output":"A"}`)}, // C receives restored A result
			},
			expectStatus: ExecutionStatusSuccess,
			expectNodeStatus: map[string]NodeStatus{
				"A": NodeStatusSuccess,
				"B": NodeStatusSuccess,
				"C": NodeStatusSuccess,
				"D": NodeStatusSuccess,
			},
		},
		{
			name: "Diamond resume - A, B success, resume C and D",
			initialStates: []*NodeState{
				{ID: "sA", NodeID: "A", ExecutionID: eID, Status: NodeStatusSuccess, Result: json.RawMessage(`{"output":"A"}`)},
				{ID: "sB", NodeID: "B", ExecutionID: eID, Status: NodeStatusSuccess, Result: json.RawMessage(`{"output":"B"}`)},
				{ID: "sC", NodeID: "C", ExecutionID: eID, Status: NodeStatusPending},
				{ID: "sD", NodeID: "D", ExecutionID: eID, Status: NodeStatusPending},
			},
			expectRun: map[string]bool{"C": true, "D": true},
			expectInputs: map[string]map[string]json.RawMessage{
				"C": {"A": json.RawMessage(`{"output":"A"}`)},                                          // C receives restored A result
				"D": {"B": json.RawMessage(`{"output":"B"}`), "C": json.RawMessage(`{"status":"ok"}`)}, // D receives restored B + fresh C
			},
			expectStatus: ExecutionStatusSuccess,
			expectNodeStatus: map[string]NodeStatus{
				"A": NodeStatusSuccess,
				"B": NodeStatusSuccess,
				"C": NodeStatusSuccess,
				"D": NodeStatusSuccess,
			},
		},
		{
			name: "All complete no-op",
			initialStates: []*NodeState{
				{ID: "sA", NodeID: "A", ExecutionID: eID, Status: NodeStatusSuccess, Result: json.RawMessage(`{"output":"A"}`)},
				{ID: "sB", NodeID: "B", ExecutionID: eID, Status: NodeStatusSuccess, Result: json.RawMessage(`{"output":"B"}`)},
				{ID: "sC", NodeID: "C", ExecutionID: eID, Status: NodeStatusSuccess, Result: json.RawMessage(`{"output":"C"}`)},
				{ID: "sD", NodeID: "D", ExecutionID: eID, Status: NodeStatusSuccess, Result: json.RawMessage(`{"output":"D"}`)},
			},
			expectRun:    map[string]bool{},
			expectStatus: ExecutionStatusSuccess,
			expectNodeStatus: map[string]NodeStatus{
				"A": NodeStatusSuccess,
				"B": NodeStatusSuccess,
				"C": NodeStatusSuccess,
				"D": NodeStatusSuccess,
			},
		},
		{
			name: "Pre-crash failure - short circuit",
			initialStates: []*NodeState{
				{ID: "sA", NodeID: "A", ExecutionID: eID, Status: NodeStatusSuccess, Result: json.RawMessage(`{"output":"A"}`)},
				{ID: "sB", NodeID: "B", ExecutionID: eID, Status: NodeStatusFailed, Error: "boom"},
				{ID: "sC", NodeID: "C", ExecutionID: eID, Status: NodeStatusPending},
				{ID: "sD", NodeID: "D", ExecutionID: eID, Status: NodeStatusPending},
			},
			expectRun:    map[string]bool{},
			expectStatus: ExecutionStatusFailed,
			expectNodeStatus: map[string]NodeStatus{
				"A": NodeStatusSuccess,
				"B": NodeStatusFailed,
				"C": NodeStatusSkipped,
				"D": NodeStatusSkipped,
			},
		},
		{
			name: "RUNNING node with FAILED sibling - short circuit",
			initialStates: []*NodeState{
				{ID: "sA", NodeID: "A", ExecutionID: eID, Status: NodeStatusSuccess, Result: json.RawMessage(`{"output":"A"}`)},
				{ID: "sB", NodeID: "B", ExecutionID: eID, Status: NodeStatusRunning},
				{ID: "sC", NodeID: "C", ExecutionID: eID, Status: NodeStatusFailed, Error: "boom"},
				{ID: "sD", NodeID: "D", ExecutionID: eID, Status: NodeStatusPending},
			},
			expectRun:    map[string]bool{},
			expectStatus: ExecutionStatusFailed,
			expectNodeStatus: map[string]NodeStatus{
				"A": NodeStatusSuccess,
				"B": NodeStatusSkipped,
				"C": NodeStatusFailed,
				"D": NodeStatusSkipped,
			},
		},
		{
			name: "Resuming with SKIPPED nodes (no FAILED nodes)",
			initialStates: []*NodeState{
				{ID: "sA", NodeID: "A", ExecutionID: eID, Status: NodeStatusSuccess, Result: json.RawMessage(`{"output":"A"}`)},
				{ID: "sB", NodeID: "B", ExecutionID: eID, Status: NodeStatusSkipped, Error: "upstream failure"},
				{ID: "sC", NodeID: "C", ExecutionID: eID, Status: NodeStatusSkipped, Error: "upstream failure"},
				{ID: "sD", NodeID: "D", ExecutionID: eID, Status: NodeStatusPending},
			},
			expectRun:    map[string]bool{},
			expectStatus: ExecutionStatusFailed,
			expectNodeStatus: map[string]NodeStatus{
				"A": NodeStatusSuccess,
				"B": NodeStatusSkipped,
				"C": NodeStatusSkipped,
				"D": NodeStatusSkipped,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executed := make(map[string]int)
			capturedInputs := make(map[string]map[string]json.RawMessage)
			var mu sync.Mutex

			bridge := &MockNodeBridge{
				ExecuteNodeFunc: func(ctx context.Context, node *Node, caps *NodeCapabilities, pluginConfig json.RawMessage, inputs map[string]json.RawMessage, secrets map[string]string) (json.RawMessage, error) {
					mu.Lock()
					executed[node.ID]++
					inputsCopy := make(map[string]json.RawMessage, len(inputs))
					for k, v := range inputs {
						inputsCopy[k] = v
					}
					capturedInputs[node.ID] = inputsCopy
					mu.Unlock()
					return json.RawMessage(`{"status":"ok"}`), nil
				},
			}

			store := &MockStore{
				workflows: map[string]*Workflow{
					wID: {ID: wID, Name: "test", Version: 1},
				},
				executions: map[string]*Execution{
					eID: {ID: eID, WorkflowID: wID, WorkflowVersion: 1, Status: ExecutionStatusRunning},
				},
				nodes: map[string][]*Node{
					wID: nodes,
				},
				edges: map[string][]*Edge{
					wID: edges,
				},
				nodeStates: map[string][]*NodeState{
					eID: tt.initialStates,
				},
			}

			o := NewOrchestrator(store, WithNodeBridge(bridge))
			err := o.Execute(ctx, eID)

			if tt.expectStatus == ExecutionStatusSuccess && err != nil {
				t.Fatalf("Expected success, got error: %v", err)
			}
			if tt.expectStatus == ExecutionStatusFailed && err == nil {
				t.Fatal("Expected error, got nil")
			}

			if store.executions[eID].Status != tt.expectStatus {
				t.Errorf("Expected execution status %s, got %s", tt.expectStatus, store.executions[eID].Status)
			}

			for id, count := range executed {
				if !tt.expectRun[id] {
					t.Errorf("Node %s was executed %d times, expected 0", id, count)
				}
			}
			for id := range tt.expectRun {
				count := executed[id]
				if count == 0 {
					t.Errorf("Node %s was NOT executed, expected it to run exactly once", id)
				} else if count > 1 {
					t.Errorf("Node %s was executed %d times, expected exactly once", id, count)
				}
			}

			// Verify that resumed nodes received the expected inputs from restored predecessors.
			for nodeID, wantInputs := range tt.expectInputs {
				gotInputs := capturedInputs[nodeID]
				for srcID, wantVal := range wantInputs {
					gotVal, ok := gotInputs[srcID]
					if !ok {
						t.Errorf("Node %s expected input from %s, but it was not present", nodeID, srcID)
					} else if string(gotVal) != string(wantVal) {
						t.Errorf("Node %s input from %s: got %s, want %s", nodeID, srcID, gotVal, wantVal)
					}
				}
			}

			// Verify every node reached its expected terminal status in the store.
			for _, ns := range store.nodeStates[eID] {
				want, ok := tt.expectNodeStatus[ns.NodeID]
				if !ok {
					continue
				}
				if ns.Status != want {
					t.Errorf("Node %s expected status %s, got %s", ns.NodeID, want, ns.Status)
				}
			}
		})
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

func TestOrchestrator_Execute_TerminalStateGuard(t *testing.T) {
	ctx := t.Context()

	terminalStatuses := []ExecutionStatus{
		ExecutionStatusSuccess,
		ExecutionStatusFailed,
		ExecutionStatusCancelled,
	}

	for _, status := range terminalStatuses {
		t.Run(string(status), func(t *testing.T) {
			store := &MockStore{
				workflows: map[string]*Workflow{
					"w1": {ID: "w1", Name: "test", Version: 1},
				},
				executions: map[string]*Execution{
					"e1": {ID: "e1", WorkflowID: "w1", WorkflowVersion: 1, Status: status},
				},
			}
			o := NewOrchestrator(store)
			err := o.Execute(ctx, "e1")
			if err == nil {
				t.Fatalf("Expected error for terminal status %s, got nil", status)
			}
			if !errors.Is(err, err) { // ensure it's a real error, not nil
				t.Fatalf("Unexpected error type: %v", err)
			}
		})
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
