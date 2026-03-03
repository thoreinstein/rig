package cmd

import (
	"testing"
	"time"

	"thoreinstein.com/rig/pkg/config"
)

func TestParseAge(t *testing.T) {
	tests := []struct {
		input   string
		want    int
		wantErr bool
	}{
		{"30d", 30, false},
		{"1d", 1, false},
		{"90d ", 90, false},
		{" 7d", 7, false},
		{"30", 0, true},
		{"30h", 0, true},
		{"0d", 0, true},
		{"-1d", 0, true},
		{"abc", 0, true},
	}

	for _, tt := range tests {
		got, err := parseAge(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("parseAge(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("parseAge(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestDetermineCutoff(t *testing.T) {
	cfg := &config.Config{
		Events: config.EventsConfig{
			RetentionDays: 30,
		},
		Orchestration: config.OrchestrationConfig{
			RetentionDays: 60,
		},
	}

	tests := []struct {
		name    string
		ageFlag string
		target  string
		wantMin int // minimum days old (approx)
		wantErr bool
	}{
		{
			name:    "explicit flag",
			ageFlag: "10d",
			target:  "all",
			wantMin: 10,
		},
		{
			name:    "events config",
			ageFlag: "",
			target:  "events",
			wantMin: 30,
		},
		{
			name:    "orchestration config",
			ageFlag: "",
			target:  "orchestration",
			wantMin: 60,
		},
		{
			name:    "all targets (uses minimum)",
			ageFlag: "",
			target:  "all",
			wantMin: 30,
		},
		{
			name:    "no age specified",
			ageFlag: "",
			target:  "events",
			wantErr: true,
		},
	}

	// For "no age specified" test, we need a config with 0 retention
	emptyCfg := &config.Config{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := cfg
			if tt.wantErr && tt.ageFlag == "" {
				c = emptyCfg
			}

			// Mock global flag
			gcTarget = tt.target

			got, err := determineCutoff(c, tt.ageFlag)
			if (err != nil) != tt.wantErr {
				t.Errorf("determineCutoff() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Check if cutoff is roughly correct (within 1 minute)
				wantTime := time.Now().AddDate(0, 0, -tt.wantMin)
				diff := wantTime.Sub(got)
				if diff < 0 {
					diff = -diff
				}
				if diff > time.Minute {
					t.Errorf("determineCutoff() = %v, want approx %v (diff %v)", got, wantTime, diff)
				}
			}
		})
	}
}
