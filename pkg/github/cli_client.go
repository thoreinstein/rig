package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
	"strings"

	rigerrors "thoreinstein.com/rig/pkg/errors"
)

// CLIClient implements the Client interface using the gh CLI.
// This is the primary implementation as most users have gh CLI installed
// and it handles authentication automatically.
type CLIClient struct {
	verbose bool
	token   string // Optional token for GITHUB_TOKEN env override
	logger  *slog.Logger
}

// CLIClientOption is a functional option for configuring CLIClient.
type CLIClientOption func(*CLIClient)

// WithToken sets a token to be used via GITHUB_TOKEN environment variable.
func WithToken(token string) CLIClientOption {
	return func(c *CLIClient) {
		c.token = token
	}
}

// WithLogger sets a custom logger for the client.
func WithLogger(logger *slog.Logger) CLIClientOption {
	return func(c *CLIClient) {
		c.logger = logger
	}
}

// NewCLIClient creates a new gh CLI-based GitHub client.
func NewCLIClient(verbose bool, opts ...CLIClientOption) (*CLIClient, error) {
	c := &CLIClient{
		verbose: verbose,
		logger:  slog.Default(),
	}

	for _, opt := range opts {
		opt(c)
	}

	// Verify gh CLI is available
	if _, err := exec.LookPath("gh"); err != nil {
		return nil, rigerrors.NewGitHubErrorWithCause("NewCLIClient", "gh CLI not found in PATH", err)
	}

	return c, nil
}

// IsAuthenticated checks if gh CLI is authenticated with GitHub.
func (c *CLIClient) IsAuthenticated() bool {
	cmd := exec.Command("gh", "auth", "status")
	if c.token != "" {
		cmd.Env = append(os.Environ(), "GITHUB_TOKEN="+c.token)
	}
	return cmd.Run() == nil
}

// CreatePR creates a new pull request using gh pr create.
func (c *CLIClient) CreatePR(ctx context.Context, opts CreatePROptions) (*PRInfo, error) {
	if opts.Title == "" {
		return nil, rigerrors.NewGitHubError("CreatePR", "title is required")
	}

	// Always pass --body (even if empty) because gh requires both --title and --body
	// when running non-interactively
	args := []string{"pr", "create", "--title", opts.Title, "--body", opts.Body}
	if opts.HeadBranch != "" {
		args = append(args, "--head", opts.HeadBranch)
	}
	if opts.BaseBranch != "" {
		args = append(args, "--base", opts.BaseBranch)
	}
	if opts.Draft {
		args = append(args, "--draft")
	}
	for _, reviewer := range opts.Reviewers {
		args = append(args, "--reviewer", reviewer)
	}

	c.logDebug("creating PR", "args", args)

	output, err := c.runGH(ctx, args...)
	if err != nil {
		return nil, rigerrors.NewGitHubErrorWithCause("CreatePR", "failed to create PR", err)
	}

	// gh pr create outputs the PR URL on success
	// We need to fetch the PR details to get full info
	prURL := strings.TrimSpace(output)
	c.logDebug("PR created", "url", prURL)

	// Extract PR number from URL and fetch details
	number, parseErr := extractPRNumber(prURL)
	if parseErr != nil {
		// Return minimal info if we can't parse the URL
		c.logDebug("could not parse PR number from URL, returning minimal info", "url", prURL, "error", parseErr)
		return &PRInfo{URL: prURL, Title: opts.Title, Draft: opts.Draft}, nil
	}

	return c.GetPR(ctx, number)
}

