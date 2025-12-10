package git

import (
	"errors"
	"strings"
	"testing"
)

// MockCommandRunner is a mock implementation of CommandRunner for testing
type MockCommandRunner struct {
	// RunFunc is called for Run() - returns error
	RunFunc func(dir string, name string, args ...string) error
	// OutputFunc is called for Output() - returns output and error
	OutputFunc func(dir string, name string, args ...string) ([]byte, error)
	// Calls records all calls made
	Calls []MockCall
}

// MockCall represents a single call to the mock
type MockCall struct {
	Method string
	Dir    string
	Name   string
	Args   []string
}

func (m *MockCommandRunner) Run(dir string, name string, args ...string) error {
	m.Calls = append(m.Calls, MockCall{Method: "Run", Dir: dir, Name: name, Args: args})
	if m.RunFunc != nil {
		return m.RunFunc(dir, name, args...)
	}
	return nil
}

func (m *MockCommandRunner) Output(dir string, name string, args ...string) ([]byte, error) {
	m.Calls = append(m.Calls, MockCall{Method: "Output", Dir: dir, Name: name, Args: args})
	if m.OutputFunc != nil {
		return m.OutputFunc(dir, name, args...)
	}
	return []byte{}, nil
}

func TestFetchAndPull_Success(t *testing.T) {
	mock := &MockCommandRunner{}
	wm := NewWorktreeManagerWithRunner("/repo", "main", false, mock)

	err := wm.fetchAndPull("main")
	if err != nil {
		t.Fatalf("fetchAndPull() error = %v, want nil", err)
	}

	// Verify correct commands were called
	if len(mock.Calls) != 2 {
		t.Fatalf("Expected 2 calls, got %d", len(mock.Calls))
	}

	// First call: git fetch origin
	if mock.Calls[0].Name != "git" {
		t.Errorf("Call 0: Name = %q, want %q", mock.Calls[0].Name, "git")
	}
	if len(mock.Calls[0].Args) < 2 || mock.Calls[0].Args[0] != "fetch" || mock.Calls[0].Args[1] != "origin" {
		t.Errorf("Call 0: Args = %v, want [fetch origin]", mock.Calls[0].Args)
	}

	// Second call: git pull origin main
	if mock.Calls[1].Name != "git" {
		t.Errorf("Call 1: Name = %q, want %q", mock.Calls[1].Name, "git")
	}
	if len(mock.Calls[1].Args) < 3 || mock.Calls[1].Args[0] != "pull" || mock.Calls[1].Args[1] != "origin" || mock.Calls[1].Args[2] != "main" {
		t.Errorf("Call 1: Args = %v, want [pull origin main]", mock.Calls[1].Args)
	}
}

func TestFetchAndPull_FetchError(t *testing.T) {
	mock := &MockCommandRunner{
		RunFunc: func(dir string, name string, args ...string) error {
			if len(args) > 0 && args[0] == "fetch" {
				return errors.New("network error")
			}
			return nil
		},
	}
	wm := NewWorktreeManagerWithRunner("/repo", "main", false, mock)

	err := wm.fetchAndPull("main")
	if err == nil {
		t.Fatal("fetchAndPull() expected error, got nil")
	}

	if !strings.Contains(err.Error(), "git fetch failed") {
		t.Errorf("Error = %q, want to contain 'git fetch failed'", err.Error())
	}

	// Should have only called fetch (failed before pull)
	if len(mock.Calls) != 1 {
		t.Errorf("Expected 1 call (fetch only), got %d", len(mock.Calls))
	}
}

func TestFetchAndPull_PullError(t *testing.T) {
	mock := &MockCommandRunner{
		RunFunc: func(dir string, name string, args ...string) error {
			if len(args) > 0 && args[0] == "pull" {
				return errors.New("merge conflict")
			}
			return nil
		},
	}
	wm := NewWorktreeManagerWithRunner("/repo", "main", false, mock)

	err := wm.fetchAndPull("main")
	if err == nil {
		t.Fatal("fetchAndPull() expected error, got nil")
	}

	if !strings.Contains(err.Error(), "git pull failed") {
		t.Errorf("Error = %q, want to contain 'git pull failed'", err.Error())
	}

	// Should have called both fetch and pull
	if len(mock.Calls) != 2 {
		t.Errorf("Expected 2 calls, got %d", len(mock.Calls))
	}
}

