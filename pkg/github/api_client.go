package github

import (
	"context"
	"log/slog"
	"os/exec"
	"strings"

	gh "github.com/google/go-github/v68/github"
	"golang.org/x/oauth2"

	rigerrors "thoreinstein.com/rig/pkg/errors"
)

// APIClient implements Client using GitHub REST API.
type APIClient struct {
	client  *gh.Client
	verbose bool
	logger  *slog.Logger
}

// Compile-time check that APIClient implements Client.
var _ Client = (*APIClient)(nil)

// APIClientOption is a functional option for configuring APIClient.
type APIClientOption func(*APIClient)

// WithAPILogger sets a custom logger for the API client.
func WithAPILogger(logger *slog.Logger) APIClientOption {
	return func(c *APIClient) {
		c.logger = logger
	}
}

// NewAPIClient creates a GitHub API client with the given token.
func NewAPIClient(token string, verbose bool, opts ...APIClientOption) (*APIClient, error) {
	if token == "" {
		return nil, rigerrors.NewGitHubError("NewAPIClient", "token is required")
	}

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(context.Background(), ts)

	client := &APIClient{
		client:  gh.NewClient(tc),
		verbose: verbose,
		logger:  slog.Default(),
	}

	for _, opt := range opts {
		opt(client)
	}

	return client, nil
}

// IsAuthenticated checks if the client is authenticated with GitHub.
func (c *APIClient) IsAuthenticated() bool {
	ctx := context.Background()
	_, _, err := c.client.Users.Get(ctx, "")
	return err == nil
}

// CreatePR creates a new pull request.
func (c *APIClient) CreatePR(ctx context.Context, opts CreatePROptions) (*PRInfo, error) {
	if opts.Title == "" {
		return nil, rigerrors.NewGitHubError("CreatePR", "title is required")
	}

	owner, repo, err := c.GetCurrentRepo(ctx)
	if err != nil {
		return nil, err
	}

	// Determine base branch if not specified
	base := opts.BaseBranch
	if base == "" {
		base, err = c.GetDefaultBranch(ctx)
		if err != nil {
			return nil, err
		}
	}

	// Determine head branch if not specified
	head := opts.HeadBranch
	if head == "" {
		head, err = getCurrentBranch()
		if err != nil {
			return nil, rigerrors.NewGitHubErrorWithCause("CreatePR", "failed to get current branch", err)
		}
	}

	c.logDebug("creating PR", "owner", owner, "repo", repo, "head", head, "base", base)

	newPR := &gh.NewPullRequest{
		Title: gh.Ptr(opts.Title),
		Head:  gh.Ptr(head),
		Base:  gh.Ptr(base),
		Body:  gh.Ptr(opts.Body),
		Draft: gh.Ptr(opts.Draft),
	}

	pr, resp, err := c.client.PullRequests.Create(ctx, owner, repo, newPR)
	if err != nil {
		return nil, toGitHubError("CreatePR", resp, err)
	}

	// Request reviewers if specified
	if len(opts.Reviewers) > 0 {
		_, _, reviewErr := c.client.PullRequests.RequestReviewers(ctx, owner, repo, pr.GetNumber(), gh.ReviewersRequest{
			Reviewers: opts.Reviewers,
		})
		if reviewErr != nil {
			c.logDebug("failed to request reviewers", "error", reviewErr)
			// Don't fail the whole operation for reviewer request failure
		}
	}

	return prInfoFromGitHub(pr), nil
}

// GetPR retrieves pull request information by number.
func (c *APIClient) GetPR(ctx context.Context, number int) (*PRInfo, error) {
	owner, repo, err := c.GetCurrentRepo(ctx)
	if err != nil {
		return nil, err
	}

	c.logDebug("getting PR", "number", number)

	pr, resp, err := c.client.PullRequests.Get(ctx, owner, repo, number)
	if err != nil {
		return nil, toGitHubError("GetPR", resp, err)
	}

	info := prInfoFromGitHub(pr)

	// Fetch reviews to determine approval status
	reviews, _, reviewErr := c.client.PullRequests.ListReviews(ctx, owner, repo, number, nil)
	if reviewErr == nil {
		info.Approved = hasApprovedReview(reviews)
	}

	// Fetch status checks
	ref := pr.GetHead().GetSHA()
	if ref != "" {
		combined, _, statusErr := c.client.Repositories.GetCombinedStatus(ctx, owner, repo, ref, nil)
		if statusErr == nil {
			info.ChecksPassing = combined.GetState() == "success"
		}
	}

	return info, nil
}

