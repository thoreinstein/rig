package ticket

import (
	"regexp"
	"strings"

	"github.com/cockroachdb/errors"
)

// ticketIDRegex matches TYPE-ID where ID can be digits or alphanumeric (e.g., proj-123, rig-abc, beads-42f).
var ticketIDRegex = regexp.MustCompile(`^([a-zA-Z]+)-([a-zA-Z0-9]+)$`)

// TicketInfo holds unified ticket information from various backends (Jira, Beads, etc.)
type TicketInfo struct {
	ID           string
	Title        string
	Type         string
	Status       string
	Priority     string
	Description  string
	CustomFields map[string]string
}

// ParsedTicket holds parsed ticket information from CLI input.
type ParsedTicket struct {
	Full    string // Original user input (e.g., "project:TYPE-ID")
	Project string // Optional project prefix (e.g., "project")
	ID      string // Clean ticket identifier (e.g., "TYPE-ID")
	Type    string // Ticket type, normalized to lowercase (e.g., "type")
	Number  string // Ticket number or alphanumeric identifier (e.g., "ID")
}

// SessionID returns a sanitized ticket identifier suitable for tmux session names
func (t *ParsedTicket) SessionID() string {
	if t.Project != "" {
		return t.Project + "-" + t.ID
	}
	return t.ID
}

// ParseTicket parses a ticket string into type and number/identifier components.
// Supports both traditional Jira-style tickets (proj-123) and beads-style tickets (rig-abc123).
// Also supports optional project prefix (project:ticket).
func ParseTicket(ticketID string) (*ParsedTicket, error) {
	var project string
	fullInput := ticketID

	// Check for optional project prefix
	if p, t, ok := strings.Cut(ticketID, ":"); ok {
		if p == "" {
			return nil, errors.New("invalid ticket format. Project name cannot be empty when using ':'")
		}
		project = p
		ticketID = t
		if ticketID == "" {
			return nil, errors.New("invalid ticket format. Ticket ID cannot be empty when using 'project:ticket'")
		}
	}

	matches := ticketIDRegex.FindStringSubmatch(ticketID)

	if len(matches) != 3 {
		return nil, errors.New("invalid ticket format. Expected format: [project:]TYPE-ID (e.g., proj-123, rig:proj-123 or rig-abc)")
	}

	return &ParsedTicket{
		Full:    fullInput,
		Project: project,
		ID:      ticketID,
		Type:    strings.ToLower(matches[1]),
		Number:  matches[2],
	}, nil
}
