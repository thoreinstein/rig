package history

import "time"

// Command represents a command from the history database
type Command struct {
	ID        int64
	Command   string
	Timestamp time.Time
	Duration  int64 // milliseconds
	ExitCode  int
	Directory string
	Session   string
	Host      string
}

// QueryOptions defines filtering options for history queries
type QueryOptions struct {
	Since        *time.Time
	Until        *time.Time
	Directory    string
	Session      string // Used for fuzzy matching or session name
	SessionID    string // Exact session ID match
	Ticket       string
	ProjectPaths []string // Paths associated with the project/ticket (OR logic with Ticket)
	ExitCode     *int
	MinDuration  time.Duration // Minimum duration filter
	Limit        int
	Pattern      string
}

// EntryKind identifies the type of entry in a unified timeline.
type EntryKind string

const (
	// EntryKindCommand represents a shell command execution.
	EntryKindCommand EntryKind = "command"
	// EntryKindEvent represents a system/workflow event.
	EntryKindEvent EntryKind = "event"
)

// UnifiedEntry represents a single chronological item in a mixed timeline.
type UnifiedEntry struct {
	Timestamp     time.Time
	Kind          EntryKind
	Command       *Command // Populated for EntryKindCommand
	Event         *Event   // Populated for EntryKindEvent
	CorrelationID string   // Used to group related events
}

// Event represents a system event mapped for timeline display.
type Event struct {
	Step    string
	Status  string
	Message string
}

// UnifiedEntryFromCommand creates a UnifiedEntry from a shell command.
func UnifiedEntryFromCommand(cmd Command) UnifiedEntry {
	return UnifiedEntry{
		Timestamp: cmd.Timestamp,
		Kind:      EntryKindCommand,
		Command:   &cmd,
	}
}

// UnifiedEntryFromEvent creates a UnifiedEntry from workflow event data.
// It takes primitive types to avoid circular dependencies with pkg/events.
func UnifiedEntryFromEvent(timestamp time.Time, step, status, message, correlationID string) UnifiedEntry {
	return UnifiedEntry{
		Timestamp:     timestamp,
		Kind:          EntryKindEvent,
		CorrelationID: correlationID,
		Event: &Event{
			Step:    step,
			Status:  status,
			Message: message,
		},
	}
}
