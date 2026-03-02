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

func TestFormatUnifiedTimeline(t *testing.T) {
	now := time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC)
	entries := []UnifiedEntry{
		UnifiedEntryFromCommand(Command{
			Command:   "ls -la",
			Timestamp: now.Add(1 * time.Minute),
			ExitCode:  0,
		}),
		UnifiedEntryFromEvent(now, "preflight", "STARTED", "Starting checks", "wf123"),
		UnifiedEntryFromEvent(now.Add(2*time.Minute), "preflight", "COMPLETED", "", "wf123"),
	}

	output := FormatUnifiedTimeline(entries, "PROJ-456")

	if !strings.Contains(output, "## Workflow Timeline - PROJ-456") {
		t.Error("Output missing header")
	}
	if !strings.Contains(output, "⚙️") || !strings.Contains(output, "[preflight]") {
		t.Error("Output missing event started")
	}
	if !strings.Contains(output, "🏁") {
		t.Error("Output missing event completed")
	}
	if !strings.Contains(output, "✅") || !strings.Contains(output, "ls -la") {
		t.Error("Output missing command")
	}

	// Verify sorting (Event @ 10:00, Cmd @ 10:01, Event @ 10:02)
	idxEventStart := strings.Index(output, "⚙️")
	idxCmd := strings.Index(output, "ls -la")
	idxEventEnd := strings.Index(output, "🏁")

	if idxEventStart > idxCmd || idxCmd > idxEventEnd {
		t.Error("Output entries not chronologically sorted")
	}
}
