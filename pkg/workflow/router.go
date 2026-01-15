package workflow

import (
	"thoreinstein.com/rig/pkg/beads"
	"thoreinstein.com/rig/pkg/config"
)

// TicketSource identifies the origin system for a ticket.
type TicketSource string

const (
	// TicketSourceUnknown indicates the ticket system could not be determined.
	TicketSourceUnknown TicketSource = "unknown"
	// TicketSourceBeads indicates the ticket belongs to beads.
	TicketSourceBeads TicketSource = "beads"
	// TicketSourceJira indicates the ticket belongs to JIRA.
	TicketSourceJira TicketSource = "jira"
)

// String returns the string representation of the ticket source.
func (s TicketSource) String() string {
	return string(s)
}

// TicketRouter determines which ticket system owns a given ticket ID.
type TicketRouter struct {
	beadsEnabled bool
	jiraEnabled  bool
	projectPath  string
	verbose      bool
}

// NewTicketRouter creates a TicketRouter configured from the given config.
func NewTicketRouter(cfg *config.Config, projectPath string, verbose bool) *TicketRouter {
	return &TicketRouter{
		beadsEnabled: cfg.Beads.Enabled,
		jiraEnabled:  cfg.Jira.Enabled,
		projectPath:  projectPath,
		verbose:      verbose,
	}
}

// RouteTicket determines which system a ticket belongs to.
//
// Routing rules:
//  1. If beads enabled AND project has .beads/ AND ticket looks like beads ID -> beads
//  2. If jira enabled AND ticket looks like JIRA ID (PROJ-123 with numeric suffix) -> jira
//  3. Otherwise -> unknown
func (r *TicketRouter) RouteTicket(ticketID string) TicketSource {
	// First, check if it could be a beads ticket
	if r.beadsEnabled && IsBeadsTicket(ticketID) {
		// Verify the project actually has beads
		if r.projectPath != "" && beads.IsBeadsProject(r.projectPath) {
			return TicketSourceBeads
		}
		// Also check by walking up from project path
		if r.projectPath != "" {
			if _, found := beads.FindBeadsRoot(r.projectPath); found {
				return TicketSourceBeads
			}
		}
	}

	// Check for JIRA-style ticket (numeric suffix only)
	if r.jiraEnabled && IsJiraTicket(ticketID) {
		return TicketSourceJira
	}

	return TicketSourceUnknown
}

// IsBeadsTicket checks if a ticket ID looks like a beads issue.
// Beads tickets have an alphanumeric suffix that contains at least one letter.
// Examples: rig-k6n, rig-abc123, proj-7xy
func IsBeadsTicket(ticketID string) bool {
	if !looksLikeTicket(ticketID) {
		return false
	}

	// Find the dash position
	dashIdx := findDash(ticketID)
	if dashIdx < 0 {
		return false
	}

	suffix := ticketID[dashIdx+1:]

	// Beads tickets have at least one letter in the suffix
	for i := range len(suffix) {
		if isLetter(suffix[i]) {
			return true
		}
	}

	return false
}

// IsJiraTicket checks if a ticket ID looks like a JIRA ticket.
// JIRA tickets have a numeric-only suffix.
// Examples: PROJ-123, FRAAS-456, ABC-1
func IsJiraTicket(ticketID string) bool {
	if !looksLikeTicket(ticketID) {
		return false
	}

	// Find the dash position
	dashIdx := findDash(ticketID)
	if dashIdx < 0 {
		return false
	}

	suffix := ticketID[dashIdx+1:]

	// JIRA tickets have only digits in the suffix
	for i := range len(suffix) {
		if !isDigit(suffix[i]) {
			return false
		}
	}

	return true
}

// findDash returns the index of the first dash in the string, or -1 if not found.
func findDash(s string) int {
	for i, c := range s {
		if c == '-' {
			return i
		}
	}
	return -1
}
