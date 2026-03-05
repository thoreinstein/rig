package ticket

import (
	"testing"

	"thoreinstein.com/rig/pkg/config"
)

func TestLocalProvider_IsAvailable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		beadsEnabled bool
		jiraEnabled  bool
		want         bool
	}{
		{
			name:         "both disabled returns false",
			beadsEnabled: false,
			jiraEnabled:  false,
			want:         false,
		},
		{
			name:         "jira enabled returns true",
			beadsEnabled: false,
			jiraEnabled:  true,
			want:         true,
		},
		{
			name:         "beads enabled returns true",
			beadsEnabled: true,
			jiraEnabled:  false,
			want:         true,
		},
		{
			name:         "both enabled returns true",
			beadsEnabled: true,
			jiraEnabled:  true,
			want:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{
				Beads: config.BeadsConfig{Enabled: tt.beadsEnabled},
				Jira:  config.JiraConfig{Enabled: tt.jiraEnabled},
			}

			p := NewLocalProvider(cfg, t.TempDir(), false)
			got := p.IsAvailable(t.Context())
			if got != tt.want {
				t.Errorf("IsAvailable() = %v, want %v", got, tt.want)
			}
		})
	}
}
