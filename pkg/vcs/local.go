package vcs

import (
	"thoreinstein.com/rig/pkg/git"
	vcsurl "thoreinstein.com/rig/pkg/vcs/url"
)

// LocalProvider implements the Provider interface using local git commands.
type LocalProvider struct {
	Verbose bool
}

// NewLocalProvider creates a new LocalProvider.
func NewLocalProvider(verbose bool) *LocalProvider {
	return &LocalProvider{Verbose: verbose}
}

func (p *LocalProvider) GetRepoRoot(path string) (string, error) {
	wm := git.NewWorktreeManagerAtPath(path, "", p.Verbose)
	return wm.GetRepoRoot()
}

func (p *LocalProvider) GetRepoName(path string) (string, error) {
	wm := git.NewWorktreeManagerAtPath(path, "", p.Verbose)
	return wm.GetRepoName()
}

func (p *LocalProvider) GetDefaultBranch(path, baseBranchConfig string) (string, error) {
	wm := git.NewWorktreeManagerAtPath(path, baseBranchConfig, p.Verbose)
	return wm.GetDefaultBranch()
}

func (p *LocalProvider) CreateWorktree(path, ticketType, name, branchName, baseBranchConfig string) (string, error) {
	wm := git.NewWorktreeManagerAtPath(path, baseBranchConfig, p.Verbose)
	return wm.CreateWorktreeWithBranch(ticketType, name, branchName)
}

func (p *LocalProvider) ListWorktrees(path string) ([]WorktreeInfo, error) {
	wm := git.NewWorktreeManagerAtPath(path, "", p.Verbose)
	gitInfos, err := wm.ListWorktreesDetailed()
	if err != nil {
		return nil, err
	}

	infos := make([]WorktreeInfo, len(gitInfos))
	for i, gi := range gitInfos {
		infos[i] = WorktreeInfo{
			Path:   gi.Path,
			Branch: gi.Branch,
		}
	}
	return infos, nil
}

func (p *LocalProvider) RemoveWorktree(path, ticketType, ticket string) error {
	wm := git.NewWorktreeManagerAtPath(path, "", p.Verbose)
	return wm.RemoveWorktree(ticketType, ticket)
}

func (p *LocalProvider) ForceRemoveWorktree(path, worktreePath string) error {
	wm := git.NewWorktreeManagerAtPath(path, "", p.Verbose)
	return wm.ForceRemoveWorktree(worktreePath)
}

func (p *LocalProvider) GetWorktreePath(path, ticketType, ticket string) (string, error) {
	wm := git.NewWorktreeManagerAtPath(path, "", p.Verbose)
	return wm.GetWorktreePath(ticketType, ticket)
}

func (p *LocalProvider) Clone(url, basePath string) (string, error) {
	repoURL, err := vcsurl.ParseGitHubURL(url)
	if err != nil {
		return "", err
	}

	cm := git.NewCloneManager(basePath, p.Verbose)
	return cm.Clone(repoURL)
}

func (p *LocalProvider) IsBranchMerged(path, branch, baseBranch string) (bool, error) {
	wm := git.NewWorktreeManagerAtPath(path, "", p.Verbose)
	return wm.IsBranchMerged(branch, baseBranch)
}
