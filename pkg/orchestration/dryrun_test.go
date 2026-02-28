package orchestration

import (
	"encoding/json"
	"testing"
)

type mockPluginChecker struct {
	plugins map[string]bool
}

func (m *mockPluginChecker) HasNodePlugin(name string) bool {
	return m.plugins[name]
}

func TestDryRunValidate(t *testing.T) {
	ctx := t.Context()

	tests := []struct {
		name        string
		nodes       []*Node
		edges       []*Edge
		opts        []DryRunOption
		expectValid bool
		expectSteps []DryRunStep
		expectDiag  []DryRunDiagnostic
		expectErr   bool
	}{
		{
			name:        "empty workflow",
			nodes:       nil,
			edges:       nil,
			expectValid: true,
			expectSteps: []DryRunStep{},
			expectDiag:  []DryRunDiagnostic{},
		},
		{
			name: "simple linear dag without schemas",
			nodes: []*Node{
				{ID: "a", Name: "Node A", Type: "t1"},
				{ID: "b", Name: "Node B", Type: "t2"},
				{ID: "c", Name: "Node C", Type: "t3"},
			},
			edges: []*Edge{
				{SourceNodeID: "a", TargetNodeID: "b"},
				{SourceNodeID: "b", TargetNodeID: "c"},
			},
			expectValid: true,
			expectSteps: []DryRunStep{
				{NodeID: "a", NodeName: "Node A", NodeType: "t1", Tier: 0},
				{NodeID: "b", NodeName: "Node B", NodeType: "t2", Tier: 1, Sources: []string{"a"}},
				{NodeID: "c", NodeName: "Node C", NodeType: "t3", Tier: 2, Sources: []string{"b"}},
			},
			expectDiag: []DryRunDiagnostic{},
		},
		{
			name: "diamond dag with matching IO",
			nodes: []*Node{
				{ID: "a", Name: "Root", Type: "t1", Config: json.RawMessage(`{"plugin": {}, "io": {"outputs": {"x": "string"}}}`)},
				{ID: "b", Name: "Left", Type: "t2", Config: json.RawMessage(`{"plugin": {}, "io": {"inputs": {"x": "string"}, "outputs": {"y": "number"}}}`)},
				{ID: "c", Name: "Right", Type: "t3", Config: json.RawMessage(`{"plugin": {}, "io": {"inputs": {"x": "string"}, "outputs": {"z": "number"}}}`)},
				{ID: "d", Name: "Sink", Type: "t4", Config: json.RawMessage(`{"plugin": {}, "io": {"inputs": {"y": "number", "z": "number"}}}`)},
			},
			edges: []*Edge{
				{SourceNodeID: "a", TargetNodeID: "b"},
				{SourceNodeID: "a", TargetNodeID: "c"},
				{SourceNodeID: "b", TargetNodeID: "d"},
				{SourceNodeID: "c", TargetNodeID: "d"},
			},
			expectValid: true,
			expectSteps: []DryRunStep{
				{NodeID: "a", NodeName: "Root", NodeType: "t1", Tier: 0, Outputs: map[string]IOType{"x": "string"}},
				{NodeID: "b", NodeName: "Left", NodeType: "t2", Tier: 1, Sources: []string{"a"}, Inputs: map[string]IOType{"x": "string"}, Outputs: map[string]IOType{"y": "number"}},
				{NodeID: "c", NodeName: "Right", NodeType: "t3", Tier: 1, Sources: []string{"a"}, Inputs: map[string]IOType{"x": "string"}, Outputs: map[string]IOType{"z": "number"}},
				{NodeID: "d", NodeName: "Sink", NodeType: "t4", Tier: 2, Sources: []string{"b", "c"}, Inputs: map[string]IOType{"y": "number", "z": "number"}},
			},
			expectDiag: []DryRunDiagnostic{},
		},
		{
			name: "type mismatch error",
			nodes: []*Node{
				{ID: "n1", Type: "t1", Config: json.RawMessage(`{"plugin": {}, "io": {"outputs": {"val": "string"}}}`)},
				{ID: "n2", Type: "t2", Config: json.RawMessage(`{"plugin": {}, "io": {"inputs": {"val": "number"}}}`)},
			},
			edges: []*Edge{
				{SourceNodeID: "n1", TargetNodeID: "n2"},
			},
			expectValid: false,
			expectSteps: []DryRunStep{
				{NodeID: "n1", NodeType: "t1", Tier: 0, Outputs: map[string]IOType{"val": "string"}},
				{NodeID: "n2", NodeType: "t2", Tier: 1, Sources: []string{"n1"}, Inputs: map[string]IOType{"val": "number"}},
			},
			expectDiag: []DryRunDiagnostic{
				{Severity: "error", NodeID: "n2", Field: "val", Message: `type mismatch on "val": upstream "n1" provides "string", expected "number"`},
			},
		},
		{
			name: "unsatisfied input key",
			nodes: []*Node{
				{ID: "n1", Type: "t1", Config: json.RawMessage(`{"plugin": {}, "io": {"outputs": {"y": "string"}}}`)},
				{ID: "n2", Type: "t2", Config: json.RawMessage(`{"plugin": {}, "io": {"inputs": {"x": "string"}}}`)},
			},
			edges: []*Edge{
				{SourceNodeID: "n1", TargetNodeID: "n2"},
			},
			expectValid: false,
			expectSteps: []DryRunStep{
				{NodeID: "n1", NodeType: "t1", Tier: 0, Outputs: map[string]IOType{"y": "string"}},
				{NodeID: "n2", NodeType: "t2", Tier: 1, Sources: []string{"n1"}, Inputs: map[string]IOType{"x": "string"}},
			},
			expectDiag: []DryRunDiagnostic{
				{Severity: "error", NodeID: "n2", Field: "x", Message: `unsatisfied input "x": no upstream node provides this output`},
			},
		},
		{
			name: "root node with declared inputs",
			nodes: []*Node{
				{ID: "n1", Type: "t1", Config: json.RawMessage(`{"plugin": {}, "io": {"inputs": {"prompt": "string"}}}`)},
			},
			expectValid: false,
			expectSteps: []DryRunStep{
				{NodeID: "n1", NodeType: "t1", Tier: 0, Inputs: map[string]IOType{"prompt": "string"}},
			},
			expectDiag: []DryRunDiagnostic{
				{Severity: "error", NodeID: "n1", Field: "prompt", Message: `unsatisfied input "prompt": node has no upstream sources`},
			},
		},
		{
			name: "undeclared upstream outputs (warning)",
			nodes: []*Node{
				{ID: "n1", Type: "t1"}, // no IO schema
				{ID: "n2", Type: "t2", Config: json.RawMessage(`{"plugin": {}, "io": {"inputs": {"x": "string"}}}`)},
			},
			edges: []*Edge{
				{SourceNodeID: "n1", TargetNodeID: "n2"},
			},
			expectValid: true,
			expectSteps: []DryRunStep{
				{NodeID: "n1", NodeType: "t1", Tier: 0},
				{NodeID: "n2", NodeType: "t2", Tier: 1, Sources: []string{"n1"}, Inputs: map[string]IOType{"x": "string"}},
			},
			expectDiag: []DryRunDiagnostic{
				{Severity: "warning", NodeID: "n2", Field: "x", Message: `cannot verify input "x": no upstream node declares outputs`},
			},
		},
		{
			name: "multiple upstreams satisfy input",
			nodes: []*Node{
				{ID: "n1", Type: "t1", Config: json.RawMessage(`{"plugin": {}, "io": {"outputs": {"x": "string"}}}`)},
				{ID: "n2", Type: "t2", Config: json.RawMessage(`{"plugin": {}, "io": {"outputs": {"x": "string"}}}`)},
				{ID: "n3", Type: "t3", Config: json.RawMessage(`{"plugin": {}, "io": {"inputs": {"x": "string"}}}`)},
			},
			edges: []*Edge{
				{SourceNodeID: "n1", TargetNodeID: "n3"},
				{SourceNodeID: "n2", TargetNodeID: "n3"},
			},
			expectValid: true,
			expectSteps: []DryRunStep{
				{NodeID: "n1", NodeType: "t1", Tier: 0, Outputs: map[string]IOType{"x": "string"}},
				{NodeID: "n2", NodeType: "t2", Tier: 0, Outputs: map[string]IOType{"x": "string"}},
				{NodeID: "n3", NodeType: "t3", Tier: 1, Sources: []string{"n1", "n2"}, Inputs: map[string]IOType{"x": "string"}},
			},
			expectDiag: []DryRunDiagnostic{},
		},
		{
			name: "plugin missing error",
			nodes: []*Node{
				{ID: "n1", Type: "missing-plugin"},
			},
			opts: []DryRunOption{
				WithPluginChecker(&mockPluginChecker{plugins: map[string]bool{"existing": true}}),
			},
			expectValid: false,
			expectSteps: []DryRunStep{
				{NodeID: "n1", NodeType: "missing-plugin", Tier: 0},
			},
			expectDiag: []DryRunDiagnostic{
				{Severity: "error", NodeID: "n1", Message: `plugin "missing-plugin" not found`},
			},
		},
		{
			name: "plugin found (no diagnostic)",
			nodes: []*Node{
				{ID: "n1", Type: "my-plugin"},
			},
			opts: []DryRunOption{
				WithPluginChecker(&mockPluginChecker{plugins: map[string]bool{"my-plugin": true}}),
			},
			expectValid: true,
			expectSteps: []DryRunStep{
				{NodeID: "n1", NodeType: "my-plugin", Tier: 0},
			},
			expectDiag: []DryRunDiagnostic{},
		},
		{
			name: "secret missing warning (sanitized)",
			nodes: []*Node{
				{ID: "n1", Type: "t1", Config: json.RawMessage(`{"capabilities": {"secrets_mapping": {"KEY": "missing-secret"}}}`)},
			},
			opts: []DryRunOption{
				WithDryRunSecretResolver(&MockSecretResolver{secrets: map[string]string{"EXISTING": "VAL"}}),
			},
			expectValid: true,
			expectSteps: []DryRunStep{
				{NodeID: "n1", NodeType: "t1", Tier: 0},
			},
			expectDiag: []DryRunDiagnostic{
				{Severity: "warning", NodeID: "n1", Message: `secret mapping "KEY" could not be resolved`},
			},
		},
		{
			name: "circular dependency error",
			nodes: []*Node{
				{ID: "n1", Type: "t1"},
				{ID: "n2", Type: "t2"},
			},
			edges: []*Edge{
				{SourceNodeID: "n1", TargetNodeID: "n2"},
				{SourceNodeID: "n2", TargetNodeID: "n1"},
			},
			expectErr: true,
		},
		{
			name: "mixed schema and no-schema nodes",
			nodes: []*Node{
				{ID: "n1", Type: "t1", Config: json.RawMessage(`{"plugin": {}, "io": {"outputs": {"x": "string"}}}`)},
				{ID: "n2", Type: "t2"}, // no schema — pass-through
				{ID: "n3", Type: "t3", Config: json.RawMessage(`{"plugin": {}, "io": {"inputs": {"x": "string"}}}`)},
			},
			edges: []*Edge{
				{SourceNodeID: "n1", TargetNodeID: "n2"},
				{SourceNodeID: "n2", TargetNodeID: "n3"},
			},
			expectValid: true,
			expectSteps: []DryRunStep{
				{NodeID: "n1", NodeType: "t1", Tier: 0, Outputs: map[string]IOType{"x": "string"}},
				{NodeID: "n2", NodeType: "t2", Tier: 1, Sources: []string{"n1"}},
				{NodeID: "n3", NodeType: "t3", Tier: 2, Sources: []string{"n2"}, Inputs: map[string]IOType{"x": "string"}},
			},
			expectDiag: []DryRunDiagnostic{
				// n2 is the only upstream and has no schema → warning
				{Severity: "warning", NodeID: "n3", Field: "x", Message: `cannot verify input "x": no upstream node declares outputs`},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := DryRunValidate(ctx, tt.nodes, tt.edges, tt.opts...)

			if tt.expectErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if res.Valid != tt.expectValid {
				t.Errorf("expected Valid=%v, got Valid=%v (diagnostics: %+v)", tt.expectValid, res.Valid, res.Diagnostics)
			}

			if len(res.Steps) != len(tt.expectSteps) {
				t.Fatalf("expected %d steps, got %d: %+v", len(tt.expectSteps), len(res.Steps), res.Steps)
			}

			for i, step := range res.Steps {
				exp := tt.expectSteps[i]
				if step.NodeID != exp.NodeID {
					t.Errorf("step %d: expected NodeID=%q, got %q", i, exp.NodeID, step.NodeID)
				}
				if step.NodeType != exp.NodeType {
					t.Errorf("step %d: expected NodeType=%q, got %q", i, exp.NodeType, step.NodeType)
				}
				if step.Tier != exp.Tier {
					t.Errorf("step %d: expected Tier=%d, got %d", i, exp.Tier, step.Tier)
				}
				assertStringSlice(t, i, "Sources", exp.Sources, step.Sources)
			}

			if len(res.Diagnostics) != len(tt.expectDiag) {
				t.Fatalf("expected %d diagnostics, got %d: %+v", len(tt.expectDiag), len(res.Diagnostics), res.Diagnostics)
			}

			for i, d := range res.Diagnostics {
				exp := tt.expectDiag[i]
				if d.Severity != exp.Severity {
					t.Errorf("diag %d: expected Severity=%q, got %q", i, exp.Severity, d.Severity)
				}
				if d.NodeID != exp.NodeID {
					t.Errorf("diag %d: expected NodeID=%q, got %q", i, exp.NodeID, d.NodeID)
				}
				if d.Field != exp.Field {
					t.Errorf("diag %d: expected Field=%q, got %q", i, exp.Field, d.Field)
				}
				if d.Message != exp.Message {
					t.Errorf("diag %d: expected Message=%q, got %q", i, exp.Message, d.Message)
				}
			}
		})
	}
}

