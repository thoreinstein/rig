package orchestration

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/cockroachdb/errors"

	"thoreinstein.com/rig/pkg/plugin"
)

// DryRunStep represents a single node in the validated execution plan.
type DryRunStep struct {
	NodeID   string            `json:"node_id"`
	NodeName string            `json:"node_name"`
	NodeType string            `json:"node_type"`
	Tier     int               `json:"tier"`
	Inputs   map[string]IOType `json:"inputs,omitempty"`
	Outputs  map[string]IOType `json:"outputs,omitempty"`
	Sources  []string          `json:"sources,omitempty"`
}

// DryRunDiagnostic represents a single validation finding.
type DryRunDiagnostic struct {
	Severity string `json:"severity"` // "error" or "warning"
	NodeID   string `json:"node_id,omitempty"`
	Field    string `json:"field,omitempty"`
	Message  string `json:"message"`
}

// DryRunResult represents the outcome of a static workflow validation.
// Valid is true when zero error-severity diagnostics were produced.
// Steps contains the ordered execution plan with tier-level concurrency info.
//
// A passing dry-run does not guarantee successful execution: dynamic factors
// (network availability, secret rotation, plugin bugs) cannot be caught statically.
type DryRunResult struct {
	Valid       bool               `json:"valid"`
	Steps       []DryRunStep       `json:"steps"`
	Diagnostics []DryRunDiagnostic `json:"diagnostics"`
}

// PluginChecker verifies plugin existence by name.
// This checks filesystem discovery only, not that the plugin advertises
// NodeCapability (which requires handshake — a side effect).
type PluginChecker interface {
	HasNodePlugin(name string) bool
	Err() error
}

// ScannerPluginChecker implements PluginChecker using a plugin.Scanner.
// Scan results are cached after the first call via sync.Once.
type ScannerPluginChecker struct {
	scanner *plugin.Scanner
	once    sync.Once
	names   map[string]bool
	scanErr error
}

// NewScannerPluginChecker creates a new ScannerPluginChecker.
func NewScannerPluginChecker(scanner *plugin.Scanner) *ScannerPluginChecker {
	return &ScannerPluginChecker{scanner: scanner}
}

// HasNodePlugin reports whether a plugin binary with the given name was found.
func (s *ScannerPluginChecker) HasNodePlugin(name string) bool {
	s.once.Do(func() {
		s.names = make(map[string]bool)
		if s.scanner == nil {
			s.scanErr = errors.New("plugin scanner is nil")
			return
		}
		res, err := s.scanner.Scan()
		if err != nil {
			s.scanErr = err
			return
		}
		for _, p := range res.Plugins {
			s.names[p.Name] = true
		}
	})
	return s.names[name]
}

