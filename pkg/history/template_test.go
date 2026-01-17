package history

import (
	"strings"
	"testing"
	"time"
)

func TestFormatTimeline(t *testing.T) {
	commands := []Command{
		{
			Command:   "git status",
			Timestamp: time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC),
			Duration:  100, // 100ms
			ExitCode:  0,
			Directory: "/home/project",
		},
		{
			Command:   "make test",
			Timestamp: time.Date(2025, 1, 1, 10, 5, 0, 0, time.UTC),
			Duration:  5000, // 5s
			ExitCode:  1,
			Directory: "/home/project",
		},
	}

	output := FormatTimeline(commands, "PROJ-123")

	if !strings.Contains(output, "## Command Timeline - PROJ-123") {
		t.Error("Output missing header")
	}
	if !strings.Contains(output, "### 2025-01-01") {
		t.Error("Output missing date section")
	}
	if !strings.Contains(output, "git status") {
		t.Error("Output missing command 1")
	}
	if !strings.Contains(output, "make test") {
		t.Error("Output missing command 2")
	}
	// Check for visual indicators (spec says red text or icon for fail)
	if !strings.Contains(output, "❌") && !strings.Contains(output, "[Exit: 1]") { 
		// Allow either new icon or old format if I decide to stick to old, but spec said improved.
		// Spec: "Failed commands (e.g., red text or specific icon)"
		t.Error("Output missing failure indicator")
	}
	// Check for duration badge/text
	if !strings.Contains(output, "5.0s") {
		t.Error("Output missing duration")
	}
}
