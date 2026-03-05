package ticket

import "context"

// Provider defines the interface for ticketing integrations.
type Provider interface {
	// IsAvailable checks if the ticketing provider is configured and reachable.
	IsAvailable(ctx context.Context) bool

	// GetTicketInfo retrieves detailed information for a specific ticket.
	GetTicketInfo(ctx context.Context, ticketID string) (*TicketInfo, error)

	// UpdateStatus updates the status of a ticket.
	UpdateStatus(ctx context.Context, ticketID, status string) error
}
