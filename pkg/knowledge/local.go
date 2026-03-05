package knowledge

import (
	"context"

	"thoreinstein.com/rig/pkg/config"
	"thoreinstein.com/rig/pkg/notes"
)

// LocalProvider implements the Provider interface using local pkg/notes.
type LocalProvider struct {
	manager *notes.Manager
	verbose bool
}

// NewLocalProvider creates a new LocalProvider with the given configuration.
func NewLocalProvider(cfg *config.Config, verbose bool) *LocalProvider {
	manager := notes.NewManager(
		cfg.Notes.Path,
		cfg.Notes.DailyDir,
		cfg.Notes.TemplateDir,
		verbose,
	)
	return &LocalProvider{
		manager: manager,
		verbose: verbose,
	}
}

// CreateTicketNote creates or returns an existing ticket note using the local manager.
func (p *LocalProvider) CreateTicketNote(ctx context.Context, data *NoteData) (*NoteResult, error) {
	result, err := p.manager.CreateTicketNote(notes.TicketData{
		Ticket:       data.Ticket,
		TicketType:   data.TicketType,
		Date:         data.Date,
		Time:         data.Time,
		Summary:      data.Summary,
		Status:       data.Status,
		Description:  data.Description,
		RepoName:     data.RepoName,
		RepoPath:     data.RepoPath,
		WorktreePath: data.WorktreePath,
	})
	if err != nil {
		return nil, err
	}
	return &NoteResult{
		Path:    result.Path,
		Created: result.Created,
	}, nil
}

// UpdateDailyNote adds an entry to the daily note using the local manager.
func (p *LocalProvider) UpdateDailyNote(ctx context.Context, ticket, ticketType string) error {
	return p.manager.UpdateDailyNote(ticket, ticketType)
}

// GetNotePath returns the path for a ticket note using the local manager.
func (p *LocalProvider) GetNotePath(ctx context.Context, ticketType, ticket string) (string, error) {
	return p.manager.GetNotePath(ticketType, ticket), nil
}
