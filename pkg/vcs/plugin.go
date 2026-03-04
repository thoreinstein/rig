package vcs

import (
	"context"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
	"thoreinstein.com/rig/pkg/plugin"
)

// PluginProvider implements the Provider interface by delegating to a Rig plugin.
type PluginProvider struct {
	Manager    *plugin.Manager
	PluginName string
	Verbose    bool
}

// NewPluginProvider creates a new PluginProvider.
func NewPluginProvider(manager *plugin.Manager, pluginName string, verbose bool) *PluginProvider {
	return &PluginProvider{
		Manager:    manager,
		PluginName: pluginName,
		Verbose:    verbose,
	}
}

func (p *PluginProvider) GetRepoRoot(path string) (string, error) {
	ctx := context.Background()
	client, err := p.Manager.GetVCSClient(ctx, p.PluginName)
	if err != nil {
		return "", err
	}

	resp, err := client.GetRepoRoot(ctx, &apiv1.GetRepoRootRequest{Path: path})
	if err != nil {
		return "", err
	}
	return resp.Root, nil
}

func (p *PluginProvider) GetRepoName(path string) (string, error) {
	ctx := context.Background()
	client, err := p.Manager.GetVCSClient(ctx, p.PluginName)
	if err != nil {
		return "", err
	}

	resp, err := client.GetRepoName(ctx, &apiv1.GetRepoNameRequest{Path: path})
	if err != nil {
		return "", err
	}
	return resp.Name, nil
}

func (p *PluginProvider) GetDefaultBranch(path, baseBranchConfig string) (string, error) {
	ctx := context.Background()
	client, err := p.Manager.GetVCSClient(ctx, p.PluginName)
	if err != nil {
		return "", err
	}

	resp, err := client.GetDefaultBranch(ctx, &apiv1.GetDefaultBranchRequest{
		Path:             path,
		BaseBranchConfig: baseBranchConfig,
	})
	if err != nil {
		return "", err
	}
	return resp.Branch, nil
}

func (p *PluginProvider) CreateWorktree(path, ticketType, name, branchName, baseBranchConfig string) (string, error) {
	ctx := context.Background()
	client, err := p.Manager.GetVCSClient(ctx, p.PluginName)
	if err != nil {
		return "", err
	}

	resp, err := client.CreateWorktree(ctx, &apiv1.CreateWorktreeRequest{
		Path:             path,
		TicketType:       ticketType,
		Name:             name,
		BranchName:       branchName,
		BaseBranchConfig: baseBranchConfig,
	})
	if err != nil {
		return "", err
	}
	return resp.WorktreePath, nil
}

func (p *PluginProvider) ListWorktrees(path string) ([]WorktreeInfo, error) {
	ctx := context.Background()
	client, err := p.Manager.GetVCSClient(ctx, p.PluginName)
	if err != nil {
		return nil, err
	}

	resp, err := client.ListWorktrees(ctx, &apiv1.ListWorktreesRequest{Path: path})
	if err != nil {
		return nil, err
	}

	infos := make([]WorktreeInfo, len(resp.Worktrees))
	for i, wt := range resp.Worktrees {
		infos[i] = WorktreeInfo{
			Path:   wt.Path,
			Branch: wt.Branch,
		}
	}
	return infos, nil
}

func (p *PluginProvider) RemoveWorktree(path, ticketType, ticket string) error {
	ctx := context.Background()
	client, err := p.Manager.GetVCSClient(ctx, p.PluginName)
	if err != nil {
		return err
	}

	_, err = client.RemoveWorktree(ctx, &apiv1.RemoveWorktreeRequest{
		Path:       path,
		TicketType: ticketType,
		Ticket:     ticket,
	})
	return err
}

func (p *PluginProvider) ForceRemoveWorktree(path, worktreePath string) error {
	ctx := context.Background()
	client, err := p.Manager.GetVCSClient(ctx, p.PluginName)
	if err != nil {
		return err
	}

	_, err = client.ForceRemoveWorktree(ctx, &apiv1.ForceRemoveWorktreeRequest{
		Path:         path,
		WorktreePath: worktreePath,
	})
	return err
}

func (p *PluginProvider) GetWorktreePath(path, ticketType, ticket string) (string, error) {
	ctx := context.Background()
	client, err := p.Manager.GetVCSClient(ctx, p.PluginName)
	if err != nil {
		return "", err
	}

	resp, err := client.GetWorktreePath(ctx, &apiv1.GetWorktreePathRequest{
		Path:       path,
		TicketType: ticketType,
		Ticket:     ticket,
	})
	if err != nil {
		return "", err
	}
	return resp.WorktreePath, nil
}

func (p *PluginProvider) Clone(url, basePath string) (string, error) {
	ctx := context.Background()
	client, err := p.Manager.GetVCSClient(ctx, p.PluginName)
	if err != nil {
		return "", err
	}

	resp, err := client.Clone(ctx, &apiv1.CloneRequest{
		Url:      url,
		BasePath: basePath,
	})
	if err != nil {
		return "", err
	}
	return resp.RepoPath, nil
}

func (p *PluginProvider) IsBranchMerged(path, branch, baseBranch string) (bool, error) {
	ctx := context.Background()
	client, err := p.Manager.GetVCSClient(ctx, p.PluginName)
	if err != nil {
		return false, err
	}

	resp, err := client.IsBranchMerged(ctx, &apiv1.IsBranchMergedRequest{
		Path:       path,
		Branch:     branch,
		BaseBranch: baseBranch,
	})
	if err != nil {
		return false, err
	}
	return resp.IsMerged, nil
}
