package workflow

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"thoreinstein.com/rig/pkg/config"
	"thoreinstein.com/rig/pkg/github"
	"thoreinstein.com/rig/pkg/jira"
)

// mockGitHubClient implements github.Client for testing.
type mockGitHubClient struct {
	pr          *github.PRInfo
	prError     error
	mergeError  error
	mergeCalled bool
}

func (m *mockGitHubClient) IsAuthenticated() bool { return true }

func (m *mockGitHubClient) CreatePR(_ context.Context, _ github.CreatePROptions) (*github.PRInfo, error) {
	return nil, nil
}

func (m *mockGitHubClient) GetPR(_ context.Context, _ int) (*github.PRInfo, error) {
	if m.prError != nil {
		return nil, m.prError
	}
	return m.pr, nil
}

func (m *mockGitHubClient) ListPRs(_ context.Context, _, _ string) ([]github.PRInfo, error) {
	return nil, nil
}

func (m *mockGitHubClient) MergePR(_ context.Context, _ int, _ github.MergeOptions) error {
	m.mergeCalled = true
	return m.mergeError
}

func (m *mockGitHubClient) DeleteBranch(_ context.Context, _ string) error {
	return nil
}

func (m *mockGitHubClient) GetDefaultBranch(_ context.Context) (string, error) {
	return "main", nil
}

func (m *mockGitHubClient) GetCurrentRepo(_ context.Context) (string, string, error) {
	return "owner", "repo", nil
}

// mockJiraClient implements jira.JiraClient for testing.
type mockJiraClient struct {
	ticketInfo      *jira.TicketInfo
	ticketError     error
	transitionError error
}

func (m *mockJiraClient) IsAvailable() bool { return true }

func (m *mockJiraClient) FetchTicketDetails(_ string) (*jira.TicketInfo, error) {
	if m.ticketError != nil {
		return nil, m.ticketError
	}
	return m.ticketInfo, nil
}

func (m *mockJiraClient) GetTransitions(_ string) ([]jira.Transition, error) {
	return nil, nil
}

func (m *mockJiraClient) TransitionTicket(_, _ string) error {
	return m.transitionError
}

func (m *mockJiraClient) TransitionTicketByName(_, _ string) error {
	return m.transitionError
}

func TestExtractTicketFromBranch(t *testing.T) {
	tests := []struct {
		name     string
		branch   string
		expected string
	}{
		{
			name:     "simple ticket",
			branch:   "PROJ-123",
			expected: "PROJ-123",
		},
		{
			name:     "feature branch",
			branch:   "feature/PROJ-456",
			expected: "PROJ-456",
		},
		{
			name:     "lowercase",
			branch:   "proj-789",
			expected: "proj-789",
		},
		{
			name:     "with underscore prefix",
			branch:   "user_PROJ-101",
			expected: "PROJ-101",
		},
		{
			name:     "no ticket",
			branch:   "main",
			expected: "",
		},
		{
			name:     "branch with dashes looks like ticket now",
			branch:   "feature-branch-name",
			expected: "feature-branch", // With alphanumeric IDs, this matches ticket pattern
		},
		{
			name:     "beads-style simple",
			branch:   "rig-abc123",
			expected: "rig-abc123",
		},
		{
			name:     "beads-style with prefix",
			branch:   "feature/rig-2o1",
			expected: "rig-2o1",
		},
		{
			name:     "beads-style with underscore prefix",
			branch:   "user_beads-xyz",
			expected: "beads-xyz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractTicketFromBranch(tt.branch)
			if result != tt.expected {
				t.Errorf("extractTicketFromBranch(%q) = %q, want %q", tt.branch, result, tt.expected)
			}
		})
	}
}

func TestLooksLikeTicket(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"PROJ-123", true},
		{"ABC-1", true},
		{"proj-456", true},
		{"A-1", true},
		// Beads-style alphanumeric identifiers
		{"rig-abc", true},
		{"beads-xyz123", true},
		{"rig-2o1", true},
		{"proj-AbC", true},
		// Invalid formats
		{"main", false},
		{"feature", false},
		{"123-ABC", false},
		{"-123", false},
		{"PROJ-", false},
		{"", false},
		{"AB", false},
		{"proj-123-456", false}, // multiple dashes not supported in suffix
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := looksLikeTicket(tt.input)
			if result != tt.expected {
				t.Errorf("looksLikeTicket(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsInReviewStatus(t *testing.T) {
	tests := []struct {
		status   string
		expected bool
	}{
		{"In Review", true},
		{"CODE REVIEW", true},
		{"Review", true},
		{"  in review  ", true},
		{"Done", false},
		{"Open", false},
		{"In Progress", false},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			result := isInReviewStatus(tt.status)
			if result != tt.expected {
				t.Errorf("isInReviewStatus(%q) = %v, want %v", tt.status, result, tt.expected)
			}
		})
	}
}

