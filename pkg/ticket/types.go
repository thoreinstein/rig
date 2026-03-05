package ticket

import "strings"

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

// NormalizedType returns the ticket type in lowercase.
func (t *TicketInfo) NormalizedType() string {
	return strings.ToLower(t.Type)
}
