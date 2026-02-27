package orchestration

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/cockroachdb/errors"
)

// WorkflowStore defines the database operations required by the Orchestrator.
type WorkflowStore interface {
	GetWorkflow(ctx context.Context, id string) (*Workflow, error)
	GetExecution(ctx context.Context, id string) (*Execution, error)
	GetNodesByWorkflow(ctx context.Context, workflowID string, version int) ([]*Node, error)
	GetEdgesByWorkflow(ctx context.Context, workflowID string, version int) ([]*Edge, error)
	GetNodeStatesByExecution(ctx context.Context, executionID string) ([]*NodeState, error)
	UpdateExecutionStatus(ctx context.Context, id string, status ExecutionStatus, errMsg string) error
	UpdateNodeStatus(ctx context.Context, id string, status NodeStatus, result []byte, errMsg string) error
}

// Orchestrator handles the execution logic for DAG-based workflows.
type Orchestrator struct {
	store WorkflowStore
}

// NewOrchestrator creates a new Orchestrator with the given WorkflowStore.
func NewOrchestrator(store WorkflowStore) *Orchestrator {
	return &Orchestrator{store: store}
}

// ValidateWorkflow checks a workflow definition for structural integrity,
// specifically identifying and rejecting circular dependencies.
func ValidateWorkflow(nodes []*Node, edges []*Edge) error {
	if len(nodes) == 0 {
		return nil
	}

	// Build adjacency list and in-degree map
	adj := make(map[string][]string)
	inDegree := make(map[string]int)
	nodeMap := make(map[string]*Node)

	for _, node := range nodes {
		nodeMap[node.ID] = node
		inDegree[node.ID] = 0
	}

	for _, edge := range edges {
		if _, ok := nodeMap[edge.SourceNodeID]; !ok {
			return errors.Errorf("edge references non-existent source node: %s", edge.SourceNodeID)
		}
		if _, ok := nodeMap[edge.TargetNodeID]; !ok {
			return errors.Errorf("edge references non-existent target node: %s", edge.TargetNodeID)
		}

		adj[edge.SourceNodeID] = append(adj[edge.SourceNodeID], edge.TargetNodeID)
		inDegree[edge.TargetNodeID]++
	}

	// Kahn's algorithm for cycle detection
	var queue []string
	for nodeID, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, nodeID)
		}
	}

	count := 0
	for len(queue) > 0 {
		u := queue[0]
		queue = queue[1:]
		count++

		for _, v := range adj[u] {
			inDegree[v]--
			if inDegree[v] == 0 {
				queue = append(queue, v)
			}
		}
	}

	if count != len(nodes) {
		return errors.New("workflow contains circular dependencies")
	}

	return nil
}

