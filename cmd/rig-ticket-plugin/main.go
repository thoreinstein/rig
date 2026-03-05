package main

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/cockroachdb/errors"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
	"thoreinstein.com/rig/pkg/beads"
	"thoreinstein.com/rig/pkg/config"
	"thoreinstein.com/rig/pkg/jira"
	"thoreinstein.com/rig/pkg/sdk"
	"thoreinstein.com/rig/pkg/workflow"
)

type TicketPlugin struct{}

func (p *TicketPlugin) Info() sdk.Info {
	return sdk.Info{
		ID:         "rig-ticket",
		APIVersion: "v1",
		SemVer:     "1.0.0",
		Capabilities: []sdk.Capability{
			{Name: "ticket", Version: "1.0.0"},
		},
	}
}

func (p *TicketPlugin) GetTicketInfo(ctx context.Context, req *apiv1.GetTicketInfoRequest) (*apiv1.GetTicketInfoResponse, error) {
	// Route by ticket ID format. The plugin uses simple heuristics (IsJiraTicket/IsBeadsTicket)
	// rather than TicketRouter because plugins run in isolated processes without filesystem
	// access to project markers like .beads/.
	if workflow.IsJiraTicket(req.TicketId) {
		client, err := p.getJiraClient(ctx)
		if err != nil {
			return nil, err
		}
		info, err := client.FetchTicketDetails(req.TicketId)
		if err != nil {
			return nil, err
		}
		return &apiv1.GetTicketInfoResponse{
			Ticket: &apiv1.TicketInfo{
				Id:           req.TicketId,
				Title:        info.Summary,
				Type:         info.Type,
				Status:       info.Status,
				Priority:     info.Priority,
				Description:  info.Description,
				CustomFields: info.CustomFields,
			},
		}, nil
	}

	// Assume Beads otherwise
	client, err := p.getBeadsClient(ctx)
	if err != nil {
		return nil, err
	}
	info, err := client.Show(req.TicketId)
	if err != nil {
		return nil, err
	}
	return &apiv1.GetTicketInfoResponse{
		Ticket: &apiv1.TicketInfo{
			Id:          info.ID,
			Title:       info.Title,
			Type:        info.Type,
			Status:      info.Status,
			Priority:    strconv.Itoa(info.Priority),
			Description: info.Description,
		},
	}, nil
}

func (p *TicketPlugin) UpdateTicketStatus(ctx context.Context, req *apiv1.UpdateTicketStatusRequest) (*apiv1.UpdateTicketStatusResponse, error) {
	if workflow.IsJiraTicket(req.TicketId) {
		client, err := p.getJiraClient(ctx)
		if err != nil {
			return nil, err
		}
		// Basic mapping for "in_progress"
		if req.Status == "in_progress" {
			err := client.TransitionTicketByName(req.TicketId, "In Progress")
			if err != nil {
				err = client.TransitionTicketByName(req.TicketId, "Start Progress")
			}
			if err != nil {
				return nil, err
			}
		} else {
			return nil, errors.Newf("status transition %q not supported for Jira in this plugin", req.Status)
		}
		return &apiv1.UpdateTicketStatusResponse{Success: true}, nil
	}

	client, err := p.getBeadsClient(ctx)
	if err != nil {
		return nil, err
	}
	err = client.UpdateStatus(req.TicketId, req.Status)
	if err != nil {
		return nil, err
	}
	return &apiv1.UpdateTicketStatusResponse{Success: true}, nil
}

func (p *TicketPlugin) ListTransitions(ctx context.Context, req *apiv1.ListTransitionsRequest) (*apiv1.ListTransitionsResponse, error) {
	if workflow.IsJiraTicket(req.TicketId) {
		client, err := p.getJiraClient(ctx)
		if err != nil {
			return nil, err
		}
		ts, err := client.GetTransitions(req.TicketId)
		if err != nil {
			return nil, err
		}
		transitions := make([]*apiv1.Transition, len(ts))
		for i, t := range ts {
			transitions[i] = &apiv1.Transition{
				Id:   t.ID,
				Name: t.Name,
			}
		}
		return &apiv1.ListTransitionsResponse{Transitions: transitions}, nil
	}
	// Beads doesn't have a transitions API in the current client
	return &apiv1.ListTransitionsResponse{}, nil
}

func (p *TicketPlugin) getJiraClient(ctx context.Context) (jira.JiraClient, error) {
	token, err := sdk.GetSecret(ctx, "JIRA_TOKEN")
	if err != nil {
		return nil, errors.Wrap(err, "failed to retrieve JIRA_TOKEN")
	}
	baseURL, err := sdk.GetSecret(ctx, "JIRA_BASE_URL")
	if err != nil {
		return nil, errors.Wrap(err, "failed to retrieve JIRA_BASE_URL")
	}
	email, _ := sdk.GetSecret(ctx, "JIRA_EMAIL")

	cfg := &config.JiraConfig{
		Enabled: true,
		Mode:    "api",
		BaseURL: baseURL,
		Email:   email,
		Token:   token,
	}
	return jira.NewJiraClient(cfg, false)
}

func (p *TicketPlugin) getBeadsClient(ctx context.Context) (beads.BeadsClient, error) {
	// Beads usually uses the CLI directly
	return beads.NewCLIClient("bd", false)
}

func main() {
	plugin := &TicketPlugin{}
	if err := sdk.Serve(plugin); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