func TestDryRunValidate_Secrets_Deterministic(t *testing.T) {
	ctx := t.Context()
	resolver := &MockSecretResolver{secrets: map[string]string{"S1": "V1"}}

	nodes := []*Node{
		{ID: "n1", Type: "t1", Config: json.RawMessage(`{"capabilities": {"secrets_mapping": {"K1": "S1", "K2": "S2"}}}`)},
	}

	res, err := DryRunValidate(ctx, nodes, nil, WithDryRunSecretResolver(resolver))
	if err != nil {
		t.Fatal(err)
	}

	if len(res.Diagnostics) != 1 {
		t.Fatalf("expected 1 diagnostic (missing S2), got %d: %+v", len(res.Diagnostics), res.Diagnostics)
	}

	expected := DryRunDiagnostic{Severity: "warning", NodeID: "n1", Message: `secret mapping "K2" could not be resolved`}
	if res.Diagnostics[0] != expected {
		t.Errorf("expected %+v, got %+v", expected, res.Diagnostics[0])
	}
}

func assertStringSlice(t *testing.T, stepIdx int, field string, expected, actual []string) {
	t.Helper()
	if len(expected) != len(actual) {
		t.Errorf("step %d %s: expected %v, got %v", stepIdx, field, expected, actual)
		return
	}
	for j, v := range expected {
		if actual[j] != v {
			t.Errorf("step %d %s[%d]: expected %q, got %q", stepIdx, field, j, v, actual[j])
		}
	}
}