func TestPreflightResult_IsReady(t *testing.T) {
	tests := []struct {
		name     string
		result   PreflightResult
		expected bool
	}{
		{
			name: "all checks pass",
			result: PreflightResult{
				PRExists:      true,
				PROpen:        true,
				PRApproved:    true,
				ChecksPassing: true,
				JiraInReview:  true,
			},
			expected: true,
		},
		{
			name: "all checks pass, jira skipped",
			result: PreflightResult{
				PRExists:      true,
				PROpen:        true,
				PRApproved:    true,
				ChecksPassing: true,
				JiraSkipped:   true,
			},
			expected: true,
		},
		{
			name: "PR not approved",
			result: PreflightResult{
				PRExists:      true,
				PROpen:        true,
				PRApproved:    false,
				ChecksPassing: true,
				JiraInReview:  true,
			},
			expected: false,
		},
		{
			name: "checks failing",
			result: PreflightResult{
				PRExists:      true,
				PROpen:        true,
				PRApproved:    true,
				ChecksPassing: false,
				JiraInReview:  true,
			},
			expected: false,
		},
		{
			name: "PR not open",
			result: PreflightResult{
				PRExists:      true,
				PROpen:        false,
				PRApproved:    true,
				ChecksPassing: true,
				JiraInReview:  true,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.result.IsReady()
			if result != tt.expected {
				t.Errorf("IsReady() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCheckpointOperations(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()

	checkpoint := &Checkpoint{
		PRNumber:       42,
		Ticket:         "TEST-123",
		Worktree:       tmpDir,
		CompletedSteps: []Step{StepPreflight, StepGather},
		CurrentStep:    StepDebrief,
		CreatedAt:      time.Now().Add(-time.Hour),
		UpdatedAt:      time.Now(),
	}

	// Test SaveCheckpoint
	err := SaveCheckpoint(tmpDir, checkpoint)
	if err != nil {
		t.Fatalf("SaveCheckpoint failed: %v", err)
	}

	// Verify file was created
	checkpointFile := filepath.Join(tmpDir, ".rig", "checkpoint.json")
	if _, err := os.Stat(checkpointFile); os.IsNotExist(err) {
		t.Error("Checkpoint file was not created")
	}

	// Test HasCheckpoint
	if !HasCheckpoint(tmpDir) {
		t.Error("HasCheckpoint returned false, want true")
	}

	// Test LoadCheckpoint
	loaded, err := LoadCheckpoint(tmpDir)
	if err != nil {
		t.Fatalf("LoadCheckpoint failed: %v", err)
	}
	if loaded.PRNumber != checkpoint.PRNumber {
		t.Errorf("PRNumber = %d, want %d", loaded.PRNumber, checkpoint.PRNumber)
	}
	if loaded.Ticket != checkpoint.Ticket {
		t.Errorf("Ticket = %q, want %q", loaded.Ticket, checkpoint.Ticket)
	}
	if len(loaded.CompletedSteps) != len(checkpoint.CompletedSteps) {
		t.Errorf("CompletedSteps length = %d, want %d", len(loaded.CompletedSteps), len(checkpoint.CompletedSteps))
	}

	// Test ClearCheckpoint
	err = ClearCheckpoint(tmpDir)
	if err != nil {
		t.Fatalf("ClearCheckpoint failed: %v", err)
	}

	// Verify file was removed
	if HasCheckpoint(tmpDir) {
		t.Error("HasCheckpoint returned true after clear, want false")
	}
}

func TestCheckpointAge(t *testing.T) {
	tmpDir := t.TempDir()

	// No checkpoint - should return 0
	age := GetCheckpointAge(tmpDir)
	if age != 0 {
		t.Errorf("GetCheckpointAge with no checkpoint = %v, want 0", age)
	}

	// Create checkpoint with old timestamp
	// Note: SaveCheckpoint will update UpdatedAt, so we need to modify the file after
	checkpoint := &Checkpoint{
		PRNumber:  1,
		UpdatedAt: time.Now(),
	}
	if err := SaveCheckpoint(tmpDir, checkpoint); err != nil {
		t.Fatalf("SaveCheckpoint failed: %v", err)
	}

	// Load, modify timestamp, and save directly to bypass UpdatedAt refresh
	checkpointFile := filepath.Join(tmpDir, ".rig", "checkpoint.json")
	checkpoint.UpdatedAt = time.Now().Add(-time.Hour)
	data, _ := json.MarshalIndent(checkpoint, "", "  ")
	if err := os.WriteFile(checkpointFile, data, 0600); err != nil {
		t.Fatalf("Failed to write modified checkpoint: %v", err)
	}

	// Check age is approximately 1 hour
	age = GetCheckpointAge(tmpDir)
	if age < 55*time.Minute || age > 65*time.Minute {
		t.Errorf("GetCheckpointAge = %v, want approximately 1 hour", age)
	}

	// Check staleness
	if !IsCheckpointStale(tmpDir, 30*time.Minute) {
		t.Error("IsCheckpointStale(30min) = false, want true")
	}
	if IsCheckpointStale(tmpDir, 2*time.Hour) {
		t.Error("IsCheckpointStale(2h) = true, want false")
	}
}

func TestAllSteps(t *testing.T) {
	steps := AllSteps()
	if len(steps) != 5 {
		t.Errorf("AllSteps() returned %d steps, want 5", len(steps))
	}

	expectedOrder := []Step{StepPreflight, StepGather, StepDebrief, StepMerge, StepCloseout}
	for i, expected := range expectedOrder {
		if steps[i] != expected {
			t.Errorf("AllSteps()[%d] = %q, want %q", i, steps[i], expected)
		}
	}
}

func TestNewEngine(t *testing.T) {
	gh := &mockGitHubClient{}
	jiraClient := &mockJiraClient{}
	cfg := &config.Config{}

	engine := NewEngine(gh, jiraClient, cfg, false)
	if engine == nil {
		t.Fatal("NewEngine returned nil")
	}
	if engine.verbose {
		t.Error("Engine verbose should be false")
	}

	engineVerbose := NewEngine(gh, jiraClient, cfg, true)
	if !engineVerbose.verbose {
		t.Error("Engine verbose should be true")
	}
}

func TestPreflight(t *testing.T) {
	tests := []struct {
		name        string
		pr          *github.PRInfo
		jiraTicket  *jira.TicketInfo
		opts        MergeOptions
		wantReady   bool
		wantFailure string
	}{
		{
			name: "all checks pass",
			pr: &github.PRInfo{
				Number:        1,
				State:         "open",
				Approved:      true,
				ChecksPassing: true,
				HeadBranch:    "TEST-123",
			},
			jiraTicket: &jira.TicketInfo{
				Status: "In Review",
			},
			opts:      MergeOptions{},
			wantReady: true,
		},
		{
			name: "skip jira",
			pr: &github.PRInfo{
				Number:        1,
				State:         "open",
				Approved:      true,
				ChecksPassing: true,
				HeadBranch:    "feature-branch",
			},
			opts:      MergeOptions{SkipJira: true},
			wantReady: true,
		},
		{
			name: "pr not approved",
			pr: &github.PRInfo{
				Number:        1,
				State:         "open",
				Approved:      false,
				ChecksPassing: true,
				HeadBranch:    "TEST-123",
			},
			opts:        MergeOptions{SkipJira: true},
			wantReady:   false,
			wantFailure: "PR is not approved (use --skip-approval for self-authored PRs)",
		},
		{
			name: "pr not approved but skip-approval set",
			pr: &github.PRInfo{
				Number:        1,
				State:         "open",
				Approved:      false,
				ChecksPassing: true,
				HeadBranch:    "TEST-123",
			},
			opts:      MergeOptions{SkipJira: true, SkipApproval: true},
			wantReady: true,
		},
		{
			name: "checks failing",
			pr: &github.PRInfo{
				Number:        1,
				State:         "open",
				Approved:      true,
				ChecksPassing: false,
				HeadBranch:    "TEST-123",
			},
			opts:        MergeOptions{SkipJira: true},
			wantReady:   false,
			wantFailure: "CI checks are not passing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gh := &mockGitHubClient{pr: tt.pr}
			jiraClient := &mockJiraClient{ticketInfo: tt.jiraTicket}
			cfg := &config.Config{}

			engine := NewEngine(gh, jiraClient, cfg, false)
			result, err := engine.Preflight(t.Context(), 1, tt.opts)
			if err != nil {
				t.Fatalf("Preflight failed: %v", err)
			}

			if result.IsReady() != tt.wantReady {
				t.Errorf("IsReady() = %v, want %v", result.IsReady(), tt.wantReady)
			}

			if tt.wantFailure != "" && result.FailureReason != tt.wantFailure {
				t.Errorf("FailureReason = %q, want %q", result.FailureReason, tt.wantFailure)
			}
		})
	}
}