// Err returns any error encountered during the plugin scan.
func (s *ScannerPluginChecker) Err() error {
	return s.scanErr
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
// It validates the DAG structure, checks that declared inputs are satisfied by
// upstream outputs, detects type mismatches, and verifies plugin/secret availability.
//
// No side effects are produced: no WorkflowStore, NodeBridge, or Dolt access.
func DryRunValidate(ctx context.Context, nodes []*Node, edges []*Edge, opts ...DryRunOption) (*DryRunResult, error) {
	options := &DryRunOptions{}
	for _, opt := range opts {
		opt(options)
	}

	result := &DryRunResult{
		Valid:       true,
		Steps:       []DryRunStep{},
		Diagnostics: []DryRunDiagnostic{},
	}

	// 1. Structural validation (cycles, dangling refs, duplicate IDs).
	if err := ValidateWorkflow(nodes, edges); err != nil {
		return nil, errors.Wrap(err, "structural validation failed")
	}

	if len(nodes) == 0 {
		return result, nil
	}

	// 2. Build adjacency list, reverse adjacency, and node map.
	adj := make(map[string][]string)
	revAdj := make(map[string][]string)
	inDegree := make(map[string]int)
	nodeMap := make(map[string]*Node)

	for _, node := range nodes {
		nodeMap[node.ID] = node
		inDegree[node.ID] = 0
	}

	for _, edge := range edges {
		adj[edge.SourceNodeID] = append(adj[edge.SourceNodeID], edge.TargetNodeID)
		revAdj[edge.TargetNodeID] = append(revAdj[edge.TargetNodeID], edge.SourceNodeID)
		inDegree[edge.TargetNodeID]++
	}

	// 3. Parse all node configs up front (capabilities + IO schemas).
	nodeIOSchemas := make(map[string]*NodeIOSchema)
	nodeCaps := make(map[string]*NodeCapabilities)

	for _, node := range nodes {
		caps, io, _, err := ParseNodeConfig(node.Config)
		if err != nil {
			addDiag(result, "error", node.ID, "", fmt.Sprintf("failed to parse config: %v", err))
			continue
		}
		nodeCaps[node.ID] = caps
		nodeIOSchemas[node.ID] = io
	}

	// 4. Tiered topological sort (Kahn's algorithm with tier tracking).
	tempInDegree := make(map[string]int)
	for k, v := range inDegree {
		tempInDegree[k] = v
	}

	var currentTier []string
	for nodeID, degree := range tempInDegree {
		if degree == 0 {
			currentTier = append(currentTier, nodeID)
		}
	}
	sort.Strings(currentTier)

	tier := 0
	for len(currentTier) > 0 {
		for _, nodeID := range currentTier {
			node := nodeMap[nodeID]
			io := nodeIOSchemas[nodeID]

			step := DryRunStep{
				NodeID:   nodeID,
				NodeName: node.Name,
				NodeType: node.Type,
				Tier:     tier,
			}

			if sources := revAdj[nodeID]; len(sources) > 0 {
				step.Sources = make([]string, len(sources))
				copy(step.Sources, sources)
				sort.Strings(step.Sources)
			}

			if io != nil {
				step.Inputs = io.Inputs
				step.Outputs = io.Outputs
			}

			result.Steps = append(result.Steps, step)
		}

		var nextTier []string
		for _, u := range currentTier {
			for _, v := range adj[u] {
				tempInDegree[v]--
				if tempInDegree[v] == 0 {
					nextTier = append(nextTier, v)
				}
			}
		}
		sort.Strings(nextTier)
		tier++
		currentTier = nextTier
	}

	// 5. Input satisfaction (AC #1) and type mismatch detection (AC #2).
	for _, node := range nodes {
		io := nodeIOSchemas[node.ID]
		if io == nil || len(io.Inputs) == 0 {
			continue
		}

		upstreams := make([]string, len(revAdj[node.ID]))
		copy(upstreams, revAdj[node.ID])
		sort.Strings(upstreams)

		// Sort input keys for deterministic diagnostics.
		inputKeys := sortedKeys(io.Inputs)

		for _, inputKey := range inputKeys {
			inputType := io.Inputs[inputKey]

			if !inputType.IsValid() {
				addDiag(result, "warning", node.ID, inputKey,
					fmt.Sprintf("unknown input type %q", string(inputType)))
			}

			if len(upstreams) == 0 {
				addDiag(result, "error", node.ID, inputKey,
					fmt.Sprintf("unsatisfied input %q: node has no upstream sources", inputKey))
				continue
			}

			satisfied := false
			anyUpstreamDeclares := false

			for _, upID := range upstreams {
				upIO := nodeIOSchemas[upID]
				if upIO == nil || upIO.Outputs == nil {
					continue
				}
				anyUpstreamDeclares = true

				if outType, ok := upIO.Outputs[inputKey]; ok {
					satisfied = true
					if outType != inputType {
						addDiag(result, "error", node.ID, inputKey,
							fmt.Sprintf("type mismatch on %q: upstream %q provides %q, expected %q",
								inputKey, upID, string(outType), string(inputType)))
					}
				}
			}

			if !anyUpstreamDeclares {
				addDiag(result, "warning", node.ID, inputKey,
					fmt.Sprintf("cannot verify input %q: no upstream node declares outputs", inputKey))
			} else if !satisfied {
				addDiag(result, "error", node.ID, inputKey,
					fmt.Sprintf("unsatisfied input %q: no upstream node provides this output", inputKey))
			}
		}
	}

	// Validate output types.
	for _, node := range nodes {
		io := nodeIOSchemas[node.ID]
		if io == nil || len(io.Outputs) == 0 {
			continue
		}
		for _, outputKey := range sortedKeys(io.Outputs) {
			if !io.Outputs[outputKey].IsValid() {
				addDiag(result, "warning", node.ID, outputKey,
					fmt.Sprintf("unknown output type %q", string(io.Outputs[outputKey])))
			}
		}
	}

	// 6. Plugin availability (AC #3).
	if options.PluginChecker != nil {
		// Prime the scanner to trigger lazy initialization.
		_ = options.PluginChecker.HasNodePlugin("")
		if scanErr := options.PluginChecker.Err(); scanErr != nil {
			addDiag(result, "warning", "", "",
				fmt.Sprintf("plugin scan failed: %v", scanErr))
		} else {
			for _, node := range nodes {
				if !options.PluginChecker.HasNodePlugin(node.Type) {
					addDiag(result, "error", node.ID, "",
						fmt.Sprintf("plugin %q not found", node.Type))
				}
			}
		}
	}

	// 7. Secret resolvability (AC #3).
	if options.SecretResolver != nil {
		for _, node := range nodes {
			caps := nodeCaps[node.ID]
			if caps == nil || len(caps.SecretsMapping) == 0 {
				continue
			}

			destKeys := sortedStringKeys(caps.SecretsMapping)
			for _, dest := range destKeys {
				src := caps.SecretsMapping[dest]
				if _, resolveErr := options.SecretResolver.Resolve(ctx, src); resolveErr != nil {
					addDiag(result, "warning", node.ID, "",
						fmt.Sprintf("secret mapping %q could not be resolved", dest))
				}
			}
		}
	}

	return result, nil
}

// addDiag appends a diagnostic to the result and updates the Valid flag.
func addDiag(result *DryRunResult, severity, nodeID, field, message string) {
	result.Diagnostics = append(result.Diagnostics, DryRunDiagnostic{
		Severity: severity,
		NodeID:   nodeID,
		Field:    field,
		Message:  message,
	})
	if severity == "error" {
		result.Valid = false
	}
}

// sortedKeys returns the keys of an IOType map in sorted order.
func sortedKeys(m map[string]IOType) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// sortedStringKeys returns the keys of a string map in sorted order.
func sortedStringKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