// GetPR retrieves pull request information by number.
func (c *CLIClient) GetPR(ctx context.Context, number int) (*PRInfo, error) {
	fields := prJSONFields()
	args := []string{
		"pr", "view", strconv.Itoa(number),
		"--json", strings.Join(fields, ","),
	}

	c.logDebug("getting PR", "number", number)

	output, err := c.runGH(ctx, args...)
	if err != nil {
		return nil, rigerrors.NewGitHubErrorWithCause("GetPR", fmt.Sprintf("failed to get PR #%d", number), err)
	}

	var resp ghPRResponse
	if err := json.Unmarshal([]byte(output), &resp); err != nil {
		return nil, rigerrors.NewGitHubErrorWithCause("GetPR", "failed to parse PR response", err)
	}

	return resp.toPRInfo(), nil
}

// ListPRs lists pull requests filtered by state and optionally by author.
func (c *CLIClient) ListPRs(ctx context.Context, opts ListPRsOptions) ([]PRInfo, error) {
	fields := prJSONFields()
	args := []string{
		"pr", "list",
		"--json", strings.Join(fields, ","),
	}

	if opts.State != "" && opts.State != "all" {
		args = append(args, "--state", opts.State)
	}

	if opts.Author != "" {
		args = append(args, "--author", opts.Author)
	}

	if opts.Limit > 0 {
		args = append(args, "--limit", strconv.Itoa(opts.Limit))
	}

	// Note: gh pr list doesn't support --page directly.
	// If page > 1, we would need to use gh api or fetch all and slice.
	// For now, we only support --limit.
	if opts.Page > 1 {
		c.logDebug("pagination (page > 1) is not supported by gh CLI client, ignoring page parameter")
	}

	c.logDebug("listing PRs", "state", opts.State, "author", opts.Author, "limit", opts.Limit)

	output, err := c.runGH(ctx, args...)
	if err != nil {
		return nil, rigerrors.NewGitHubErrorWithCause("ListPRs", "failed to list PRs", err)
	}

	var responses []ghPRResponse
	if err := json.Unmarshal([]byte(output), &responses); err != nil {
		return nil, rigerrors.NewGitHubErrorWithCause("ListPRs", "failed to parse PR list response", err)
	}

	prs := make([]PRInfo, 0, len(responses))
	for _, resp := range responses {
		prs = append(prs, *resp.toPRInfo())
	}

	return prs, nil
}

// MergePR merges a pull request.
func (c *CLIClient) MergePR(ctx context.Context, number int, opts MergeOptions) error {
	args := []string{"pr", "merge", strconv.Itoa(number)}

	switch opts.Method {
	case "merge":
		args = append(args, "--merge")
	case "squash":
		args = append(args, "--squash")
	case "rebase":
		args = append(args, "--rebase")
	default:
		// Use repo default if not specified
	}

	if opts.CommitTitle != "" {
		args = append(args, "--subject", opts.CommitTitle)
	}
	if opts.CommitBody != "" {
		args = append(args, "--body", opts.CommitBody)
	}
	// Note: --delete-branch is intentionally NOT passed here.
	// Branch deletion is handled separately via the API to avoid worktree conflicts
	// when gh tries to checkout main to delete the local branch.

	c.logDebug("merging PR", "number", number, "method", opts.Method)

	_, err := c.runGH(ctx, args...)
	if err != nil {
		return rigerrors.NewGitHubErrorWithCause("MergePR", fmt.Sprintf("failed to merge PR #%d", number), err)
	}

	return nil
}

// DeleteBranch deletes a branch from the remote repository.
func (c *CLIClient) DeleteBranch(ctx context.Context, branch string) error {
	if branch == "" {
		return rigerrors.NewGitHubError("DeleteBranch", "branch name is required")
	}

	// Get repo info first
	owner, repo, err := c.GetCurrentRepo(ctx)
	if err != nil {
		return err
	}

	// Use gh api to delete the branch ref
	endpoint := fmt.Sprintf("repos/%s/%s/git/refs/heads/%s", owner, repo, branch)
	args := []string{"api", endpoint, "-X", "DELETE"}

	c.logDebug("deleting branch", "branch", branch, "endpoint", endpoint)

	_, err = c.runGH(ctx, args...)
	if err != nil {
		return rigerrors.NewGitHubErrorWithCause("DeleteBranch", "failed to delete branch "+branch, err)
	}

	return nil
}

