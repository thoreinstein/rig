package github

import (
	"context"
	"log/slog"
	"os"

	"golang.org/x/oauth2"

	"thoreinstein.com/rig/pkg/config"
	rigerrors "thoreinstein.com/rig/pkg/errors"
)

// Client defines the interface for GitHub operations.
// Implementations include CLIClient (wrapping gh CLI) and APIClient (using GitHub REST API).
type Client interface {
	// IsAuthenticated checks if the client is authenticated with GitHub.
	IsAuthenticated() bool

	// CreatePR creates a new pull request.
	CreatePR(ctx context.Context, opts CreatePROptions) (*PRInfo, error)

	// GetPR retrieves pull request information by number.
	GetPR(ctx context.Context, number int) (*PRInfo, error)

	// ListPRs lists pull requests filtered by state ("open", "closed", "merged", "all").
	// If author is non-empty, only PRs by that author are returned.
	// Use "@me" to filter by the authenticated user.
	ListPRs(ctx context.Context, state, author string) ([]PRInfo, error)

	// MergePR merges a pull request.
	MergePR(ctx context.Context, number int, opts MergeOptions) error

	// DeleteBranch deletes a branch from the remote repository.
	DeleteBranch(ctx context.Context, branch string) error

	// GetDefaultBranch returns the repository's default branch name.
	GetDefaultBranch(ctx context.Context) (string, error)

	// GetCurrentRepo returns the owner and repo name for the current repository.
	GetCurrentRepo(ctx context.Context) (owner, repo string, err error)
}

// Compile-time checks that implementations satisfy the Client interface.
var (
	_ Client = (*CLIClient)(nil)
	_ Client = (*APIClient)(nil)
)

// NewClient creates a GitHub client based on the provided configuration.
//
// Token resolution order:
//  1. GITHUB_TOKEN environment variable
//  2. RIG_GITHUB_TOKEN environment variable
//  3. Token from config file (github.token)
//  4. Cached OAuth token (keychain or file)
//  5. OAuth device flow (if client_id configured)
//  6. Fall back to gh CLI
func NewClient(cfg *config.GitHubConfig, verbose bool) (Client, error) {
	if cfg == nil {
		return nil, rigerrors.NewGitHubError("NewClient", "github config is required")
	}

	// Check environment variable tokens first (highest precedence)
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		token = os.Getenv("RIG_GITHUB_TOKEN")
	}
	if token == "" {
		token = cfg.Token
	}

	// Determine which client to create based on auth method
	switch AuthMethod(cfg.AuthMethod) {
	case AuthToken:
		if token == "" {
			return nil, rigerrors.NewGitHubError("NewClient",
				"token auth requires GITHUB_TOKEN, RIG_GITHUB_TOKEN env var, or github.token in config")
		}
		return NewAPIClient(token, verbose)

	case AuthOAuth:
		return newOAuthClient(cfg, verbose)

	case AuthGHCLI, "":
		// Default: prefer API client if we have a token, fall back to CLI
		if token != "" {
			return NewAPIClient(token, verbose)
		}
		return NewCLIClient(verbose)

	default:
		return nil, rigerrors.NewGitHubError("NewClient", "unknown auth method: "+cfg.AuthMethod)
	}
}

// newOAuthClient creates a client using OAuth device flow with token caching.
func newOAuthClient(cfg *config.GitHubConfig, verbose bool) (Client, error) {
	cache := NewTokenCache()

	// Try cached token first
	cachedToken, err := cache.Get()
	if err != nil {
		// Log but don't fail - we can try device flow
		if verbose {
			slog.Debug("failed to read cached token", "error", err)
		}
	}

	if cachedToken != nil && cachedToken.Valid() {
		if verbose {
			slog.Debug("using cached OAuth token")
		}
		return NewAPIClient(cachedToken.AccessToken, verbose)
	}

	// No valid cached token - need to do device flow
	if cfg.ClientID == "" {
		return nil, rigerrors.NewGitHubError("NewClient",
			"oauth auth requires github.client_id in config; alternatively use gh_cli auth method")
	}

	oauthCfg := OAuthConfig{
		ClientID: cfg.ClientID,
		Scopes:   []string{"repo", "read:org"},
	}

	// Perform device flow authentication
	apiToken, err := DeviceAuth(context.Background(), oauthCfg, os.Stdout)
	if err != nil {
		return nil, err
	}

	// Convert to oauth2.Token and cache it
	token := &oauth2.Token{
		AccessToken: apiToken.Token,
		TokenType:   apiToken.Type,
	}

	if cacheErr := cache.Set(token); cacheErr != nil {
		// Log but don't fail - auth succeeded
		if verbose {
			slog.Debug("failed to cache token", "error", cacheErr)
		}
	} else if verbose {
		slog.Debug("cached OAuth token for future use")
	}

	return NewAPIClient(token.AccessToken, verbose)
}
