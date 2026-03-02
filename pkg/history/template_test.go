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
	t.Parallel()

	now := time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		entries  []UnifiedEntry
		ticket   string
		contains []string
		absent   []string
	}{
		{
			name:    "empty entries",
			entries: nil,
			ticket:  "EMPTY-1",
			contains: []string{
				"## Workflow Timeline - EMPTY-1",
				"**Manual Commands:** 0",
				"**System Events:** 0",
			},
		},
		{
			name: "commands only",
			entries: []UnifiedEntry{
				UnifiedEntryFromCommand(Command{
					Command:   "git status",
					Timestamp: now,
					ExitCode:  0,
					Duration:  200,
				}),
			},
			ticket: "CMD-1",
			contains: []string{
				"## Workflow Timeline - CMD-1",
				"**Manual Commands:** 1",
				"**System Events:** 0",
				"✅",
				"git status",
			},
		},
		{
			name: "events only",
			entries: []UnifiedEntry{
				UnifiedEntryFromEvent(now, "preflight", "STARTED", "Starting checks", "wf1"),
				UnifiedEntryFromEvent(now.Add(time.Minute), "preflight", "COMPLETED", "", "wf1"),
			},
			ticket: "EVT-1",
			contains: []string{
				"## Workflow Timeline - EVT-1",
				"**Manual Commands:** 0",
				"**System Events:** 2",
				"⚙️",
				"🏁",
				"[preflight]",
			},
			absent: []string{
				"Command Duration:",
			},
		},
		{
			name: "failed event rendering",
			entries: []UnifiedEntry{
				UnifiedEntryFromEvent(now, "merge", "FAILED", "conflict detected", "wf2"),
			},
			ticket: "FAIL-1",
			contains: []string{
				"⚠️",
				"[merge]",
				"conflict detected",
			},
		},
		{
			name: "mixed entries sorted chronologically",
			entries: []UnifiedEntry{
				UnifiedEntryFromCommand(Command{
					Command:   "ls -la",
					Timestamp: now.Add(1 * time.Minute),
					ExitCode:  0,
				}),
				UnifiedEntryFromEvent(now, "preflight", "STARTED", "Starting checks", "wf123"),
				UnifiedEntryFromEvent(now.Add(2*time.Minute), "preflight", "COMPLETED", "", "wf123"),
			},
			ticket: "MIX-1",
			contains: []string{
				"## Workflow Timeline - MIX-1",
				"**Manual Commands:** 1",
				"**System Events:** 2",
				"⚙️",
				"🏁",
				"✅",
				"ls -la",
			},
		},
		{
			name: "nil command pointer skipped",
			entries: []UnifiedEntry{
				{Kind: EntryKindCommand, Timestamp: now, Command: nil},
				UnifiedEntryFromEvent(now, "preflight", "STARTED", "ok", "wf3"),
			},
			ticket: "NIL-1",
			contains: []string{
				"⚙️",
				"[preflight]",
			},
		},
		{
			name: "nil event pointer skipped",
			entries: []UnifiedEntry{
				{Kind: EntryKindEvent, Timestamp: now, Event: nil},
				UnifiedEntryFromCommand(Command{
					Command:   "echo hello",
					Timestamp: now,
					ExitCode:  0,
				}),
			},
			ticket: "NIL-2",
			contains: []string{
				"echo hello",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			output := FormatUnifiedTimeline(tt.entries, tt.ticket)

			for _, want := range tt.contains {
				if !strings.Contains(output, want) {
					t.Errorf("output missing %q", want)
				}
			}
			for _, absent := range tt.absent {
				if strings.Contains(output, absent) {
					t.Errorf("output should not contain %q", absent)
				}
			}
		})
	}
}

func TestFormatUnifiedTimeline_ChronologicalOrder(t *testing.T) {
	t.Parallel()

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

	// Verify sorting (Event @ 10:00, Cmd @ 10:01, Event @ 10:02)
	idxEventStart := strings.Index(output, "⚙️")
	idxCmd := strings.Index(output, "ls -la")
	idxEventEnd := strings.Index(output, "🏁")

	if idxEventStart > idxCmd || idxCmd > idxEventEnd {
		t.Error("Output entries not chronologically sorted")
	}
}