// ListPRs lists pull requests filtered by state and optionally by author.
func (c *APIClient) ListPRs(ctx context.Context, opts ListPRsOptions) ([]PRInfo, error) {
	owner, repo, err := c.GetCurrentRepo(ctx)
	if err != nil {
		return nil, err
	}

	c.logDebug("listing PRs", "state", opts.State, "author", opts.Author, "limit", opts.Limit, "page", opts.Page)

	ghOpts := &gh.PullRequestListOptions{
		State: opts.State,
	}
	if opts.State == "" || opts.State == "all" {
		ghOpts.State = "all"
	}

	if opts.Limit > 0 {
		ghOpts.PerPage = opts.Limit
	}
	if opts.Page > 0 {
		ghOpts.Page = opts.Page
	}

	prs, resp, err := c.client.PullRequests.List(ctx, owner, repo, ghOpts)
	if err != nil {
		return nil, toGitHubError("ListPRs", resp, err)
	}

	// Get current user login if author is "@me"
	filterAuthor := opts.Author
	if opts.Author == "@me" {
		user, _, err := c.client.Users.Get(ctx, "")
		if err != nil {
			return nil, toGitHubError("ListPRs", nil, err)
		}
		filterAuthor = user.GetLogin()
	}

	result := make([]PRInfo, 0, len(prs))
	for _, pr := range prs {
		info := prInfoFromGitHub(pr)
		// Filter by author if specified
		if filterAuthor != "" && info.Author != filterAuthor {
			continue
		}
		result = append(result, *info)
	}

	return result, nil
}

// MergePR merges a pull request.
func (c *APIClient) MergePR(ctx context.Context, number int, opts MergeOptions) error {
	owner, repo, err := c.GetCurrentRepo(ctx)
	if err != nil {
		return err
	}

	c.logDebug("merging PR", "number", number, "method", opts.Method)

	mergeOpts := &gh.PullRequestOptions{}
	switch opts.Method {
	case "merge":
		mergeOpts.MergeMethod = "merge"
	case "squash":
		mergeOpts.MergeMethod = "squash"
	case "rebase":
		mergeOpts.MergeMethod = "rebase"
	}

	commitMsg := opts.CommitTitle
	if opts.CommitBody != "" {
		commitMsg = opts.CommitTitle + "\n\n" + opts.CommitBody
	}

	_, resp, err := c.client.PullRequests.Merge(ctx, owner, repo, number, commitMsg, mergeOpts)
	if err != nil {
		return toGitHubError("MergePR", resp, err)
	}

	// Delete branch if requested
	if opts.DeleteBranch {
		pr, _, prErr := c.client.PullRequests.Get(ctx, owner, repo, number)
		if prErr == nil && pr.GetHead() != nil {
			branch := pr.GetHead().GetRef()
			if branch != "" {
				deleteErr := c.DeleteBranch(ctx, branch)
				if deleteErr != nil {
					c.logDebug("failed to delete branch after merge", "branch", branch, "error", deleteErr)
				}
			}
		}
	}

	return nil
}

// DeleteBranch deletes a branch from the remote repository.
func (c *APIClient) DeleteBranch(ctx context.Context, branch string) error {
	if branch == "" {
		return rigerrors.NewGitHubError("DeleteBranch", "branch name is required")
	}

	owner, repo, err := c.GetCurrentRepo(ctx)
	if err != nil {
		return err
	}

	c.logDebug("deleting branch", "branch", branch)

	ref := "heads/" + branch
	resp, err := c.client.Git.DeleteRef(ctx, owner, repo, ref)
	if err != nil {
		return toGitHubError("DeleteBranch", resp, err)
	}

	return nil
}

