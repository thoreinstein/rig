package orchestration

import (
	"testing"
)

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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateWorkflow(tt.nodes, tt.edges); (err != nil) != tt.wantErr {
				t.Errorf("ValidateWorkflow() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