// Execute starts the execution of a workflow run identified by executionID.
// It executes independent nodes concurrently and respects dependencies.
func (o *Orchestrator) Execute(ctx context.Context, executionID string) error {
	execution, err := o.store.GetExecution(ctx, executionID)
	if err != nil {
		return errors.Wrap(err, "failed to get execution")
	}

	nodes, err := o.store.GetNodesByWorkflow(ctx, execution.WorkflowID, execution.WorkflowVersion)
	if err != nil {
		return errors.Wrap(err, "failed to get nodes")
	}
	edges, err := o.store.GetEdgesByWorkflow(ctx, execution.WorkflowID, execution.WorkflowVersion)
	if err != nil {
		return errors.Wrap(err, "failed to get edges")
	}

	if err := o.store.UpdateExecutionStatus(ctx, executionID, ExecutionStatusRunning, ""); err != nil {
		return errors.Wrap(err, "failed to start execution")
	}

	// Build graph
	adj := make(map[string][]string)
	inDegree := make(map[string]int)
	nodeMap := make(map[string]*Node)
	for _, node := range nodes {
		nodeMap[node.ID] = node
		inDegree[node.ID] = 0
	}
	for _, edge := range edges {
		adj[edge.SourceNodeID] = append(adj[edge.SourceNodeID], edge.TargetNodeID)
		inDegree[edge.TargetNodeID]++
	}

	states, err := o.store.GetNodeStatesByExecution(ctx, executionID)
	if err != nil {
		return errors.Wrap(err, "failed to get node states")
	}
	stateMap := make(map[string]string)
	for _, s := range states {
		stateMap[s.NodeID] = s.ID
	}

	var mu sync.Mutex
	results := make(map[string]json.RawMessage)
	failed := false
	var failErr error
	activeNodes := 0
	cond := sync.NewCond(&mu)

	// topological queue
	ready := []string{}
	for nodeID, degree := range inDegree {
		if degree == 0 {
			ready = append(ready, nodeID)
		}
	}

	for {
		mu.Lock()
		// Wait if no nodes are ready and some are still running
		for len(ready) == 0 && activeNodes > 0 && !failed {
			cond.Wait()
		}

		// Check if we are done or failed
		if (len(ready) == 0 && activeNodes == 0) || failed {
			// If failed, mark remaining ready and downstream as skipped
			if failed {
				o.skipRemaining(ctx, ready, adj, inDegree, stateMap)
			}
			mu.Unlock()
			break
		}

		// Take all ready nodes and run them
		currentReady := ready
		ready = []string{}
		activeNodes += len(currentReady)
		mu.Unlock()

		for _, nodeID := range currentReady {
			go func(id string) {
				res, err := o.runNode(ctx, nodeMap[id], stateMap[id], results)

				mu.Lock()
				defer mu.Unlock()
				activeNodes--

				if err != nil {
					failed = true
					failErr = err
				} else {
					results[id] = res
					// Success: check downstream nodes
					for _, v := range adj[id] {
						inDegree[v]--
						if inDegree[v] == 0 {
							ready = append(ready, v)
						}
					}
				}
				cond.Broadcast()
			}(nodeID)
		}
	}

	if failed {
		errMsg := "execution failed"
		if failErr != nil {
			errMsg = failErr.Error()
		}
		_ = o.store.UpdateExecutionStatus(ctx, executionID, ExecutionStatusFailed, errMsg)
		return failErr
	}

	return o.store.UpdateExecutionStatus(ctx, executionID, ExecutionStatusSuccess, "")
}

func (o *Orchestrator) skipRemaining(ctx context.Context, ready []string, adj map[string][]string, inDegree map[string]int, stateMap map[string]string) {
	queue := ready
	visited := make(map[string]bool)
	for _, id := range queue {
		visited[id] = true
	}

	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]

		_ = o.store.UpdateNodeStatus(ctx, stateMap[id], NodeStatusSkipped, nil, "upstream failure")

		for _, v := range adj[id] {
			if !visited[v] {
				visited[v] = true
				queue = append(queue, v)
			}
		}
	}
}

func (o *Orchestrator) runNode(ctx context.Context, node *Node, stateID string, results map[string]json.RawMessage) (json.RawMessage, error) {
	if err := o.store.UpdateNodeStatus(ctx, stateID, NodeStatusRunning, nil, ""); err != nil {
		return nil, errors.Wrap(err, "failed to update node status to running")
	}

	// Simulated execution based on node type
	var result json.RawMessage
	var execErr error

	switch node.Type {
	case "hello":
		result = json.RawMessage(`{"output": "Hello"}`)
	case "world":
		result = json.RawMessage(`{"output": "World"}`)
	case "fail":
		execErr = errors.New("simulated failure")
	default:
		// Default success with empty result
		result = json.RawMessage(`{}`)
	}

	status := NodeStatusSuccess
	var errMsg string
	if execErr != nil {
		status = NodeStatusFailed
		errMsg = execErr.Error()
	}

	if err := o.store.UpdateNodeStatus(ctx, stateID, status, result, errMsg); err != nil {
		return nil, errors.Wrap(err, "failed to update node status to final")
	}

	return result, execErr
}