// GetDefaultBranch returns the repository's default branch name.
func (c *APIClient) GetDefaultBranch(ctx context.Context) (string, error) {
	owner, repo, err := c.GetCurrentRepo(ctx)
	if err != nil {
		return "", err
	}

	c.logDebug("getting default branch")

	repository, resp, err := c.client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		return "", toGitHubError("GetDefaultBranch", resp, err)
	}

	return repository.GetDefaultBranch(), nil
}

// GetCurrentRepo returns the owner and repo name for the current repository.
// This parses the git remote URL to determine owner/repo.
func (c *APIClient) GetCurrentRepo(ctx context.Context) (owner, repo string, err error) {
	// Parse from git remote
	owner, repo, err = parseGitRemote()
	if err != nil {
		return "", "", rigerrors.NewGitHubErrorWithCause("GetCurrentRepo", "failed to parse git remote", err)
	}
	return owner, repo, nil
}

func (c *APIClient) logDebug(msg string, args ...any) {
	if c.verbose {
		c.logger.Debug(msg, args...)
	}
}

// Helper functions

func prInfoFromGitHub(pr *gh.PullRequest) *PRInfo {
	info := &PRInfo{
		Number:    pr.GetNumber(),
		Title:     pr.GetTitle(),
		Body:      pr.GetBody(),
		State:     pr.GetState(),
		Draft:     pr.GetDraft(),
		URL:       pr.GetHTMLURL(),
		Author:    pr.GetUser().GetLogin(),
		CreatedAt: pr.GetCreatedAt().Time,
		UpdatedAt: pr.GetUpdatedAt().Time,
	}

	if pr.Head != nil {
		info.HeadBranch = pr.GetHead().GetRef()
	}
	if pr.Base != nil {
		info.BaseBranch = pr.GetBase().GetRef()
	}

	// Map mergeable status
	if pr.Mergeable != nil {
		if *pr.Mergeable {
			info.Mergeable = "MERGEABLE"
		} else {
			info.Mergeable = "CONFLICTING"
		}
	} else {
		info.Mergeable = "UNKNOWN"
	}

	// Map mergeable state
	info.MergeableState = strings.ToUpper(pr.GetMergeableState())

	return info
}

func hasApprovedReview(reviews []*gh.PullRequestReview) bool {
	approvers := make(map[string]bool)
	for _, review := range reviews {
		if review.GetState() == "APPROVED" {
			approvers[review.GetUser().GetLogin()] = true
		}
	}
	return len(approvers) > 0
}

func toGitHubError(operation string, resp *gh.Response, err error) error {
	if resp != nil && resp.StatusCode > 0 {
		return rigerrors.NewGitHubErrorWithStatus(operation, resp.StatusCode, err.Error())
	}
	return rigerrors.NewGitHubErrorWithCause(operation, "API request failed", err)
}

func parseGitRemote() (owner, repo string, err error) {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	output, err := cmd.Output()
	if err != nil {
		return "", "", err
	}

	url := strings.TrimSpace(string(output))
	return parseGitHubURL(url)
}

func parseGitHubURL(url string) (owner, repo string, err error) {
	// Handle SSH format: git@github.com:owner/repo.git
	if strings.HasPrefix(url, "git@") {
		parts := strings.Split(url, ":")
		if len(parts) != 2 {
			return "", "", rigerrors.NewGitHubError("parseGitHubURL", "invalid SSH URL format")
		}
		path := strings.TrimSuffix(parts[1], ".git")
		segments := strings.Split(path, "/")
		if len(segments) != 2 {
			return "", "", rigerrors.NewGitHubError("parseGitHubURL", "invalid repository path")
		}
		return segments[0], segments[1], nil
	}

	// Handle HTTPS format: https://github.com/owner/repo.git
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")
	url = strings.TrimSuffix(url, ".git")

	parts := strings.Split(url, "/")
	if len(parts) < 3 {
		return "", "", rigerrors.NewGitHubError("parseGitHubURL", "invalid HTTPS URL format")
	}

	return parts[1], parts[2], nil
}

func getCurrentBranch() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}
