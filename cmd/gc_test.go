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

func TestDetermineCutoffs(t *testing.T) {
	cfg := &config.Config{
		Events: config.EventsConfig{
			RetentionDays: 30,
		},
		Orchestration: config.OrchestrationConfig{
			RetentionDays: 60,
		},
	}

	t.Run("events cutoff from flag", func(t *testing.T) {
		got, age, err := determineEventsCutoff(cfg, "10d")
		if err != nil {
			t.Fatal(err)
		}
		if age != "10d" {
			t.Errorf("expected 10d, got %s", age)
		}
		assertRecent(t, got, 10)
	})

	t.Run("events cutoff from config", func(t *testing.T) {
		got, age, err := determineEventsCutoff(cfg, "")
		if err != nil {
			t.Fatal(err)
		}
		if age != "30d" {
			t.Errorf("expected 30d, got %s", age)
		}
		assertRecent(t, got, 30)
	})

	t.Run("orch cutoff from flag", func(t *testing.T) {
		got, age, err := determineOrchCutoff(cfg, "10d")
		if err != nil {
			t.Fatal(err)
		}
		if age != "10d" {
			t.Errorf("expected 10d, got %s", age)
		}
		assertRecent(t, got, 10)
	})

	t.Run("orch cutoff from config", func(t *testing.T) {
		got, age, err := determineOrchCutoff(cfg, "")
		if err != nil {
			t.Fatal(err)
		}
		if age != "60d" {
			t.Errorf("expected 60d, got %s", age)
		}
		assertRecent(t, got, 60)
	})

	t.Run("events missing age", func(t *testing.T) {
		_, _, err := determineEventsCutoff(&config.Config{}, "")
		if err == nil {
			t.Error("expected error when age not specified anywhere")
		}
	})
}

func assertRecent(t *testing.T, got time.Time, days int) {
	t.Helper()
	wantTime := time.Now().AddDate(0, 0, -days)
	diff := wantTime.Sub(got)
	if diff < 0 {
		diff = -diff
	}
	if diff > time.Minute {
		t.Errorf("cutoff = %v, want approx %v (diff %v)", got, wantTime, diff)
	}
}
