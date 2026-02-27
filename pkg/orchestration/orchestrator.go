package orchestration

import (
	"context"

	"github.com/cockroachdb/errors"
)

// Orchestrator handles the execution logic for DAG-based workflows.
type Orchestrator struct {
	dm *DatabaseManager
}

// NewOrchestrator creates a new Orchestrator with the given DatabaseManager.
func NewOrchestrator(dm *DatabaseManager) *Orchestrator {
	return &Orchestrator{dm: dm}
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
		// Verify source and target nodes exist in the nodes list
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
func (o *Orchestrator) Execute(ctx context.Context, executionID string) error {
	// TODO: Implement execution loop in Phase 2
	return errors.New("not implemented")
}
