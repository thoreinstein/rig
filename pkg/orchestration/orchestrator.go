package orchestration

import (
	"context"
	"encoding/json"
	"fmt"
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
		if node.ID == "" {
			return errors.New("node ID must not be empty")
		}
		if _, exists := nodeMap[node.ID]; exists {
			return errors.Errorf("duplicate node ID: %s", node.ID)
		}
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

	// Validate the workflow definition before transitioning to RUNNING.
	if err := ValidateWorkflow(nodes, edges); err != nil {
		return errors.Wrap(err, "workflow validation failed")
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
	for _, node := range nodes {
		if _, ok := stateMap[node.ID]; !ok {
			return errors.Errorf("missing node state for node %s in execution %s", node.ID, executionID)
		}
	}

	// All preflight checks passed — transition to RUNNING.
	if err := o.store.UpdateExecutionStatus(ctx, executionID, ExecutionStatusRunning, ""); err != nil {
		return errors.Wrap(err, "failed to start execution")
	}

	var mu sync.Mutex
	results := make(map[string]json.RawMessage)
	launched := make(map[string]bool)
	failed := false
	var failErrs []error
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
		// Wait until there's work to dispatch or all in-flight work has completed
		for len(ready) == 0 && activeNodes > 0 {
			cond.Wait()
		}

		// Check for context cancellation after every wake-up.
		if err := ctx.Err(); err != nil && !failed {
			failed = true
			failErrs = append(failErrs, err)
		}

		// All work is done when nothing is ready and nothing is in flight
		if len(ready) == 0 && activeNodes == 0 {
			mu.Unlock()
			break
		}

		// After a failure, stop launching new nodes — drain in-flight goroutines
		if failed {
			ready = ready[:0]
			mu.Unlock()
			continue
		}

		// Take all ready nodes and run them
		currentReady := ready
		ready = []string{}
		activeNodes += len(currentReady)
		for _, id := range currentReady {
			launched[id] = true
		}
		mu.Unlock()

		for _, nodeID := range currentReady {
			go func(id string) {
				decremented := false
				defer func() {
					if r := recover(); r != nil {
						// Best-effort: mark the panicked node as FAILED.
						cleanupCtx := context.WithoutCancel(ctx)
						_ = o.store.UpdateNodeStatus(cleanupCtx, stateMap[id], NodeStatusFailed, nil, fmt.Sprintf("panic: %v", r))

						mu.Lock()
						failed = true
						failErrs = append(failErrs, errors.Errorf("panic in node %s: %v", id, r))
						if !decremented {
							activeNodes--
						}
						mu.Unlock()
						cond.Broadcast()
					}
				}()

				res, err := o.runNode(ctx, nodeMap[id], stateMap[id])

				mu.Lock()
				defer mu.Unlock()
				activeNodes--
				decremented = true

				if err != nil {
					failed = true
					failErrs = append(failErrs, err)
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
		// Use a detached context for cleanup — the original ctx may be cancelled.
		cleanupCtx := context.WithoutCancel(ctx)

		// Skip all nodes that were never executed
		skipErr := o.skipUnprocessed(cleanupCtx, launched, stateMap)

		failErr := errors.Join(failErrs...)
		failErr = errors.CombineErrors(failErr, skipErr)

		errMsg := "execution failed"
		if failErr != nil {
			errMsg = failErr.Error()
		}
		if updateErr := o.store.UpdateExecutionStatus(cleanupCtx, executionID, ExecutionStatusFailed, errMsg); updateErr != nil {
			return errors.CombineErrors(failErr, errors.Wrap(updateErr, "failed to update execution status to failed"))
		}
		return failErr
	}

	return o.store.UpdateExecutionStatus(ctx, executionID, ExecutionStatusSuccess, "")
}

// skipUnprocessed marks all nodes that were never launched as SKIPPED.
// Nodes that were launched (whether they succeeded or failed) already have
// their terminal status set by runNode, so we only touch nodes that never ran.
func (o *Orchestrator) skipUnprocessed(ctx context.Context, launched map[string]bool, stateMap map[string]string) error {
	var errs []error
	for nodeID, stateID := range stateMap {
		if launched[nodeID] {
			continue
		}
		if err := o.store.UpdateNodeStatus(ctx, stateID, NodeStatusSkipped, nil, "upstream failure"); err != nil {
			errs = append(errs, errors.Wrapf(err, "failed to skip node %s", nodeID))
		}
	}
	return errors.Join(errs...)
}

func (o *Orchestrator) runNode(ctx context.Context, node *Node, stateID string) (json.RawMessage, error) {
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
	case "panic":
		panic("simulated panic")
	default:
		execErr = errors.Errorf("unrecognized node type: %s", node.Type)
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