func TestFetchAndPull_DifferentBranch(t *testing.T) {
	mock := &MockCommandRunner{}
	wm := NewWorktreeManagerWithRunner("/repo", "develop", false, mock)

	err := wm.fetchAndPull("develop")
	if err != nil {
		t.Fatalf("fetchAndPull() error = %v, want nil", err)
	}

	// Verify pull uses correct branch
	if len(mock.Calls) < 2 {
		t.Fatalf("Expected at least 2 calls, got %d", len(mock.Calls))
	}

	if mock.Calls[1].Args[2] != "develop" {
		t.Errorf("Pull branch = %q, want %q", mock.Calls[1].Args[2], "develop")
	}
}

func TestBranchExists_True(t *testing.T) {
	mock := &MockCommandRunner{
		RunFunc: func(dir string, name string, args ...string) error {
			// git show-ref returns 0 when branch exists
			return nil
		},
	}
	wm := NewWorktreeManagerWithRunner("/repo", "main", false, mock)

	result := wm.branchExists("main")
	if !result {
		t.Error("branchExists() = false, want true")
	}

	// Verify correct command
	if len(mock.Calls) != 1 {
		t.Fatalf("Expected 1 call, got %d", len(mock.Calls))
	}

	call := mock.Calls[0]
	if call.Name != "git" {
		t.Errorf("Name = %q, want %q", call.Name, "git")
	}
	if len(call.Args) < 4 || call.Args[0] != "show-ref" || call.Args[1] != "--verify" || call.Args[2] != "--quiet" {
		t.Errorf("Args = %v, want [show-ref --verify --quiet refs/heads/main]", call.Args)
	}
	if !strings.Contains(call.Args[3], "refs/heads/main") {
		t.Errorf("Branch ref = %q, want to contain 'refs/heads/main'", call.Args[3])
	}
}

func TestBranchExists_False(t *testing.T) {
	mock := &MockCommandRunner{
		RunFunc: func(dir string, name string, args ...string) error {
			// git show-ref returns non-zero when branch doesn't exist
			return errors.New("exit status 1")
		},
	}
	wm := NewWorktreeManagerWithRunner("/repo", "main", false, mock)

	result := wm.branchExists("nonexistent")
	if result {
		t.Error("branchExists() = true, want false")
	}
}

func TestBranchExists_DifferentBranches(t *testing.T) {
	tests := []struct {
		name   string
		branch string
	}{
		{"main branch", "main"},
		{"master branch", "master"},
		{"feature branch", "feature/test"},
		{"release branch", "release-1.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &MockCommandRunner{}
			wm := NewWorktreeManagerWithRunner("/repo", "main", false, mock)

			wm.branchExists(tt.branch)

			if len(mock.Calls) != 1 {
				t.Fatalf("Expected 1 call, got %d", len(mock.Calls))
			}

			expectedRef := "refs/heads/" + tt.branch
			if !strings.Contains(mock.Calls[0].Args[3], expectedRef) {
				t.Errorf("Branch ref = %q, want to contain %q", mock.Calls[0].Args[3], expectedRef)
			}
		})
	}
}

func TestGetFirstBranch_Success(t *testing.T) {
	mock := &MockCommandRunner{
		OutputFunc: func(dir string, name string, args ...string) ([]byte, error) {
			return []byte("  origin/HEAD -> origin/main\n  origin/main\n  origin/develop\n"), nil
		},
	}
	wm := NewWorktreeManagerWithRunner("/repo", "main", false, mock)

	branch, err := wm.getFirstBranch()
	if err != nil {
		t.Fatalf("getFirstBranch() error = %v, want nil", err)
	}

	if branch != "main" {
		t.Errorf("getFirstBranch() = %q, want %q", branch, "main")
	}
}

