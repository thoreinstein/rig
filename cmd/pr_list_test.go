package cmd

import (
	"context"
	"testing"

	"thoreinstein.com/rig/pkg/config"
	"thoreinstein.com/rig/pkg/github"
)

func TestRunPRList(t *testing.T) {
	mockClient := &mockGHClient{
		isAuthenticated: true,
	}
	cfg := &config.Config{}

	tests := []struct {
		name    string
		opts    ListOptions
		prs     []github.PRInfo
		wantErr bool
	}{
		{
			name: "list open PRs",
			opts: ListOptions{State: "open"},
			prs: []github.PRInfo{
				{Number: 1, Title: "PR 1", State: "open"},
			},
			wantErr: false,
		},
		{
			name: "list no PRs",
			opts: ListOptions{State: "open"},
			prs:  []github.PRInfo{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient.listPRsFunc = func(ctx context.Context, state, author string) ([]github.PRInfo, error) {
				return tt.prs, nil
			}

			err := runPRList(tt.opts, mockClient, cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("runPRList() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
