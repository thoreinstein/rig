// Package github provides GitHub integration for PR management.
//
// This package implements the Client interface for interacting with GitHub,
// supporting operations like creating PRs, merging, and branch management.
// The primary implementation uses the gh CLI tool for maximum compatibility.
package github

import "time"

// AuthMethod represents the authentication method for GitHub.
type AuthMethod string

const (
	// AuthToken uses a personal access token for authentication.
	AuthToken AuthMethod = "token"
	// AuthOAuth uses OAuth for authentication.
	AuthOAuth AuthMethod = "oauth"
	// AuthGHCLI uses the gh CLI's stored credentials.
	AuthGHCLI AuthMethod = "gh_cli"
)

// PRInfo represents pull request information.
type PRInfo struct {
	Number         int       `json:"number"`
	Title          string    `json:"title"`
	Body           string    `json:"body"`
	State          string    `json:"state"`   // "open", "closed", "merged"
	Draft          bool      `json:"isDraft"` // gh CLI uses isDraft
	URL            string    `json:"url"`
	HeadBranch     string    `json:"headRefName"`      // gh CLI uses headRefName
	BaseBranch     string    `json:"baseRefName"`      // gh CLI uses baseRefName
	Mergeable      string    `json:"mergeable"`        // gh CLI returns string: "MERGEABLE", "CONFLICTING", "UNKNOWN"
	MergeableState string    `json:"mergeStateStatus"` // gh CLI uses mergeStateStatus: "CLEAN", "DIRTY", "BLOCKED", etc.
	Reviewers      []string  `json:"-"`                // Populated from reviewRequests
	Approved       bool      `json:"-"`                // Computed from reviews
	ChecksPassing  bool      `json:"-"`                // Computed from statusCheckRollup
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// IsMergeable returns true if the PR has no merge conflicts.
func (pr *PRInfo) IsMergeable() bool {
	return pr.Mergeable == "MERGEABLE"
}

// IsClean returns true if the PR is in a clean state (checks pass, reviews approved).
func (pr *PRInfo) IsClean() bool {
	return pr.MergeableState == "CLEAN"
}

// CreatePROptions holds options for creating a pull request.
type CreatePROptions struct {
	Title      string   // PR title (required)
	Body       string   // PR body/description
	HeadBranch string   // Source branch (defaults to current branch)
	BaseBranch string   // Target branch (defaults to repo default branch)
	Draft      bool     // Create as draft PR
	Reviewers  []string // Requested reviewers
}

// MergeOptions holds options for merging a pull request.
type MergeOptions struct {
	Method       string // "merge", "squash", "rebase" (defaults to repo setting)
	CommitTitle  string // Custom commit title (optional)
	CommitBody   string // Custom commit body (optional)
	DeleteBranch bool   // Delete head branch after merge
}

// ghPRResponse represents the JSON response from gh pr view/list.
// Used internally for JSON parsing before converting to PRInfo.
type ghPRResponse struct {
	Number           int       `json:"number"`
	Title            string    `json:"title"`
	Body             string    `json:"body"`
	State            string    `json:"state"`
	IsDraft          bool      `json:"isDraft"`
	URL              string    `json:"url"`
	HeadRefName      string    `json:"headRefName"`
	BaseRefName      string    `json:"baseRefName"`
	Mergeable        string    `json:"mergeable"`
	MergeStateStatus string    `json:"mergeStateStatus"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
	ReviewRequests   []struct {
		Login string `json:"login"`
	} `json:"reviewRequests"`
	Reviews []struct {
		State  string `json:"state"`
		Author struct {
			Login string `json:"login"`
		} `json:"author"`
	} `json:"reviews"`
	StatusCheckRollup []struct {
		Context    string `json:"context"`
		State      string `json:"state"`
		Conclusion string `json:"conclusion"`
	} `json:"statusCheckRollup"`
}

// toPRInfo converts a ghPRResponse to PRInfo with computed fields.
func (r *ghPRResponse) toPRInfo() *PRInfo {
	pr := &PRInfo{
		Number:         r.Number,
		Title:          r.Title,
		Body:           r.Body,
		State:          r.State,
		Draft:          r.IsDraft,
		URL:            r.URL,
		HeadBranch:     r.HeadRefName,
		BaseBranch:     r.BaseRefName,
		Mergeable:      r.Mergeable,
		MergeableState: r.MergeStateStatus,
		CreatedAt:      r.CreatedAt,
		UpdatedAt:      r.UpdatedAt,
	}

	// Extract reviewers
	for _, req := range r.ReviewRequests {
		if req.Login != "" {
			pr.Reviewers = append(pr.Reviewers, req.Login)
		}
	}

	// Check if approved (any APPROVED review from a unique reviewer)
	approvedBy := make(map[string]bool)
	for _, review := range r.Reviews {
		if review.State == "APPROVED" {
			approvedBy[review.Author.Login] = true
		}
	}
	pr.Approved = len(approvedBy) > 0

	// Check if all checks pass
	pr.ChecksPassing = true
	for _, check := range r.StatusCheckRollup {
		if check.State == "FAILURE" || check.State == "ERROR" ||
			check.Conclusion == "FAILURE" || check.Conclusion == "ERROR" {
			pr.ChecksPassing = false
			break
		}
	}

	return pr
}

// ghRepoResponse represents the JSON response from gh repo view.
type ghRepoResponse struct {
	Name  string `json:"name"`
	Owner struct {
		Login string `json:"login"`
	} `json:"owner"`
	DefaultBranchRef struct {
		Name string `json:"name"`
	} `json:"defaultBranchRef"`
}