func TestGetFirstBranch_SkipsHEAD(t *testing.T) {
	mock := &MockCommandRunner{
		OutputFunc: func(dir string, name string, args ...string) ([]byte, error) {
			// HEAD -> line should be skipped
			return []byte("  origin/HEAD -> origin/main\n  origin/develop\n"), nil
		},
	}
	wm := NewWorktreeManagerWithRunner("/repo", "main", false, mock)

	branch, err := wm.getFirstBranch()
	if err != nil {
		t.Fatalf("getFirstBranch() error = %v, want nil", err)
	}

	if branch != "develop" {
		t.Errorf("getFirstBranch() = %q, want %q", branch, "develop")
	}
}

func TestGetFirstBranch_NoBranches(t *testing.T) {
	mock := &MockCommandRunner{
		OutputFunc: func(dir string, name string, args ...string) ([]byte, error) {
			return []byte(""), nil
		},
	}
	wm := NewWorktreeManagerWithRunner("/repo", "main", false, mock)

	_, err := wm.getFirstBranch()
	if err == nil {
		t.Fatal("getFirstBranch() expected error for empty branches, got nil")
	}

	if !strings.Contains(err.Error(), "no branches found") {
		t.Errorf("Error = %q, want to contain 'no branches found'", err.Error())
	}
}

func TestGetFirstBranch_GitError(t *testing.T) {
	mock := &MockCommandRunner{
		OutputFunc: func(dir string, name string, args ...string) ([]byte, error) {
			return nil, errors.New("fatal: not a git repository")
		},
	}
	wm := NewWorktreeManagerWithRunner("/repo", "main", false, mock)

	_, err := wm.getFirstBranch()
	if err == nil {
		t.Fatal("getFirstBranch() expected error, got nil")
	}
}

func TestGetFirstBranch_OnlyHEAD(t *testing.T) {
	mock := &MockCommandRunner{
		OutputFunc: func(dir string, name string, args ...string) ([]byte, error) {
			// Only HEAD line, no actual branches
			return []byte("  origin/HEAD -> origin/main\n"), nil
		},
	}
	wm := NewWorktreeManagerWithRunner("/repo", "main", false, mock)

	_, err := wm.getFirstBranch()
	if err == nil {
		t.Fatal("getFirstBranch() expected error when only HEAD exists, got nil")
	}
}

func TestCreateInitialBranch_Success(t *testing.T) {
	mock := &MockCommandRunner{}
	wm := NewWorktreeManagerWithRunner("/repo", "main", false, mock)

	branch, err := wm.createInitialBranch()
	if err != nil {
		t.Fatalf("createInitialBranch() error = %v, want nil", err)
	}

	if branch != "main" {
		t.Errorf("createInitialBranch() = %q, want %q", branch, "main")
	}

	// Should call: git switch -c main, then git commit --allow-empty
	if len(mock.Calls) != 2 {
		t.Fatalf("Expected 2 calls, got %d", len(mock.Calls))
	}

	// First call: git switch -c main
	if mock.Calls[0].Args[0] != "switch" || mock.Calls[0].Args[1] != "-c" || mock.Calls[0].Args[2] != "main" {
		t.Errorf("Call 0 Args = %v, want [switch -c main]", mock.Calls[0].Args)
	}

	// Second call: git commit --allow-empty -m "Initial commit"
	if mock.Calls[1].Args[0] != "commit" {
		t.Errorf("Call 1 Args[0] = %q, want %q", mock.Calls[1].Args[0], "commit")
	}
	if mock.Calls[1].Args[1] != "--allow-empty" {
		t.Errorf("Call 1 Args[1] = %q, want %q", mock.Calls[1].Args[1], "--allow-empty")
	}
}

func TestCreateInitialBranch_SwitchError(t *testing.T) {
	mock := &MockCommandRunner{
		RunFunc: func(dir string, name string, args ...string) error {
			if len(args) > 0 && args[0] == "switch" {
				return errors.New("branch already exists")
			}
			return nil
		},
	}
	wm := NewWorktreeManagerWithRunner("/repo", "main", false, mock)

	_, err := wm.createInitialBranch()
	if err == nil {
		t.Fatal("createInitialBranch() expected error, got nil")
	}

	if !strings.Contains(err.Error(), "failed to create main branch") {
		t.Errorf("Error = %q, want to contain 'failed to create main branch'", err.Error())
	}
}

