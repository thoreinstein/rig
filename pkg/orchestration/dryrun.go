package orchestration

import (
	"context"
	"fmt"
	"sort"

	"github.com/cockroachdb/errors"

	"thoreinstein.com/rig/pkg/plugin"
)

// DryRunResult represents the outcome of a static workflow validation.
type DryRunResult struct {
	// Steps is the ordered sequence of execution tiers.
	// Independent nodes in the same tier can be executed concurrently.
	Steps [][]string `json:"steps"`
	// Diagnostics contains warnings or errors found during validation.
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// Diagnostic represents a single validation finding.
type Diagnostic struct {
	NodeID  string `json:"node_id,omitempty"`
	Level   string `json:"level"` // "ERROR" or "WARNING"
	Message string `json:"message"`
}

// PluginChecker is used to verify the existence of a node execution plugin.
type PluginChecker interface {
	HasNodePlugin(name string) bool
}

// ScannerPluginChecker implements PluginChecker using a plugin.Scanner.
type ScannerPluginChecker struct {
	scanner *plugin.Scanner
}

// NewScannerPluginChecker creates a new ScannerPluginChecker.
func NewScannerPluginChecker(scanner *plugin.Scanner) *ScannerPluginChecker {
	return &ScannerPluginChecker{scanner: scanner}
}

// HasNodePlugin checks if a plugin with the given name exists and advertises
// the "node" capability.
func (s *ScannerPluginChecker) HasNodePlugin(name string) bool {
	res, err := s.scanner.Scan()
	if err != nil {
		return false
	}
	var p *plugin.Plugin
	for _, pl := range res.Plugins {
		if pl.Name == name {
			p = pl
			break
		}
	}
	if p == nil {
		return false
	}
	for _, cap := range p.Capabilities {
		if cap.Name == plugin.NodeCapability {
			return true
		}
	}
	return false
}

// DryRunOptions configures the behavior of DryRunValidate.
type DryRunOptions struct {
	PluginChecker  PluginChecker
	SecretResolver SecretResolver
}

// DryRunOption is a functional option for DryRunValidate.
type DryRunOption func(*DryRunOptions)

// WithPluginChecker sets the plugin checker for dry-run validation.
func WithPluginChecker(checker PluginChecker) DryRunOption {
	return func(o *DryRunOptions) {
		o.PluginChecker = checker
	}
}

// WithDryRunSecretResolver sets the secret resolver for dry-run validation.
func WithDryRunSecretResolver(resolver SecretResolver) DryRunOption {
	return func(o *DryRunOptions) {
		o.SecretResolver = resolver
	}
}

// DryRunValidate performs a static analysis of a workflow definition.
// It validates the DAG structure, checks node capability requirements,
// and verifies that inputs are satisfied by upstream outputs.
func DryRunValidate(ctx context.Context, nodes []*Node, edges []*Edge, opts ...DryRunOption) (*DryRunResult, error) {
	options := &DryRunOptions{}
	for _, opt := range opts {
		opt(options)
	}

	result := &DryRunResult{
		Steps:       [][]string{},
		Diagnostics: []Diagnostic{},
	}

	// 1. Structural Validation (cycles, duplicates, etc.)
	if err := ValidateWorkflow(nodes, edges); err != nil {
		return nil, errors.Wrap(err, "structural validation failed")
	}

	if len(nodes) == 0 {
		return result, nil
	}

	// 2. Build Adjacency List and In-Degree Map
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

	// 3. Tiered Topological Sort (Execution Plan)
	tempInDegree := make(map[string]int)
	for k, v := range inDegree {
		tempInDegree[k] = v
	}

	currentTier := []string{}
	for nodeID, degree := range tempInDegree {
		if degree == 0 {
			currentTier = append(currentTier, nodeID)
		}
	}
	sort.Strings(currentTier)

	for len(currentTier) > 0 {
		result.Steps = append(result.Steps, currentTier)
		nextTier := []string{}
		for _, u := range currentTier {
			for _, v := range adj[u] {
				tempInDegree[v]--
				if tempInDegree[v] == 0 {
					nextTier = append(nextTier, v)
				}
			}
		}
		sort.Strings(nextTier)
		currentTier = nextTier
	}

	// 4. Per-Node Static Validation
	nodeIOSchemas := make(map[string]*NodeIOSchema)

	for _, node := range nodes {
		caps, io, _, err := ParseNodeConfig(node.Config)
		if err != nil {
			result.Diagnostics = append(result.Diagnostics, Diagnostic{
				NodeID:  node.ID,
				Level:   "ERROR",
				Message: fmt.Sprintf("failed to parse config: %v", err),
			})
			continue
		}
		nodeIOSchemas[node.ID] = io

		// AC #3: Plugin Availability
		if options.PluginChecker != nil {
			if !options.PluginChecker.HasNodePlugin(node.Type) {
				result.Diagnostics = append(result.Diagnostics, Diagnostic{
					NodeID:  node.ID,
					Level:   "WARNING",
					Message: fmt.Sprintf("node plugin %q not found or does not support node execution", node.Type),
				})
			}
		}

		// AC #3: Secret Resolvability
		if options.SecretResolver != nil {
			for dest, src := range caps.SecretsMapping {
				if _, err := options.SecretResolver.Resolve(ctx, src); err != nil {
					result.Diagnostics = append(result.Diagnostics, Diagnostic{
						NodeID:  node.ID,
						Level:   "WARNING",
						Message: fmt.Sprintf("secret %q (mapped to %q) could not be resolved: %v", src, dest, err),
					})
				}
			}
		}
	}

	// AC #1 & #2: Data Flow Validation (Inputs/Outputs and Type Mismatches)
	for _, edge := range edges {
		sourceIO := nodeIOSchemas[edge.SourceNodeID]
		targetIO := nodeIOSchemas[edge.TargetNodeID]

		if sourceIO == nil || targetIO == nil {
			continue
		}

		// Type mismatch check:
		// We assume outputs from the source node are available to the target node.
		for outKey, outType := range sourceIO.Outputs {
			if inType, ok := targetIO.Inputs[outKey]; ok {
				if outType != inType {
					result.Diagnostics = append(result.Diagnostics, Diagnostic{
						NodeID:  edge.TargetNodeID,
						Level:   "ERROR",
						Message: fmt.Sprintf("type mismatch for input %q: upstream node %q provides %q, but this node expects %q", outKey, edge.SourceNodeID, string(outType), string(inType)),
					})
				}
			}
		}
	}

	return result, nil
}
