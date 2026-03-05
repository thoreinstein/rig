package ticket

import (
	"context"
	"strconv"

	"github.com/cockroachdb/errors"

	"thoreinstein.com/rig/pkg/beads"
	"thoreinstein.com/rig/pkg/config"
	"thoreinstein.com/rig/pkg/jira"
	"thoreinstein.com/rig/pkg/workflow"
)

// LocalProvider implements the Provider interface by delegating to
// existing pkg/jira and pkg/beads implementations.
type LocalProvider struct {
	cfg         *config.Config
	projectPath string
	verbose     bool
	router      *workflow.TicketRouter
}

// NewLocalProvider creates a new LocalProvider with the given configuration.
func NewLocalProvider(cfg *config.Config, projectPath string, verbose bool) *LocalProvider {
	return &LocalProvider{
		cfg:         cfg,
		projectPath: projectPath,
		verbose:     verbose,
		router:      workflow.NewTicketRouter(cfg, projectPath, verbose),
	}
}

// IsAvailable checks if the ticketing system for the current project is reachable.
func (p *LocalProvider) IsAvailable(ctx context.Context) bool {
	// Simple check: are either beads or jira enabled?
	return p.cfg.Beads.Enabled || p.cfg.Jira.Enabled
}

// GetTicketInfo retrieves ticket information by routing to the appropriate backend.
func (p *LocalProvider) GetTicketInfo(ctx context.Context, ticketID string) (*TicketInfo, error) {
	source := p.router.RouteTicket(ticketID)

	switch source {
	case workflow.TicketSourceBeads:
		client, err := beads.NewCLIClient(p.cfg.Beads.CliCommand, p.verbose)
		if err != nil {
			return nil, errors.Wrap(err, "failed to initialize beads client")
		}
		info, err := client.Show(ticketID)
		if err != nil {
			return nil, err
		}
		return &TicketInfo{
			ID:          info.ID,
			Title:       info.Title,
			Type:        info.Type,
			Status:      info.Status,
			Priority:    strconv.Itoa(info.Priority),
			Description: info.Description,
		}, nil

	case workflow.TicketSourceJira:
		client, err := jira.NewJiraClient(&p.cfg.Jira, p.verbose)
		if err != nil {
			return nil, errors.Wrap(err, "failed to initialize jira client")
		}
		info, err := client.FetchTicketDetails(ticketID)
		if err != nil {
			return nil, err
		}
		return &TicketInfo{
			ID:           ticketID,
			Title:        info.Summary,
			Type:         info.Type,
			Status:       info.Status,
			Priority:     info.Priority,
			Description:  info.Description,
			CustomFields: info.CustomFields,
		}, nil

	default:
		return nil, errors.Newf("unsupported or unknown ticket source for ID %q", ticketID)
	}
}

// UpdateStatus updates the status of the ticket in the appropriate backend.
func (p *LocalProvider) UpdateStatus(ctx context.Context, ticketID, status string) error {
	source := p.router.RouteTicket(ticketID)

	switch source {
	case workflow.TicketSourceBeads:
		client, err := beads.NewCLIClient(p.cfg.Beads.CliCommand, p.verbose)
		if err != nil {
			return errors.Wrap(err, "failed to initialize beads client")
		}
		return client.UpdateStatus(ticketID, status)

	case workflow.TicketSourceJira:
		// Jira transitions are complex; for local mode we map "in_progress" to common Jira status names
		// for basic "work" command support.
		client, err := jira.NewJiraClient(&p.cfg.Jira, p.verbose)
		if err != nil {
			return errors.Wrap(err, "failed to initialize jira client")
		}

		// Attempt to transition to common Jira "In Progress" status names
		if status == "in_progress" {
			err := client.TransitionTicketByName(ticketID, "In Progress")
			if err != nil {
				// Fallback to "Start Progress" if "In Progress" fails
				return client.TransitionTicketByName(ticketID, "Start Progress")
			}
			return nil
		}
		return errors.Newf("status transition %q not supported in local jira mode", status)

	default:
		return errors.Newf("unsupported or unknown ticket source for ID %q", ticketID)
	}
}
