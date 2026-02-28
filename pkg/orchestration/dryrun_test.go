package orchestration

import (
	"encoding/json"
	"testing"
)

type MockPluginChecker struct {
	plugins map[string]bool
}

func (m *MockPluginChecker) HasNodePlugin(name string) bool {
	return m.plugins[name]
}

func TestDryRunValidate(t *testing.T) {
	ctx := t.Context()

	tests := []struct {
		name        string
		nodes       []*Node
		edges       []*Edge
		opts        []DryRunOption
		expectSteps [][]string
		expectDiag  []Diagnostic
		expectErr   bool
	}{
		{
			name: "simple linear dag",
			nodes: []*Node{
				{ID: "n1", Type: "t1"},
				{ID: "n2", Type: "t2"},
			},
			edges: []*Edge{
				{SourceNodeID: "n1", TargetNodeID: "n2"},
			},
			expectSteps: [][]string{{"n1"}, {"n2"}},
			expectDiag:  []Diagnostic{},
		},
		{
			name: "parallel nodes",
			nodes: []*Node{
				{ID: "n1", Type: "t1"},
				{ID: "n2", Type: "t2"},
				{ID: "n3", Type: "t3"},
			},
			edges: []*Edge{
				{SourceNodeID: "n1", TargetNodeID: "n2"},
				{SourceNodeID: "n1", TargetNodeID: "n3"},
			},
			expectSteps: [][]string{{"n1"}, {"n2", "n3"}},
			expectDiag:  []Diagnostic{},
		},
		{
			name: "diamond dag",
			nodes: []*Node{
				{ID: "n1", Type: "t1"},
				{ID: "n2", Type: "t2"},
				{ID: "n3", Type: "t3"},
				{ID: "n4", Type: "t4"},
			},
			edges: []*Edge{
				{SourceNodeID: "n1", TargetNodeID: "n2"},
				{SourceNodeID: "n1", TargetNodeID: "n3"},
				{SourceNodeID: "n2", TargetNodeID: "n4"},
				{SourceNodeID: "n3", TargetNodeID: "n4"},
			},
			expectSteps: [][]string{{"n1"}, {"n2", "n3"}, {"n4"}},
			expectDiag:  []Diagnostic{},
		},
		{
			name: "type mismatch error",
			nodes: []*Node{
				{ID: "n1", Type: "t1", Config: json.RawMessage(`{"io": {"outputs": {"out1": "string"}}}`)},
				{ID: "n2", Type: "t2", Config: json.RawMessage(`{"io": {"inputs": {"out1": "number"}}}`)},
			},
			edges: []*Edge{
				{SourceNodeID: "n1", TargetNodeID: "n2"},
			},
			expectSteps: [][]string{{"n1"}, {"n2"}},
			expectDiag: []Diagnostic{
				{NodeID: "n2", Level: "ERROR", Message: `type mismatch for input "out1": upstream node "n1" provides "string", but this node expects "number"`},
			},
		},
		{
			name: "plugin missing warning",
			nodes: []*Node{
				{ID: "n1", Type: "missing-plugin"},
			},
			opts: []DryRunOption{
				WithPluginChecker(&MockPluginChecker{plugins: map[string]bool{"existing": true}}),
			},
			expectSteps: [][]string{{"n1"}},
			expectDiag: []Diagnostic{
				{NodeID: "n1", Level: "WARNING", Message: `node plugin "missing-plugin" not found or does not support node execution`},
			},
		},
		{
			name: "secret missing warning",
			nodes: []*Node{
				{ID: "n1", Type: "t1", Config: json.RawMessage(`{"capabilities": {"secrets_mapping": {"KEY": "missing-secret"}}}`)},
			},
			opts: []DryRunOption{
				WithDryRunSecretResolver(&MockSecretResolver{secrets: map[string]string{"EXISTING": "VAL"}}),
			},
			expectSteps: [][]string{{"n1"}},
			expectDiag: []Diagnostic{
				{NodeID: "n1", Level: "WARNING", Message: `secret "missing-secret" (mapped to "KEY") could not be resolved: secret not found: missing-secret`},
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

			if len(res.Steps) != len(tt.expectSteps) {
				t.Errorf("expected %d steps, got %d", len(tt.expectSteps), len(res.Steps))
			}

			for i, step := range res.Steps {
				if len(step) != len(tt.expectSteps[i]) {
					t.Errorf("step %d: expected %d nodes, got %d", i, len(tt.expectSteps[i]), len(step))
				}
			}

			if len(res.Diagnostics) != len(tt.expectDiag) {
				t.Errorf("expected %d diagnostics, got %d: %v", len(tt.expectDiag), len(res.Diagnostics), res.Diagnostics)
			}

			for i, d := range res.Diagnostics {
				if d.NodeID != tt.expectDiag[i].NodeID || d.Level != tt.expectDiag[i].Level || d.Message != tt.expectDiag[i].Message {
					t.Errorf("diagnostic %d: expected %+v, got %+v", i, tt.expectDiag[i], d)
				}
			}
		})
	}
}

func TestDryRunValidate_Secrets_Specific(t *testing.T) {
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

	expected := Diagnostic{NodeID: "n1", Level: "WARNING", Message: `secret "S2" (mapped to "K2") could not be resolved: secret not found: S2`}
	if res.Diagnostics[0] != expected {
		t.Errorf("expected %+v, got %+v", expected, res.Diagnostics[0])
	}
}