// GetDefaultBranch returns the repository's default branch name.
func (c *CLIClient) GetDefaultBranch(ctx context.Context) (string, error) {
	args := []string{"repo", "view", "--json", "defaultBranchRef"}

	c.logDebug("getting default branch")

	output, err := c.runGH(ctx, args...)
	if err != nil {
		return "", rigerrors.NewGitHubErrorWithCause("GetDefaultBranch", "failed to get default branch", err)
	}

	var resp ghRepoResponse
	if err := json.Unmarshal([]byte(output), &resp); err != nil {
		return "", rigerrors.NewGitHubErrorWithCause("GetDefaultBranch", "failed to parse repo response", err)
	}

	return resp.DefaultBranchRef.Name, nil
}

// GetCurrentRepo returns the owner and repo name for the current repository.
func (c *CLIClient) GetCurrentRepo(ctx context.Context) (owner, repo string, err error) {
	args := []string{"repo", "view", "--json", "owner,name"}

	c.logDebug("getting current repo")

	output, err := c.runGH(ctx, args...)
	if err != nil {
		return "", "", rigerrors.NewGitHubErrorWithCause("GetCurrentRepo", "failed to get repo info", err)
	}

	var resp ghRepoResponse
	if err := json.Unmarshal([]byte(output), &resp); err != nil {
		return "", "", rigerrors.NewGitHubErrorWithCause("GetCurrentRepo", "failed to parse repo response", err)
	}

	return resp.Owner.Login, resp.Name, nil
}

// runGH executes a gh command and returns its output.
func (c *CLIClient) runGH(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "gh", args...)

	// Set GITHUB_TOKEN if configured
	if c.token != "" {
		cmd.Env = append(os.Environ(), "GITHUB_TOKEN="+c.token)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = err.Error()
		}
		// Check for specific error patterns to determine retryability
		ghErr := rigerrors.NewGitHubError("gh", errMsg)
		if isRetryableGHError(errMsg) {
			ghErr.Retryable = true
		}
		return "", ghErr
	}

	return stdout.String(), nil
}

// logDebug logs a debug message if verbose mode is enabled.
func (c *CLIClient) logDebug(msg string, args ...any) {
	if c.verbose {
		c.logger.Debug(msg, args...)
	}
}

// prJSONFields returns the list of fields to request from gh pr view/list.
func prJSONFields() []string {
	return []string{
		"number",
		"title",
		"body",
		"state",
		"isDraft",
		"url",
		"headRefName",
		"baseRefName",
		"mergeable",
		"mergeStateStatus",
		"createdAt",
		"updatedAt",
		"reviewRequests",
		"reviews",
		"statusCheckRollup",
	}
}

// extractPRNumber extracts the PR number from a GitHub PR URL.
func extractPRNumber(url string) (int, error) {
	// URL format: https://github.com/owner/repo/pull/123
	parts := strings.Split(url, "/")
	if len(parts) < 2 {
		return 0, rigerrors.NewGitHubError("extractPRNumber", "invalid PR URL format")
	}
	numberStr := parts[len(parts)-1]
	number, err := strconv.Atoi(numberStr)
	if err != nil {
		return 0, rigerrors.NewGitHubErrorWithCause("extractPRNumber", "failed to parse PR number", err)
	}
	return number, nil
}

// isRetryableGHError checks if a gh CLI error message indicates a retryable error.
func isRetryableGHError(errMsg string) bool {
	retryablePatterns := []string{
		"rate limit",
		"timeout",
		"connection refused",
		"network",
		"502",
		"503",
		"504",
	}

	lowerErr := strings.ToLower(errMsg)
	for _, pattern := range retryablePatterns {
		if strings.Contains(lowerErr, pattern) {
			return true
		}
	}
	return false
}
