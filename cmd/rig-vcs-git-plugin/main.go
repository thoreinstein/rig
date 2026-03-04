package main

import (
	"context"
	"fmt"
	"os"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
	"thoreinstein.com/rig/pkg/git"
	"thoreinstein.com/rig/pkg/sdk"
)

type GitPlugin struct{}

func (p *GitPlugin) Info() sdk.Info {
	return sdk.Info{
		ID:         "vcs-git",
		APIVersion: "v1",
		SemVer:     "1.0.0",
		Capabilities: []sdk.Capability{
			{Name: "vcs", Version: "1.0.0"},
		},
	}
}

func (p *GitPlugin) GetRepoRoot(ctx context.Context, req *apiv1.GetRepoRootRequest) (*apiv1.GetRepoRootResponse, error) {
	wm := git.NewWorktreeManagerAtPath(req.Path, "", false)
	root, err := wm.GetRepoRoot()
	if err != nil {
		return nil, err
	}
	return &apiv1.GetRepoRootResponse{Root: root}, nil
}

func (p *GitPlugin) GetRepoName(ctx context.Context, req *apiv1.GetRepoNameRequest) (*apiv1.GetRepoNameResponse, error) {
	wm := git.NewWorktreeManagerAtPath(req.Path, "", false)
	name, err := wm.GetRepoName()
	if err != nil {
		return nil, err
	}
	return &apiv1.GetRepoNameResponse{Name: name}, nil
}

func (p *GitPlugin) GetDefaultBranch(ctx context.Context, req *apiv1.GetDefaultBranchRequest) (*apiv1.GetDefaultBranchResponse, error) {
	wm := git.NewWorktreeManagerAtPath(req.Path, req.BaseBranchConfig, false)
	branch, err := wm.GetDefaultBranch()
	if err != nil {
		return nil, err
	}
	return &apiv1.GetDefaultBranchResponse{Branch: branch}, nil
}

func (p *GitPlugin) CreateWorktree(ctx context.Context, req *apiv1.CreateWorktreeRequest) (*apiv1.CreateWorktreeResponse, error) {
	wm := git.NewWorktreeManagerAtPath(req.Path, req.BaseBranchConfig, false)
	path, err := wm.CreateWorktreeWithBranch(req.TicketType, req.Name, req.BranchName)
	if err != nil {
		return nil, err
	}
	return &apiv1.CreateWorktreeResponse{WorktreePath: path}, nil
}

func (p *GitPlugin) ListWorktrees(ctx context.Context, req *apiv1.ListWorktreesRequest) (*apiv1.ListWorktreesResponse, error) {
	wm := git.NewWorktreeManagerAtPath(req.Path, "", false)
	gitInfos, err := wm.ListWorktreesDetailed()
	if err != nil {
		return nil, err
	}

	worktrees := make([]*apiv1.WorktreeInfo, len(gitInfos))
	for i, gi := range gitInfos {
		worktrees[i] = &apiv1.WorktreeInfo{
			Path:   gi.Path,
			Branch: gi.Branch,
		}
	}
	return &apiv1.ListWorktreesResponse{Worktrees: worktrees}, nil
}

func (p *GitPlugin) RemoveWorktree(ctx context.Context, req *apiv1.RemoveWorktreeRequest) (*apiv1.RemoveWorktreeResponse, error) {
	wm := git.NewWorktreeManagerAtPath(req.Path, "", false)
	err := wm.RemoveWorktree(req.TicketType, req.Ticket)
	if err != nil {
		return nil, err
	}
	return &apiv1.RemoveWorktreeResponse{}, nil
}

func (p *GitPlugin) ForceRemoveWorktree(ctx context.Context, req *apiv1.ForceRemoveWorktreeRequest) (*apiv1.ForceRemoveWorktreeResponse, error) {
	wm := git.NewWorktreeManagerAtPath(req.Path, "", false)
	err := wm.ForceRemoveWorktree(req.WorktreePath)
	if err != nil {
		return nil, err
	}
	return &apiv1.ForceRemoveWorktreeResponse{}, nil
}

func (p *GitPlugin) GetWorktreePath(ctx context.Context, req *apiv1.GetWorktreePathRequest) (*apiv1.GetWorktreePathResponse, error) {
	wm := git.NewWorktreeManagerAtPath(req.Path, "", false)
	path, err := wm.GetWorktreePath(req.TicketType, req.Ticket)
	if err != nil {
		return nil, err
	}
	return &apiv1.GetWorktreePathResponse{WorktreePath: path}, nil
}

func (p *GitPlugin) Clone(ctx context.Context, req *apiv1.CloneRequest) (*apiv1.CloneResponse, error) {
	repoURL, err := git.ParseGitHubURL(req.Url)
	if err != nil {
		return nil, err
	}

	cm := git.NewCloneManager(req.BasePath, false)
	path, err := cm.Clone(repoURL)
	if err != nil {
		return nil, err
	}
	return &apiv1.CloneResponse{RepoPath: path}, nil
}

func (p *GitPlugin) IsBranchMerged(ctx context.Context, req *apiv1.IsBranchMergedRequest) (*apiv1.IsBranchMergedResponse, error) {
	wm := git.NewWorktreeManagerAtPath(req.Path, "", false)
	merged, err := wm.IsBranchMerged(req.Branch, req.BaseBranch)
	if err != nil {
		return nil, err
	}
	return &apiv1.IsBranchMergedResponse{IsMerged: merged}, nil
}

func main() {
	plugin := &GitPlugin{}
	if err := sdk.Serve(plugin); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
