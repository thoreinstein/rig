package history

import (
	"testing"
	"time"
)

func TestBuildZshHistdbQuery_NewFilters(t *testing.T) {
	dm := NewDatabaseManager("", false)

	// Test MinDuration
	t.Run("MinDuration", func(t *testing.T) {
		options := QueryOptions{
			MinDuration: 5 * time.Second,
		}
		query, args := dm.buildZshHistdbQuery(options)

		expected := "c.duration >="
		if !containsString(query, expected) {
			t.Errorf("query missing duration filter %q: %s", expected, query)
		}

		// Assuming seconds for zsh-histdb
		foundArg := false
		for _, arg := range args {
			if val, ok := arg.(int64); ok && val == 5 {
				foundArg = true
			}
		}
		
		if !foundArg {
			// Try checking if it was passed as int
			for _, arg := range args {
				if val, ok := arg.(int); ok && val == 5 {
					foundArg = true
				}
			}
		}

		if !foundArg {
			t.Errorf("Expected arg 5 (seconds) not found in %v", args)
		}
	})

	// Test SessionID
	t.Run("SessionID", func(t *testing.T) {
		options := QueryOptions{
			SessionID: "session_123",
		}
		query, args := dm.buildZshHistdbQuery(options)

		expected := "s.session ="
		if !containsString(query, expected) {
			t.Errorf("query missing session_id filter %q: %s", expected, query)
		}

		foundArg := false
		for _, arg := range args {
			if str, ok := arg.(string); ok && str == "session_123" {
				foundArg = true
			}
		}
		if !foundArg {
			t.Errorf("Expected arg 'session_123' not found in %v", args)
		}
	})
}

func TestBuildAtuinQuery_NewFilters(t *testing.T) {
	dm := NewDatabaseManager("", false)

	// Test MinDuration
	t.Run("MinDuration", func(t *testing.T) {
		options := QueryOptions{
			MinDuration: 2 * time.Second,
		}
		query, args := dm.buildAtuinQuery(options)

		expected := "duration >="
		if !containsString(query, expected) {
			t.Errorf("query missing duration filter %q: %s", expected, query)
		}
		
		// Atuin stores duration in nanoseconds
		expectedNs := int64(2 * time.Second) // 2e9
		foundArg := false
		for _, arg := range args {
			if val, ok := arg.(int64); ok && val == expectedNs {
				foundArg = true
			}
		}
		if !foundArg {
			t.Errorf("Expected arg %d (ns) not found in %v", expectedNs, args)
		}
	})

	// Test SessionID
	t.Run("SessionID", func(t *testing.T) {
		options := QueryOptions{
			SessionID: "uuid-123",
		}
		query, args := dm.buildAtuinQuery(options)

		expected := "session ="
		if !containsString(query, expected) {
			t.Errorf("query missing session_id filter %q: %s", expected, query)
		}
		
		foundArg := false
		for _, arg := range args {
			if str, ok := arg.(string); ok && str == "uuid-123" {
				foundArg = true
			}
		}
		if !foundArg {
			t.Errorf("Expected arg 'uuid-123' not found in %v", args)
		}
	})
}