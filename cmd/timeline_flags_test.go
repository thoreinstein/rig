package cmd

import (
	"testing"
)

func TestTimelineCommandNewFlags(t *testing.T) {
	cmd := timelineCmd

	expectedFlags := []struct {
		name     string
		defValue string
	}{
		{"exit-code", "-1"},
		{"min-duration", "0s"},
		{"session-id", ""},
	}

	for _, expected := range expectedFlags {
		flag := cmd.Flags().Lookup(expected.name)
		if flag == nil {
			t.Errorf("timeline command should have --%s flag", expected.name)
			continue
		}
		if flag.DefValue != expected.defValue {
			t.Errorf("--%s default = %q, want %q", expected.name, flag.DefValue, expected.defValue)
		}
	}
}
