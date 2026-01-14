package github

import (
	"context"
	"os"

	"thoreinstein.com/rig/pkg/config"
	rigerrors "thoreinstein.com/rig/pkg/errors"
)

// Client defines the interface for GitHub operations.
// Implementations include CLIClient (wrapping gh CLI) and potentially
// future API-based clients.
type Client interface {
	// IsAuthenticated checks if the client is authenticated with GitHub.
	IsAuthenticated() bool

	// CreatePR creates a new pull request.
	CreatePR(ctx context.Context, opts CreatePROptions) (*PRInfo, error)

	// GetPR retrieves pull request information by number.
	GetPR(ctx context.Context, number int) (*PRInfo, error)

	// ListPRs lists pull requests filtered by state ("open", "closed", "merged", "all").
	ListPRs(ctx context.Context, state string) ([]PRInfo, error)

	// MergePR merges a pull request.
	MergePR(ctx context.Context, number int, opts MergeOptions) error

	// DeleteBranch deletes a branch from the remote repository.
	DeleteBranch(ctx context.Context, branch string) error

	// GetDefaultBranch returns the repository's default branch name.
	GetDefaultBranch(ctx context.Context) (string, error)

	// GetCurrentRepo returns the owner and repo name for the current repository.
	GetCurrentRepo(ctx context.Context) (owner, repo string, err error)
}

// Compile-time check that CLIClient implements Client.
var _ Client = (*CLIClient)(nil)

// NewClient creates a GitHub client based on the provided configuration.
//
// The function checks for the RIG_GITHUB_TOKEN environment variable first,
// which takes precedence over the config file token for token-based auth.
//
// Currently only CLIClient (wrapping gh CLI) is implemented. Future versions
// may add API-based clients for OAuth or direct API token authentication.
func NewClient(cfg *config.GitHubConfig, verbose bool) (Client, error) {
	if cfg == nil {
		return nil, rigerrors.NewGitHubError("NewClient", "github config is required")
	}

	// Check for environment variable token override
	token := os.Getenv("RIG_GITHUB_TOKEN")
	if token == "" {
		token = cfg.Token
	}

	// Determine which client to create based on auth method
	switch AuthMethod(cfg.AuthMethod) {
	case AuthToken:
		if token == "" {
			return nil, rigerrors.NewGitHubError("NewClient", "token auth requires RIG_GITHUB_TOKEN env var or github.token in config")
		}
		// For now, even token auth uses CLIClient since gh CLI can use GITHUB_TOKEN
		// Future: implement a direct API client
		return NewCLIClient(verbose, WithToken(token))

	case AuthOAuth:
		// OAuth not yet implemented, fall back to gh CLI
		return nil, rigerrors.NewGitHubError("NewClient", "oauth auth not yet implemented, use gh_cli auth method")

	case AuthGHCLI, "": // Default to gh CLI
		return NewCLIClient(verbose)

	default:
		return nil, rigerrors.NewGitHubError("NewClient", "unknown auth method: "+cfg.AuthMethod)
	}
}
