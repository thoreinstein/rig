package vcs

import (
	"context"
	"time"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
)

// PluginManager defines the interface for acquiring and releasing VCS plugins.
type PluginManager interface {
	GetVCSClient(ctx context.Context, name string) (apiv1.VCSServiceClient, error)
	ReleasePlugin(name string)
}

// rpcTimeout is the default timeout for VCS plugin RPC calls.
const rpcTimeout = 30 * time.Second

// rpcLongTimeout is the timeout for potentially long-running VCS plugin RPC calls (e.g., Clone).
const rpcLongTimeout = 15 * time.Minute

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

func (p *PluginProvider) GetRepoRoot(path string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()

	client, err := p.Manager.GetVCSClient(ctx, p.PluginName)
	if err != nil {
		return "", err
	}
	defer p.Manager.ReleasePlugin(p.PluginName)

	resp, err := client.GetRepoRoot(ctx, &apiv1.GetRepoRootRequest{Path: path})
	if err != nil {
		return "", err
	}
	return resp.Root, nil
}

func (p *PluginProvider) GetRepoName(path string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()

	client, err := p.Manager.GetVCSClient(ctx, p.PluginName)
	if err != nil {
		return "", err
	}
	defer p.Manager.ReleasePlugin(p.PluginName)

	resp, err := client.GetRepoName(ctx, &apiv1.GetRepoNameRequest{Path: path})
	if err != nil {
		return "", err
	}
	return resp.Name, nil
}

func (p *PluginProvider) GetDefaultBranch(path, baseBranchConfig string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()

	client, err := p.Manager.GetVCSClient(ctx, p.PluginName)
	if err != nil {
		return "", err
	}
	defer p.Manager.ReleasePlugin(p.PluginName)

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
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()

	client, err := p.Manager.GetVCSClient(ctx, p.PluginName)
	if err != nil {
		return "", err
	}
	defer p.Manager.ReleasePlugin(p.PluginName)

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
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()

	client, err := p.Manager.GetVCSClient(ctx, p.PluginName)
	if err != nil {
		return nil, err
	}
	defer p.Manager.ReleasePlugin(p.PluginName)

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
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()

	client, err := p.Manager.GetVCSClient(ctx, p.PluginName)
	if err != nil {
		return err
	}
	defer p.Manager.ReleasePlugin(p.PluginName)

	_, err = client.RemoveWorktree(ctx, &apiv1.RemoveWorktreeRequest{
		Path:       path,
		TicketType: ticketType,
		Ticket:     ticket,
	})
	return err
}

func (p *PluginProvider) ForceRemoveWorktree(path, worktreePath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()

	client, err := p.Manager.GetVCSClient(ctx, p.PluginName)
	if err != nil {
		return err
	}
	defer p.Manager.ReleasePlugin(p.PluginName)

	_, err = client.ForceRemoveWorktree(ctx, &apiv1.ForceRemoveWorktreeRequest{
		Path:         path,
		WorktreePath: worktreePath,
	})
	return err
}

func (p *PluginProvider) GetWorktreePath(path, ticketType, ticket string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()

	client, err := p.Manager.GetVCSClient(ctx, p.PluginName)
	if err != nil {
		return "", err
	}
	defer p.Manager.ReleasePlugin(p.PluginName)

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
	ctx, cancel := context.WithTimeout(context.Background(), rpcLongTimeout)
	defer cancel()

	client, err := p.Manager.GetVCSClient(ctx, p.PluginName)
	if err != nil {
		return "", err
	}
	defer p.Manager.ReleasePlugin(p.PluginName)

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
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()

	client, err := p.Manager.GetVCSClient(ctx, p.PluginName)
	if err != nil {
		return false, err
	}
	defer p.Manager.ReleasePlugin(p.PluginName)

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
