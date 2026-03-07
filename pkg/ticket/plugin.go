package ticket

import (
	"context"
	"time"

	"github.com/cockroachdb/errors"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
)

// rpcLongTimeout is the timeout for potentially long-running ticket plugin RPC calls (e.g., Jira API).
const rpcLongTimeout = 15 * time.Minute

// PluginManager is the interface for getting and releasing ticket plugin clients.
type PluginManager interface {
	GetTicketClient(ctx context.Context, name string) (apiv1.TicketServiceClient, error)
	ReleasePlugin(name string)
}

// PluginProvider implements the Provider interface by delegating to a Rig plugin.
type PluginProvider struct {
	Manager    PluginManager
	PluginName string
}

// NewPluginProvider creates a new PluginProvider.
func NewPluginProvider(manager PluginManager, pluginName string) *PluginProvider {
	return &PluginProvider{
		Manager:    manager,
		PluginName: pluginName,
	}
}

// IsAvailable checks if the ticketing provider is configured and reachable.
func (p *PluginProvider) IsAvailable(ctx context.Context) bool {
	// For plugin mode, we assume availability if the plugin can be started.
	// We use a short timeout for this check.
	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	client, err := p.Manager.GetTicketClient(checkCtx, p.PluginName)
	if err != nil {
		return false
	}
	defer p.Manager.ReleasePlugin(p.PluginName)

	return client != nil
}

// GetTicketInfo retrieves detailed information for a specific ticket via gRPC.
func (p *PluginProvider) GetTicketInfo(ctx context.Context, ticketID string) (*TicketInfo, error) {
	// Jira API calls can be slow, use long timeout
	rpcCtx, cancel := context.WithTimeout(ctx, rpcLongTimeout)
	defer cancel()

	client, err := p.Manager.GetTicketClient(rpcCtx, p.PluginName)
	if err != nil {
		return nil, err
	}
	defer p.Manager.ReleasePlugin(p.PluginName)

	resp, err := client.GetTicketInfo(rpcCtx, &apiv1.GetTicketInfoRequest{TicketId: ticketID})
	if err != nil {
		return nil, err
	}

	if resp == nil || resp.Ticket == nil {
		return nil, errors.Newf("plugin returned empty response for ticket %q", ticketID)
	}

	return &TicketInfo{
		ID:           resp.Ticket.Id,
		Title:        resp.Ticket.Title,
		Type:         resp.Ticket.Type,
		Status:       resp.Ticket.Status,
		Priority:     resp.Ticket.Priority,
		Description:  resp.Ticket.Description,
		CustomFields: resp.Ticket.CustomFields,
	}, nil
}

// UpdateStatus updates the status of a ticket via gRPC.
func (p *PluginProvider) UpdateStatus(ctx context.Context, ticketID, status string) error {
	rpcCtx, cancel := context.WithTimeout(ctx, rpcLongTimeout)
	defer cancel()

	client, err := p.Manager.GetTicketClient(rpcCtx, p.PluginName)
	if err != nil {
		return err
	}
	defer p.Manager.ReleasePlugin(p.PluginName)

	resp, err := client.UpdateTicketStatus(rpcCtx, &apiv1.UpdateTicketStatusRequest{
		TicketId: ticketID,
		Status:   status,
	})
	if err != nil {
		return err
	}

	if resp == nil {
		return errors.Newf("plugin returned empty response for ticket %q status update", ticketID)
	}

	if !resp.Success {
		return errors.Newf("plugin failed to update ticket status for %q to %q", ticketID, status)
	}

	return nil
}
