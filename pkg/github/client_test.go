package github

import (
	"testing"
	"time"

	"thoreinstein.com/rig/pkg/config"
)

func TestPRInfoIsMergeable(t *testing.T) {
	tests := []struct {
		name      string
		mergeable string
		want      bool
	}{
		{"mergeable", "MERGEABLE", true},
		{"conflicting", "CONFLICTING", false},
		{"unknown", "UNKNOWN", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr := &PRInfo{Mergeable: tt.mergeable}
			if got := pr.IsMergeable(); got != tt.want {
				t.Errorf("PRInfo.IsMergeable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPRInfoIsClean(t *testing.T) {
	tests := []struct {
		name           string
		mergeableState string
		want           bool
	}{
		{"clean", "CLEAN", true},
		{"dirty", "DIRTY", false},
		{"blocked", "BLOCKED", false},
		{"unstable", "UNSTABLE", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr := &PRInfo{MergeableState: tt.mergeableState}
			if got := pr.IsClean(); got != tt.want {
				t.Errorf("PRInfo.IsClean() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGHPRResponseToPRInfo(t *testing.T) {
	now := time.Now()
	resp := &ghPRResponse{
		Number:           42,
		Title:            "Test PR",
		Body:             "Test body",
		State:            "open",
		IsDraft:          true,
		URL:              "https://github.com/owner/repo/pull/42",
		HeadRefName:      "feature-branch",
		BaseRefName:      "main",
		Mergeable:        "MERGEABLE",
		MergeStateStatus: "CLEAN",
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	resp.ReviewRequests = []struct {
		Login string `json:"login"`
	}{
		{Login: "reviewer1"},
		{Login: "reviewer2"},
	}
	resp.Reviews = []struct {
		State  string `json:"state"`
		Author struct {
			Login string `json:"login"`
		} `json:"author"`
	}{
		{State: "APPROVED", Author: struct {
			Login string `json:"login"`
		}{Login: "reviewer1"}},
	}

	pr := resp.toPRInfo()

	if pr.Number != 42 {
		t.Errorf("Number = %d, want 42", pr.Number)
	}
	if pr.Title != "Test PR" {
		t.Errorf("Title = %s, want Test PR", pr.Title)
	}
	if !pr.Draft {
		t.Error("Draft = false, want true")
	}
	if len(pr.Reviewers) != 2 {
		t.Errorf("len(Reviewers) = %d, want 2", len(pr.Reviewers))
	}
	if !pr.Approved {
		t.Error("Approved = false, want true (has APPROVED review)")
	}
	if !pr.ChecksPassing {
		t.Error("ChecksPassing = false, want true (no failed checks)")
	}
}

func TestGHPRResponseToPRInfo_FailedChecks(t *testing.T) {
	resp := &ghPRResponse{
		Number: 1,
	}
	resp.StatusCheckRollup = []struct {
		Context    string `json:"context"`
		State      string `json:"state"`
		Conclusion string `json:"conclusion"`
	}{
		{Context: "ci", State: "SUCCESS", Conclusion: "SUCCESS"},
		{Context: "lint", State: "FAILURE", Conclusion: "FAILURE"},
	}

	pr := resp.toPRInfo()

	if pr.ChecksPassing {
		t.Error("ChecksPassing = true, want false (has FAILURE check)")
	}
}

func TestNewClient_NilConfig(t *testing.T) {
	_, err := NewClient(nil, false)
	if err == nil {
		t.Error("NewClient(nil, false) should return error")
	}
}

func TestNewClient_OAuthNotImplemented(t *testing.T) {
	cfg := &config.GitHubConfig{
		AuthMethod: "oauth",
	}
	_, err := NewClient(cfg, false)
	if err == nil {
		t.Error("NewClient with oauth should return error (not implemented)")
	}
}

func TestNewClient_UnknownAuthMethod(t *testing.T) {
	cfg := &config.GitHubConfig{
		AuthMethod: "unknown",
	}
	_, err := NewClient(cfg, false)
	if err == nil {
		t.Error("NewClient with unknown auth should return error")
	}
}

func TestNewClient_TokenAuthMissingToken(t *testing.T) {
	cfg := &config.GitHubConfig{
		AuthMethod: "token",
		Token:      "", // No token
	}
	// Clear env var if set
	t.Setenv("RIG_GITHUB_TOKEN", "")

	_, err := NewClient(cfg, false)
	if err == nil {
		t.Error("NewClient with token auth but no token should return error")
	}
}

func TestExtractPRNumber(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		want    int
		wantErr bool
	}{
		{
			name: "valid url",
			url:  "https://github.com/owner/repo/pull/123",
			want: 123,
		},
		{
			name:    "valid url with trailing slash",
			url:     "https://github.com/owner/repo/pull/456/",
			want:    0, // Will fail because trailing slash leaves empty string
			wantErr: true,
		},
		{
			name:    "invalid - not a number",
			url:     "https://github.com/owner/repo/pull/abc",
			wantErr: true,
		},
		{
			name:    "invalid - too short",
			url:     "123",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractPRNumber(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractPRNumber() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("extractPRNumber() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsRetryableGHError(t *testing.T) {
	tests := []struct {
		name   string
		errMsg string
		want   bool
	}{
		{"rate limit", "API rate limit exceeded", true},
		{"timeout", "request timeout", true},
		{"connection refused", "connection refused", true},
		{"network error", "network error", true},
		{"502", "HTTP 502 Bad Gateway", true},
		{"503", "HTTP 503 Service Unavailable", true},
		{"504", "HTTP 504 Gateway Timeout", true},
		{"not found", "resource not found", false},
		{"unauthorized", "unauthorized", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRetryableGHError(tt.errMsg); got != tt.want {
				t.Errorf("isRetryableGHError(%q) = %v, want %v", tt.errMsg, got, tt.want)
			}
		})
	}
}

func TestCreatePROptions_EmptyTitle(t *testing.T) {
	// Skip if gh CLI is not available
	client, err := NewCLIClient(false)
	if err != nil {
		t.Skip("gh CLI not available")
	}

	_, err = client.CreatePR(t.Context(), CreatePROptions{
		Title: "", // Empty title should fail
	})
	if err == nil {
		t.Error("CreatePR with empty title should return error")
	}
}

func TestDeleteBranch_EmptyBranch(t *testing.T) {
	// Skip if gh CLI is not available
	client, err := NewCLIClient(false)
	if err != nil {
		t.Skip("gh CLI not available")
	}

	err = client.DeleteBranch(t.Context(), "")
	if err == nil {
		t.Error("DeleteBranch with empty branch should return error")
	}
}
