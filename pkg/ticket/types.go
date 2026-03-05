package ticket

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
