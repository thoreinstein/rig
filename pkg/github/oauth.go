package github

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/cli/oauth"
	"github.com/cli/oauth/api"

	rigerrors "thoreinstein.com/rig/pkg/errors"
)

const (
	// DefaultGitHubHost is the default GitHub API host.
	DefaultGitHubHost = "https://github.com"

	// DefaultScopes are the OAuth scopes required for PR operations.
	DefaultScopes = "repo"
)

// OAuthConfig holds OAuth configuration for device flow authentication.
type OAuthConfig struct {
	ClientID string   // OAuth app client ID (required for device flow)
	Scopes   []string // OAuth scopes to request
	HostURL  string   // GitHub host URL (default: github.com)
}

// DeviceAuth performs OAuth device flow authentication.
// It displays a code for the user to enter at GitHub's verification URL,
// then polls until authorization completes.
//
// Flow:
//  1. Request device code from GitHub
//  2. Display code and URL to user
//  3. Poll for authorization
//  4. Return access token
func DeviceAuth(ctx context.Context, cfg OAuthConfig, stdout io.Writer) (*api.AccessToken, error) {
	if cfg.ClientID == "" {
		return nil, rigerrors.NewGitHubError("DeviceAuth", "client_id is required for OAuth device flow")
	}

	hostURL := cfg.HostURL
	if hostURL == "" {
		hostURL = DefaultGitHubHost
	}

	scopes := cfg.Scopes
	if len(scopes) == 0 {
		scopes = []string{DefaultScopes}
	}

	host, err := oauth.NewGitHubHost(hostURL)
	if err != nil {
		return nil, rigerrors.NewGitHubErrorWithCause("DeviceAuth", "invalid GitHub host URL", err)
	}

	// Set up the OAuth flow
	flow := &oauth.Flow{
		Host:     host,
		ClientID: cfg.ClientID,
		Scopes:   scopes,
		Stdout:   stdout,
		Stdin:    os.Stdin,
		DisplayCode: func(code, verificationURL string) error {
			fmt.Fprintf(stdout, "\n! First, copy your one-time code: %s\n", code)
			fmt.Fprintf(stdout, "- Press Enter to open %s in your browser...\n", verificationURL)
			return nil
		},
	}

	// Perform device flow (cli/oauth handles polling automatically)
	token, err := flow.DeviceFlow()
	if err != nil {
		return nil, rigerrors.NewGitHubErrorWithCause("DeviceAuth", "device flow failed", err)
	}

	return token, nil
}
