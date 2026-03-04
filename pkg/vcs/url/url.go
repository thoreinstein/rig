package url

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/cockroachdb/errors"
)

// RepoURL represents a parsed GitHub repository URL
type RepoURL struct {
	Original  string // Original input
	Canonical string // Normalized URL for cloning
	Protocol  string // "ssh" or "https"
	Owner     string // GitHub org/user
	Repo      string // Repository name (without .git)
}

// URL parsing patterns for GitHub repository URLs
var (
	// SSH format: git@github.com:owner/repo.git or git@github.com:owner/repo
	sshURLRegex = regexp.MustCompile(`^git@github\.com:([a-zA-Z0-9_.-]+)/([a-zA-Z0-9_.-]+?)(?:\.git)?$`)

	// HTTPS format: https://github.com/owner/repo or https://github.com/owner/repo.git
	httpsURLRegex = regexp.MustCompile(`^https://github\.com/([a-zA-Z0-9_.-]+)/([a-zA-Z0-9_.-]+?)(?:\.git)?$`)

	// Shorthand format: github.com/owner/repo (no protocol)
	shorthandURLRegex = regexp.MustCompile(`^github\.com/([a-zA-Z0-9_.-]+)/([a-zA-Z0-9_.-]+?)(?:\.git)?$`)

	// Shorthand format: owner/repo (interpreted as SSH by default)
	// Owner allows alphanumeric, hyphens, and underscores but excludes dots
	// to avoid matching domain-like strings (e.g. "github.com/owner").
	ownerRepoRegex = regexp.MustCompile(`^([a-zA-Z0-9_-]+)/([a-zA-Z0-9_.-]+?)(?:\.git)?$`)
)

// ParseGitHubURL parses various GitHub URL formats and returns a normalized RepoURL.
// Supported formats:
//   - SSH: git@github.com:owner/repo.git
//   - HTTPS: https://github.com/owner/repo
//   - Shorthand: github.com/owner/repo (interpreted as SSH by default)
//   - Shorthand: owner/repo (interpreted as SSH by default)
func ParseGitHubURL(input string) (*RepoURL, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, errors.New("empty URL provided")
	}

	// Try SSH format first
	if matches := sshURLRegex.FindStringSubmatch(input); len(matches) == 3 {
		return &RepoURL{
			Original:  input,
			Canonical: fmt.Sprintf("git@github.com:%s/%s.git", matches[1], matches[2]),
			Protocol:  "ssh",
			Owner:     matches[1],
			Repo:      matches[2],
		}, nil
	}

	// Try HTTPS format
	if matches := httpsURLRegex.FindStringSubmatch(input); len(matches) == 3 {
		return &RepoURL{
			Original:  input,
			Canonical: fmt.Sprintf("https://github.com/%s/%s.git", matches[1], matches[2]),
			Protocol:  "https",
			Owner:     matches[1],
			Repo:      matches[2],
		}, nil
	}

	// Try shorthand format (default to SSH)
	if matches := shorthandURLRegex.FindStringSubmatch(input); len(matches) == 3 {
		return &RepoURL{
			Original:  input,
			Canonical: fmt.Sprintf("git@github.com:%s/%s.git", matches[1], matches[2]),
			Protocol:  "ssh",
			Owner:     matches[1],
			Repo:      matches[2],
		}, nil
	}

	// Try owner/repo shorthand (default to SSH)
	if matches := ownerRepoRegex.FindStringSubmatch(input); len(matches) == 3 {
		return &RepoURL{
			Original:  input,
			Canonical: fmt.Sprintf("git@github.com:%s/%s.git", matches[1], matches[2]),
			Protocol:  "ssh",
			Owner:     matches[1],
			Repo:      matches[2],
		}, nil
	}

	return nil, errors.Newf("invalid GitHub URL format: %q\n\nSupported formats:\n  git@github.com:owner/repo.git (SSH)\n  https://github.com/owner/repo (HTTPS)\n  github.com/owner/repo (shorthand)\n  owner/repo (shorthand)", input)
}
