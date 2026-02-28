package orchestration

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/cockroachdb/errors"
)

type MockNodeBridge struct {
	ExecuteNodeFunc func(ctx context.Context, node *Node, caps *NodeCapabilities, pluginConfig json.RawMessage, inputs map[string]json.RawMessage, secrets map[string]string) (json.RawMessage, error)
}

func (m *MockNodeBridge) ExecuteNode(ctx context.Context, node *Node, caps *NodeCapabilities, pluginConfig json.RawMessage, inputs map[string]json.RawMessage, secrets map[string]string) (json.RawMessage, error) {
	if m.ExecuteNodeFunc != nil {
		return m.ExecuteNodeFunc(ctx, node, caps, pluginConfig, inputs, secrets)
	}
	return json.RawMessage(`{"mock": "success"}`), nil
}

type MockSecretResolver struct {
	secrets map[string]string
}

func (m *MockSecretResolver) Resolve(ctx context.Context, key string) (string, error) {
	val, ok := m.secrets[key]
	if !ok {
		return "", errors.Newf("secret not found: %s", key)
	}
	return val, nil
}

func TestOrchestrator_Execute_WithBridge(t *testing.T) {
	// Setup a simple workflow with two nodes where Node2 depends on Node1
	store := NewMemoryStore()
	ctx := t.Context()

	wf := &Workflow{ID: "wf-1", Version: 1, Status: WorkflowStatusActive}
	store.workflows[wf.ID] = wf

	exec := &Execution{ID: "exec-1", WorkflowID: "wf-1", WorkflowVersion: 1, Status: ExecutionStatusPending}
	store.executions[exec.ID] = exec

	n1 := &Node{
		ID: "n1", WorkflowID: "wf-1", WorkflowVersion: 1, Type: "test-plugin",
		Config: json.RawMessage(`{"capabilities": {"secrets_mapping": {"KEY": "src-key"}}}`),
	}
	n2 := &Node{
		ID: "n2", WorkflowID: "wf-1", WorkflowVersion: 1, Type: "test-plugin",
		Config: json.RawMessage(`{"plugin": {"task": "do-work"}}`),
	}
	store.nodes["wf-1:1"] = []*Node{n1, n2}

	edge := &Edge{
		ID: "e1", WorkflowID: "wf-1", WorkflowVersion: 1,
		SourceNodeID: "n1", TargetNodeID: "n2",
	}
	store.edges["wf-1:1"] = []*Edge{edge}

	store.nodeStates["exec-1"] = []*NodeState{
		{ID: "ns1", ExecutionID: "exec-1", NodeID: "n1", Status: NodeStatusPending},
		{ID: "ns2", ExecutionID: "exec-1", NodeID: "n2", Status: NodeStatusPending},
	}

	// Track execution order and inputs
	var execOrder []string
	var n2Inputs map[string]json.RawMessage

	bridge := &MockNodeBridge{
		ExecuteNodeFunc: func(ctx context.Context, node *Node, caps *NodeCapabilities, pluginConfig json.RawMessage, inputs map[string]json.RawMessage, secrets map[string]string) (json.RawMessage, error) {
			execOrder = append(execOrder, node.ID)

			if node.ID == "n1" {
				if secrets["KEY"] != "resolved-value" {
					t.Errorf("expected secret 'KEY' to be 'resolved-value', got %q", secrets["KEY"])
				}
				return json.RawMessage(`{"output": "from-n1"}`), nil
			}

			if node.ID == "n2" {
				n2Inputs = inputs
				return json.RawMessage(`{"output": "from-n2"}`), nil
			}

			return nil, errors.New("unexpected node")
		},
	}

	resolver := &MockSecretResolver{
		secrets: map[string]string{"src-key": "resolved-value"},
	}

	orchestrator := NewOrchestrator(store, WithNodeBridge(bridge), WithSecretResolver(resolver))

	err := orchestrator.Execute(ctx, "exec-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify execution order
	if len(execOrder) != 2 || execOrder[0] != "n1" || execOrder[1] != "n2" {
		t.Errorf("expected execution order [n1, n2], got %v", execOrder)
	}

	// Verify n2 received n1's output as input
	if val, ok := n2Inputs["n1"]; !ok || string(val) != `{"output": "from-n1"}` {
		t.Errorf("expected n2 to receive n1's output, got %v", n2Inputs)
	}

	// Verify final state
	execState, _ := store.GetExecution(ctx, "exec-1")
	if execState.Status != ExecutionStatusSuccess {
		t.Errorf("expected execution status SUCCESS, got %s", execState.Status)
	}
}

// MemoryStore is a simple implementation of WorkflowStore for testing.
type MemoryStore struct {
	mu         sync.Mutex
	workflows  map[string]*Workflow
	executions map[string]*Execution
	nodes      map[string][]*Node
	edges      map[string][]*Edge
	nodeStates map[string][]*NodeState
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		workflows:  make(map[string]*Workflow),
		executions: make(map[string]*Execution),
		nodes:      make(map[string][]*Node),
		edges:      make(map[string][]*Edge),
		nodeStates: make(map[string][]*NodeState),
	}
}

func (s *MemoryStore) GetWorkflow(ctx context.Context, id string) (*Workflow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if wf, ok := s.workflows[id]; ok {
		return wf, nil
	}
	return nil, errors.New("not found")
}

func (s *MemoryStore) GetExecution(ctx context.Context, id string) (*Execution, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ex, ok := s.executions[id]; ok {
		return ex, nil
	}
	return nil, errors.New("not found")
}

func (s *MemoryStore) GetNodesByWorkflow(ctx context.Context, workflowID string, version int) ([]*Node, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := workflowID + ":" + string(rune(version+'0'))
	return s.nodes[key], nil
}

func (s *MemoryStore) GetEdgesByWorkflow(ctx context.Context, workflowID string, version int) ([]*Edge, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := workflowID + ":" + string(rune(version+'0'))
	return s.edges[key], nil
}

func (s *MemoryStore) GetNodeStatesByExecution(ctx context.Context, executionID string) ([]*NodeState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.nodeStates[executionID], nil
}

func (s *MemoryStore) UpdateExecutionStatus(ctx context.Context, id string, status ExecutionStatus, errMsg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ex, ok := s.executions[id]; ok {
		ex.Status = status
		ex.Error = errMsg
		now := time.Now()
		switch status {
		case ExecutionStatusRunning:
			ex.StartedAt = &now
		case ExecutionStatusSuccess, ExecutionStatusFailed, ExecutionStatusCancelled:
			ex.CompletedAt = &now
		}
		return nil
	}
	return errors.New("not found")
}

func (s *MemoryStore) UpdateNodeStatus(ctx context.Context, id string, status NodeStatus, result []byte, errMsg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, states := range s.nodeStates {
		for _, state := range states {
			if state.ID == id {
				state.Status = status
				state.Error = errMsg
				if result != nil {
					state.Result = result
				}
				now := time.Now()
				switch status {
				case NodeStatusRunning:
					state.StartedAt = &now
				case NodeStatusSuccess, NodeStatusFailed, NodeStatusSkipped:
					state.CompletedAt = &now
				}
				return nil
			}
		}
	}
	return errors.New("not found")
}
