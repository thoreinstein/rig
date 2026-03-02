package history

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// formatCommandLine renders a single command as a markdown list item.
func formatCommandLine(cmd *Command) string {
	timeStr := cmd.Timestamp.Format("15:04:05")

	var statusIcon string
	if cmd.ExitCode == 0 {
		statusIcon = "✅"
	} else {
		statusIcon = "❌"
	}

	var durationStr string
	if cmd.Duration > 0 {
		durationStr = fmt.Sprintf(" (%s)", formatDuration(cmd.Duration))
	}

	var exitStr string
	if cmd.ExitCode != 0 {
		exitStr = fmt.Sprintf(" [Exit: %d]", cmd.ExitCode)
	}

	var dirStr string
	if cmd.Directory != "" {
		if len(cmd.Directory) > 50 {
			dirStr = fmt.Sprintf(" `.../%s`", cmd.Directory[len(cmd.Directory)-30:])
		} else {
			dirStr = fmt.Sprintf(" `%s`", cmd.Directory)
		}
	}

	return fmt.Sprintf("- %s **%s**%s%s%s: `%s`\n",
		statusIcon, timeStr, durationStr, exitStr, dirStr, cmd.Command)
}

// formatEventLine renders a single event as a markdown list item.
func formatEventLine(ev *Event, timestamp time.Time) string {
	timeStr := timestamp.Format("15:04:05")

	var statusIcon string
	switch ev.Status {
	case "STARTED":
		statusIcon = "⚙️"
	case "COMPLETED":
		statusIcon = "🏁"
	case "FAILED":
		statusIcon = "⚠️"
	default:
		statusIcon = "🔹"
	}

	msg := ev.Message
	if msg == "" {
		msg = ev.Status
	}

	return fmt.Sprintf("- %s **%s** `[%s]` %s\n", statusIcon, timeStr, ev.Step, msg)
}

// FormatTimeline generates a markdown timeline from commands
func FormatTimeline(commands []Command, ticket string) string {
	var timeline strings.Builder

	// Calculate summary stats
	totalCommands := len(commands)
	var successCount int
	var totalDuration int64

	dayGroups := make(map[string][]Command)
	for _, cmd := range commands {
		if cmd.ExitCode == 0 {
			successCount++
		}
		totalDuration += cmd.Duration

		day := cmd.Timestamp.Format("2006-01-02")
		dayGroups[day] = append(dayGroups[day], cmd)
	}

	successRate := 0.0
	if totalCommands > 0 {
		successRate = float64(successCount) / float64(totalCommands) * 100.0
	}

	// Header and Summary
	timeline.WriteString(fmt.Sprintf("## Command Timeline - %s\n\n", ticket))
	timeline.WriteString(fmt.Sprintf("Generated: %s\n\n", time.Now().Format("2006-01-02 15:04:05")))

	timeline.WriteString("### Summary\n")
	timeline.WriteString(fmt.Sprintf("- **Total Commands:** %d\n", totalCommands))
	timeline.WriteString(fmt.Sprintf("- **Success Rate:** %.1f%%\n", successRate))
	timeline.WriteString(fmt.Sprintf("- **Total Duration:** %s\n\n", formatDuration(totalDuration)))

	// Sort days
	days := make([]string, 0, len(dayGroups))
	for day := range dayGroups {
		days = append(days, day)
	}
	sort.Strings(days)

	for _, day := range days {
		dayCommands := dayGroups[day]
		timeline.WriteString(fmt.Sprintf("### %s\n\n", day))

		for i := range dayCommands {
			timeline.WriteString(formatCommandLine(&dayCommands[i]))
		}

		timeline.WriteString("\n")
	}

	return timeline.String()
}

// FormatUnifiedTimeline generates a markdown timeline from interleaved commands and events.
func FormatUnifiedTimeline(entries []UnifiedEntry, ticket string) string {
	var timeline strings.Builder

	// Sort a copy so callers are not surprised by in-place mutation.
	sorted := make([]UnifiedEntry, len(entries))
	copy(sorted, entries)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Timestamp.Before(sorted[j].Timestamp)
	})

	// Group by day
	dayGroups := make(map[string][]UnifiedEntry)
	var commandCount, eventCount int
	var totalDuration int64

	for _, entry := range sorted {
		day := entry.Timestamp.Format("2006-01-02")
		dayGroups[day] = append(dayGroups[day], entry)

		if entry.Kind == EntryKindCommand && entry.Command != nil {
			commandCount++
			totalDuration += entry.Command.Duration
		} else if entry.Kind == EntryKindEvent && entry.Event != nil {
			eventCount++
		}
	}

	// Header and Summary
	fmt.Fprintf(&timeline, "## Workflow Timeline - %s\n\n", ticket)
	fmt.Fprintf(&timeline, "Generated: %s\n\n", time.Now().Format("2006-01-02 15:04:05"))

	timeline.WriteString("### Summary\n")
	fmt.Fprintf(&timeline, "- **Manual Commands:** %d\n", commandCount)
	fmt.Fprintf(&timeline, "- **System Events:** %d\n", eventCount)
	if commandCount > 0 {
		fmt.Fprintf(&timeline, "- **Command Duration:** %s\n", formatDuration(totalDuration))
	}
	timeline.WriteString("\n")

	// Sort days
	days := make([]string, 0, len(dayGroups))
	for day := range dayGroups {
		days = append(days, day)
	}
	sort.Strings(days)

	for _, day := range days {
		dayEntries := dayGroups[day]
		fmt.Fprintf(&timeline, "### %s\n\n", day)

		for _, entry := range dayEntries {
			if entry.Kind == EntryKindCommand {
				if entry.Command == nil {
					continue
				}
				timeline.WriteString(formatCommandLine(entry.Command))
			} else {
				if entry.Event == nil {
					continue
				}
				timeline.WriteString(formatEventLine(entry.Event, entry.Timestamp))
			}
		}
		timeline.WriteString("\n")
	}

	return timeline.String()
}

func formatDuration(ms int64) string {
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	seconds := float64(ms) / 1000.0
	if seconds < 60 {
		return fmt.Sprintf("%.1fs", seconds)
	}
	minutes := int(seconds / 60)
	remSeconds := int(seconds) % 60
	return fmt.Sprintf("%dm%ds", minutes, remSeconds)
}
