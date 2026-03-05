package knowledge

import (
	"context"
)

// Provider defines the interface for knowledge management integrations.
type Provider interface {
	// CreateTicketNote creates or returns an existing ticket note.
	CreateTicketNote(ctx context.Context, data *NoteData) (*NoteResult, error)

	// UpdateDailyNote adds an entry to the daily note, creating it if necessary.
	UpdateDailyNote(ctx context.Context, ticket, ticketType string) error

	// GetNotePath returns the path for a ticket note without creating it.
	GetNotePath(ctx context.Context, ticketType, ticket string) (string, error)

	// GetDailyNotePath returns the path for today's daily note.
	GetDailyNotePath(ctx context.Context) (string, error)
}
