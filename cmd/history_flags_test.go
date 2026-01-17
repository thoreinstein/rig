package cmd

import (
	"testing"
)

func TestHistoryQueryCommandNewFlags(t *testing.T) {
	cmd := historyQueryCmd

	expectedFlags := []struct {
		name     string
		defValue string
	}{
		{"exit-code", "-1"},
		{"min-duration", "0s"},
		{"session-id", ""},
		{"ticket", ""},
	}

	for _, expected := range expectedFlags {
		flag := cmd.Flags().Lookup(expected.name)
		if flag == nil {
			t.Errorf("history query command should have --%s flag", expected.name)
			continue
		}
		if flag.DefValue != expected.defValue {
			t.Errorf("--%s default = %q, want %q", expected.name, flag.DefValue, expected.defValue)
		}
	}
}
