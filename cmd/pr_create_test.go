package cmd

import (
	"testing"

	"thoreinstein.com/rig/pkg/config"
)

func TestRunPRCreate(t *testing.T) {
	mockClient := &mockGHClient{
		isAuthenticated: true,
	}
	cfg := &config.Config{
		GitHub: config.GitHubConfig{
			DefaultReviewers: []string{"default-rev"},
		},
	}

	tests := []struct {
		name    string
		opts    CreateOptions
		wantErr bool
	}{
		{
			name: "basic PR creation",
			opts: CreateOptions{
				Title:     "Test PR",
				NoBrowser: true,
			},
			wantErr: false,
		},
		{
			name: "PR creation with draft and reviewers",
			opts: CreateOptions{
				Title:     "Draft PR",
				Draft:     true,
				Reviewers: []string{"rev1"},
				NoBrowser: true,
			},
			wantErr: false,
		},
		{
			name: "unauthenticated",
			opts: CreateOptions{
				Title: "Fail PR",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "unauthenticated" {
				mockClient.isAuthenticated = false
			} else {
				mockClient.isAuthenticated = true
			}

			err := runPRCreate(tt.opts, mockClient, cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("runPRCreate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
