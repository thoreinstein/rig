package vcs

// WorktreeInfo contains information about a VCS worktree
type WorktreeInfo struct {
	Path   string
	Branch string
}

// Provider defines the interface for Version Control System operations.
// This abstraction allows Rig to support different VCS backends (like Git)
// and to offload VCS logic to plugins.
type Provider interface {
	// GetRepoRoot returns the root directory of the repository containing the path.
	GetRepoRoot(path string) (string, error)

	// GetRepoName returns the name of the repository.
	GetRepoName(path string) (string, error)

	// GetDefaultBranch determines the default branch of the repository.
	GetDefaultBranch(path, baseBranchConfig string) (string, error)

	// CreateWorktree creates a new worktree for the given ticket.
	CreateWorktree(path, ticketType, name, branchName, baseBranchConfig string) (string, error)

	// ListWorktrees returns a detailed list of all existing worktrees in the repository.
	ListWorktrees(path string) ([]WorktreeInfo, error)

	// RemoveWorktree removes a worktree.
	RemoveWorktree(path, ticketType, ticket string) error

	// ForceRemoveWorktree removes a worktree even if it has uncommitted changes.
	ForceRemoveWorktree(path, worktreePath string) error

	// GetWorktreePath returns the absolute path for a ticket's worktree.
	GetWorktreePath(path, ticketType, ticket string) (string, error)

	// Clone clones a repository from the given URL to the base path.
	Clone(url, basePath string) (string, error)

	// IsBranchMerged checks if the given branch is merged into the base branch.
	IsBranchMerged(path, branch, baseBranch string) (bool, error)
}