func TestCreateInitialBranch_CommitError(t *testing.T) {
	mock := &MockCommandRunner{
		RunFunc: func(dir string, name string, args ...string) error {
			if len(args) > 0 && args[0] == "commit" {
				return errors.New("commit failed")
			}
			return nil
		},
	}
	wm := NewWorktreeManagerWithRunner("/repo", "main", false, mock)

	_, err := wm.createInitialBranch()
	if err == nil {
		t.Fatal("createInitialBranch() expected error, got nil")
	}

	if !strings.Contains(err.Error(), "failed to create initial commit") {
		t.Errorf("Error = %q, want to contain 'failed to create initial commit'", err.Error())
	}
}

func TestGetBaseBranch_PreferredBranchExists(t *testing.T) {
	mock := &MockCommandRunner{
		RunFunc: func(dir string, name string, args ...string) error {
			// Only "develop" exists
			if len(args) > 3 && strings.Contains(args[3], "refs/heads/develop") {
				return nil
			}
			return errors.New("not found")
		},
	}
	wm := NewWorktreeManagerWithRunner("/repo", "develop", false, mock)

	branch, err := wm.getBaseBranch()
	if err != nil {
		t.Fatalf("getBaseBranch() error = %v, want nil", err)
	}

	if branch != "develop" {
		t.Errorf("getBaseBranch() = %q, want %q", branch, "develop")
	}
}

func TestGetBaseBranch_FallbackToMaster(t *testing.T) {
	mock := &MockCommandRunner{
		RunFunc: func(dir string, name string, args ...string) error {
			// Only "master" exists
			if len(args) > 3 && strings.Contains(args[3], "refs/heads/master") {
				return nil
			}
			return errors.New("not found")
		},
	}
	wm := NewWorktreeManagerWithRunner("/repo", "nonexistent", false, mock)

	branch, err := wm.getBaseBranch()
	if err != nil {
		t.Fatalf("getBaseBranch() error = %v, want nil", err)
	}

	if branch != "master" {
		t.Errorf("getBaseBranch() = %q, want %q", branch, "master")
	}
}

func TestGetBaseBranch_FallbackToMain(t *testing.T) {
	mock := &MockCommandRunner{
		RunFunc: func(dir string, name string, args ...string) error {
			// Only "main" exists
			if len(args) > 3 && strings.Contains(args[3], "refs/heads/main") {
				return nil
			}
			return errors.New("not found")
		},
	}
	wm := NewWorktreeManagerWithRunner("/repo", "nonexistent", false, mock)

	branch, err := wm.getBaseBranch()
	if err != nil {
		t.Fatalf("getBaseBranch() error = %v, want nil", err)
	}

	if branch != "main" {
		t.Errorf("getBaseBranch() = %q, want %q", branch, "main")
	}
}

func TestListWorktrees_Success(t *testing.T) {
	mock := &MockCommandRunner{
		OutputFunc: func(dir string, name string, args ...string) ([]byte, error) {
			return []byte("worktree /home/user/repo\nHEAD abc123\nbranch refs/heads/main\n\nworktree /home/user/repo/fraas/FRAAS-123\nHEAD def456\nbranch refs/heads/FRAAS-123\n"), nil
		},
	}
	wm := NewWorktreeManagerWithRunner("/repo", "main", false, mock)

	worktrees, err := wm.ListWorktrees()
	if err != nil {
		t.Fatalf("ListWorktrees() error = %v, want nil", err)
	}

	if len(worktrees) != 2 {
		t.Fatalf("ListWorktrees() returned %d worktrees, want 2", len(worktrees))
	}

	if worktrees[0] != "/home/user/repo" {
		t.Errorf("worktrees[0] = %q, want %q", worktrees[0], "/home/user/repo")
	}
	if worktrees[1] != "/home/user/repo/fraas/FRAAS-123" {
		t.Errorf("worktrees[1] = %q, want %q", worktrees[1], "/home/user/repo/fraas/FRAAS-123")
	}
}

