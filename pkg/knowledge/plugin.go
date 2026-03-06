package knowledge

import (
	"context"
	"time"

	"github.com/cockroachdb/errors"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
)

// rpcTimeout is the timeout for knowledge plugin RPC calls.
const rpcTimeout = 30 * time.Second

// PluginManager abstracts plugin session lifecycle for testing.
type PluginManager interface {
	GetKnowledgeClient(ctx context.Context, name string) (apiv1.KnowledgeServiceClient, error)
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

// CreateTicketNote creates or returns an existing ticket note via gRPC.
func (p *PluginProvider) CreateTicketNote(ctx context.Context, data *NoteData) (*NoteResult, error) {
	rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
	defer cancel()

	client, err := p.Manager.GetKnowledgeClient(rpcCtx, p.PluginName)
	if err != nil {
		return nil, err
	}
	defer p.Manager.ReleasePlugin(p.PluginName)

	resp, err := client.CreateTicketNote(rpcCtx, &apiv1.CreateTicketNoteRequest{
		Metadata: &apiv1.NoteMetadata{
			TicketId:     data.Ticket,
			TicketType:   data.TicketType,
			Date:         data.Date,
			Time:         data.Time,
			Summary:      data.Summary,
			Status:       data.Status,
			Description:  data.Description,
			RepoName:     data.RepoName,
			RepoPath:     data.RepoPath,
			WorktreePath: data.WorktreePath,
		},
	})
	if err != nil {
		return nil, err
	}

	if resp == nil {
		return nil, errors.Newf("plugin %q returned empty response for CreateTicketNote", p.PluginName)
	}

	return &NoteResult{
		Path:    resp.Path,
		Created: resp.Created,
	}, nil
}

// UpdateDailyNote updates the daily note via gRPC.
func (p *PluginProvider) UpdateDailyNote(ctx context.Context, ticket, ticketType string) error {
	rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
	defer cancel()

	client, err := p.Manager.GetKnowledgeClient(rpcCtx, p.PluginName)
	if err != nil {
		return err
	}
	defer p.Manager.ReleasePlugin(p.PluginName)

	resp, err := client.UpdateDailyNote(rpcCtx, &apiv1.UpdateDailyNoteRequest{
		TicketId:   ticket,
		TicketType: ticketType,
	})
	if err != nil {
		return err
	}

	if resp == nil {
		return errors.Newf("plugin %q returned empty response for UpdateDailyNote", p.PluginName)
	}

	if !resp.Success {
		return errors.Newf("plugin %q failed to update daily note for %s", p.PluginName, ticket)
	}

	return nil
}

// GetNotePath returns the path for a ticket note via gRPC.
func (p *PluginProvider) GetNotePath(ctx context.Context, ticketType, ticket string) (string, error) {
	rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
	defer cancel()

	client, err := p.Manager.GetKnowledgeClient(rpcCtx, p.PluginName)
	if err != nil {
		return "", err
	}
	defer p.Manager.ReleasePlugin(p.PluginName)

	resp, err := client.GetNotePath(rpcCtx, &apiv1.GetNotePathRequest{
		TicketId:   ticket,
		TicketType: ticketType,
	})
	if err != nil {
		return "", err
	}

	if resp == nil {
		return "", errors.Newf("plugin %q returned empty response for GetNotePath", p.PluginName)
	}

	return resp.Path, nil
}

// GetDailyNotePath returns the path for today's daily note via gRPC.
func (p *PluginProvider) GetDailyNotePath(ctx context.Context) (string, error) {
	rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
	defer cancel()

	client, err := p.Manager.GetKnowledgeClient(rpcCtx, p.PluginName)
	if err != nil {
		return "", err
	}
	defer p.Manager.ReleasePlugin(p.PluginName)

	resp, err := client.GetDailyNotePath(rpcCtx, &apiv1.GetDailyNotePathRequest{})
	if err != nil {
		return "", err
	}

	if resp == nil {
		return "", errors.Newf("plugin %q returned empty response for GetDailyNotePath", p.PluginName)
	}

	return resp.Path, nil
}
