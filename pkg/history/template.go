package history

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

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

		for _, cmd := range dayCommands {
			// Format timestamp
			timeStr := cmd.Timestamp.Format("15:04:05")

			// Format status
			var statusIcon string
			if cmd.ExitCode == 0 {
				statusIcon = "✅"
			} else {
				statusIcon = "❌"
			}

			// Format duration
			var durationStr string
			if cmd.Duration > 0 {
				durationStr = fmt.Sprintf(" (%s)", formatDuration(cmd.Duration))
			}

			// Format exit code for failures
			var exitStr string
			if cmd.ExitCode != 0 {
				exitStr = fmt.Sprintf(" [Exit: %d]", cmd.ExitCode)
			}

			// Format directory
			var dirStr string
			if cmd.Directory != "" {
				if len(cmd.Directory) > 50 {
					dirStr = fmt.Sprintf(" `.../%s`", cmd.Directory[len(cmd.Directory)-30:])
				} else {
					dirStr = fmt.Sprintf(" `%s`", cmd.Directory)
				}
			}

			timeline.WriteString(fmt.Sprintf("- %s **%s**%s%s%s: `%s`\n",
				statusIcon, timeStr, durationStr, exitStr, dirStr, cmd.Command))
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