func TestListWorktrees_Empty(t *testing.T) {
	mock := &MockCommandRunner{
		OutputFunc: func(dir string, name string, args ...string) ([]byte, error) {
			return []byte(""), nil
		},
	}
	wm := NewWorktreeManagerWithRunner("/repo", "main", false, mock)

	worktrees, err := wm.ListWorktrees()
	if err != nil {
		t.Fatalf("ListWorktrees() error = %v, want nil", err)
	}

	if len(worktrees) != 0 {
		t.Errorf("ListWorktrees() returned %d worktrees, want 0", len(worktrees))
	}
}

func TestListWorktrees_Error(t *testing.T) {
	mock := &MockCommandRunner{
		OutputFunc: func(dir string, name string, args ...string) ([]byte, error) {
			return nil, errors.New("not a git repository")
		},
	}
	wm := NewWorktreeManagerWithRunner("/repo", "main", false, mock)

	_, err := wm.ListWorktrees()
	if err == nil {
		t.Fatal("ListWorktrees() expected error, got nil")
	}

	if !strings.Contains(err.Error(), "failed to list worktrees") {
		t.Errorf("Error = %q, want to contain 'failed to list worktrees'", err.Error())
	}
}

func TestCreateWorktreeFromBranchWithName_Success(t *testing.T) {
	mock := &MockCommandRunner{}
	wm := NewWorktreeManagerWithRunner("/repo", "main", false, mock)

	err := wm.createWorktreeFromBranchWithName("fraas", "FRAAS-123", "feature-branch", "main")
	if err != nil {
		t.Fatalf("createWorktreeFromBranchWithName() error = %v, want nil", err)
	}

	if len(mock.Calls) != 1 {
		t.Fatalf("Expected 1 call, got %d", len(mock.Calls))
	}

	// Verify: git worktree add fraas/FRAAS-123 -b feature-branch main
	call := mock.Calls[0]
	if call.Name != "git" {
		t.Errorf("Name = %q, want %q", call.Name, "git")
	}

	expectedArgs := []string{"worktree", "add", "fraas/FRAAS-123", "-b", "feature-branch", "main"}
	if len(call.Args) != len(expectedArgs) {
		t.Fatalf("Args length = %d, want %d", len(call.Args), len(expectedArgs))
	}
	for i, arg := range expectedArgs {
		if call.Args[i] != arg {
			t.Errorf("Args[%d] = %q, want %q", i, call.Args[i], arg)
		}
	}
}

func TestNewWorktreeManager(t *testing.T) {
	wm := NewWorktreeManager("/repo", "main", true)

	if wm.RepoPath != "/repo" {
		t.Errorf("RepoPath = %q, want %q", wm.RepoPath, "/repo")
	}
	if wm.BaseBranch != "main" {
		t.Errorf("BaseBranch = %q, want %q", wm.BaseBranch, "main")
	}
	if !wm.Verbose {
		t.Error("Verbose = false, want true")
	}
	if wm.runner == nil {
		t.Error("runner should not be nil")
	}
}

func TestNewWorktreeManagerWithRunner(t *testing.T) {
	mock := &MockCommandRunner{}
	wm := NewWorktreeManagerWithRunner("/repo", "develop", false, mock)

	if wm.RepoPath != "/repo" {
		t.Errorf("RepoPath = %q, want %q", wm.RepoPath, "/repo")
	}
	if wm.BaseBranch != "develop" {
		t.Errorf("BaseBranch = %q, want %q", wm.BaseBranch, "develop")
	}
	if wm.Verbose {
		t.Error("Verbose = true, want false")
	}
	if wm.runner != mock {
		t.Error("runner should be the provided mock")
	}
}

func TestRealCommandRunner_Interface(t *testing.T) {
	// Verify RealCommandRunner implements CommandRunner
	var _ CommandRunner = &RealCommandRunner{}
}
