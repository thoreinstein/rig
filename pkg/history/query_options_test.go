package history_test

import (
	"testing"
	"time"

	"thoreinstein.com/rig/pkg/history"
)

func TestQueryOptions_NewFields(t *testing.T) {
	// This test will fail to compile if fields are missing
	opts := history.QueryOptions{
		MinDuration: 5 * time.Second, // Should fail compilation
	}

	if opts.MinDuration != 5*time.Second {
		t.Errorf("Expected MinDuration to be 5s")
	}
}
