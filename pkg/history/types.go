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
	Since       *time.Time
	Until       *time.Time
	Directory   string
	Session     string // Used for fuzzy matching or session name
	SessionID   string // Exact session ID match
	Ticket      string
	ExitCode    *int
	MinDuration time.Duration // Minimum duration filter
	Limit       int
	Pattern     string
}
