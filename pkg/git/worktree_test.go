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

func TestGetRepoRoot_Success(t *testing.T) {
	mock := &MockCommandRunner{
		OutputFunc: func(dir string, name string, args ...string) ([]byte, error) {
			if len(args) > 1 && args[0] == "rev-parse" && args[1] == "--git-common-dir" {
				// Absolute path returned from a worktree
				return []byte("/home/user/src/myorg/myrepo\n"), nil
			}
			return []byte{}, nil
		},
	}
	wm := NewWorktreeManagerWithRunner("", false, mock)

	root, err := wm.GetRepoRoot()
	if err != nil {
		t.Fatalf("GetRepoRoot() error = %v, want nil", err)
	}

	if root != "/home/user/src/myorg/myrepo" {
		t.Errorf("GetRepoRoot() = %q, want %q", root, "/home/user/src/myorg/myrepo")
	}
}

func TestGetRepoRoot_NotGitRepo(t *testing.T) {
	mock := &MockCommandRunner{
		OutputFunc: func(dir string, name string, args ...string) ([]byte, error) {
			if len(args) > 1 && args[0] == "rev-parse" && args[1] == "--git-common-dir" {
				return nil, errors.New("fatal: not a git repository")
			}
			return []byte{}, nil
		},
	}
	wm := NewWorktreeManagerWithRunner("", false, mock)

	_, err := wm.GetRepoRoot()
	if err == nil {
		t.Fatal("GetRepoRoot() expected error, got nil")
	}

	if !strings.Contains(err.Error(), "not inside a git repository") {
		t.Errorf("Error = %q, want to contain 'not inside a git repository'", err.Error())
	}
}

func TestGetRepoRoot_BareRepoRelativePath(t *testing.T) {
	// In a bare repo, git rev-parse --git-common-dir returns "."
	mock := &MockCommandRunner{
		OutputFunc: func(dir string, name string, args ...string) ([]byte, error) {
			if len(args) > 1 && args[0] == "rev-parse" && args[1] == "--git-common-dir" {
				return []byte(".\n"), nil
			}
			return []byte{}, nil
		},
	}
	wm := NewWorktreeManagerWithRunner("", false, mock)
	// Override getwd to return a known path
	wm.getwd = func() (string, error) {
		return "/home/user/src/myorg/myrepo", nil
	}

	root, err := wm.GetRepoRoot()
	if err != nil {
		t.Fatalf("GetRepoRoot() error = %v, want nil", err)
	}

	if root != "/home/user/src/myorg/myrepo" {
		t.Errorf("GetRepoRoot() = %q, want %q", root, "/home/user/src/myorg/myrepo")
	}
}

func TestGetRepoRoot_GetwdError(t *testing.T) {
	// Test error handling when getwd fails
	mock := &MockCommandRunner{
		OutputFunc: func(dir string, name string, args ...string) ([]byte, error) {
			if len(args) > 1 && args[0] == "rev-parse" && args[1] == "--git-common-dir" {
				return []byte(".\n"), nil
			}
			return []byte{}, nil
		},
	}
	wm := NewWorktreeManagerWithRunner("", false, mock)
	wm.getwd = func() (string, error) {
		return "", errors.New("permission denied")
	}

	_, err := wm.GetRepoRoot()
	if err == nil {
		t.Fatal("GetRepoRoot() expected error, got nil")
	}

	if !strings.Contains(err.Error(), "failed to get working directory") {
		t.Errorf("Error = %q, want to contain 'failed to get working directory'", err.Error())
	}
}

func TestGetRepoRoot_PathCleaning(t *testing.T) {
	// Test that paths with .. components are cleaned properly
	mock := &MockCommandRunner{
		OutputFunc: func(dir string, name string, args ...string) ([]byte, error) {
			if len(args) > 1 && args[0] == "rev-parse" && args[1] == "--git-common-dir" {
				// Simulate a relative path with parent directory reference
				return []byte("../myrepo\n"), nil
			}
			return []byte{}, nil
		},
	}
	wm := NewWorktreeManagerWithRunner("", false, mock)
	wm.getwd = func() (string, error) {
		return "/home/user/src/myorg/worktree", nil
	}

	root, err := wm.GetRepoRoot()
	if err != nil {
		t.Fatalf("GetRepoRoot() error = %v, want nil", err)
	}

	// /home/user/src/myorg/worktree + ../myrepo -> /home/user/src/myorg/myrepo
	if root != "/home/user/src/myorg/myrepo" {
		t.Errorf("GetRepoRoot() = %q, want %q", root, "/home/user/src/myorg/myrepo")
	}
}

func TestGetRepoName_Success(t *testing.T) {
	mock := &MockCommandRunner{
		OutputFunc: func(dir string, name string, args ...string) ([]byte, error) {
			if len(args) > 1 && args[0] == "rev-parse" && args[1] == "--git-common-dir" {
				return []byte("/home/user/src/myorg/myrepo\n"), nil
			}
			return []byte{}, nil
		},
	}
	wm := NewWorktreeManagerWithRunner("", false, mock)

	name, err := wm.GetRepoName()
	if err != nil {
		t.Fatalf("GetRepoName() error = %v, want nil", err)
	}

	if name != "myrepo" {
		t.Errorf("GetRepoName() = %q, want %q", name, "myrepo")
	}
}

func TestGetDefaultBranch_ConfigOverride(t *testing.T) {
	mock := &MockCommandRunner{
		OutputFunc: func(dir string, name string, args ...string) ([]byte, error) {
			if len(args) > 1 && args[0] == "rev-parse" && args[1] == "--git-common-dir" {
				return []byte("/repo\n"), nil
			}
			return []byte{}, nil
		},
		RunFunc: func(dir string, name string, args ...string) error {
			// Branch "develop" exists
			if len(args) > 3 && args[0] == "show-ref" && strings.Contains(args[3], "refs/heads/develop") {
				return nil
			}
			return errors.New("not found")
		},
	}
	wm := NewWorktreeManagerWithRunner("develop", false, mock)

	branch, err := wm.GetDefaultBranch()
	if err != nil {
		t.Fatalf("GetDefaultBranch() error = %v, want nil", err)
	}

	if branch != "develop" {
		t.Errorf("GetDefaultBranch() = %q, want %q", branch, "develop")
	}
}

func TestGetDefaultBranch_RemoteHEAD(t *testing.T) {
	mock := &MockCommandRunner{
		OutputFunc: func(dir string, name string, args ...string) ([]byte, error) {
			if len(args) > 1 && args[0] == "rev-parse" && args[1] == "--git-common-dir" {
				return []byte("/repo\n"), nil
			}
			if len(args) > 0 && args[0] == "symbolic-ref" {
				return []byte("refs/remotes/origin/main\n"), nil
			}
			return []byte{}, nil
		},
		RunFunc: func(dir string, name string, args ...string) error {
			// Branch "main" exists
			if len(args) > 3 && args[0] == "show-ref" && strings.Contains(args[3], "refs/heads/main") {
				return nil
			}
			return errors.New("not found")
		},
	}
	wm := NewWorktreeManagerWithRunner("", false, mock)

	branch, err := wm.GetDefaultBranch()
	if err != nil {
		t.Fatalf("GetDefaultBranch() error = %v, want nil", err)
	}

	if branch != "main" {
		t.Errorf("GetDefaultBranch() = %q, want %q", branch, "main")
	}
}

func TestGetDefaultBranch_FallbackToMain(t *testing.T) {
	mock := &MockCommandRunner{
		OutputFunc: func(dir string, name string, args ...string) ([]byte, error) {
			if len(args) > 1 && args[0] == "rev-parse" && args[1] == "--git-common-dir" {
				return []byte("/repo\n"), nil
			}
			if len(args) > 0 && args[0] == "symbolic-ref" {
				return nil, errors.New("not found")
			}
			return []byte{}, nil
		},
		RunFunc: func(dir string, name string, args ...string) error {
			// Only "main" exists
			if len(args) > 3 && args[0] == "show-ref" && strings.Contains(args[3], "refs/heads/main") {
				return nil
			}
			return errors.New("not found")
		},
	}
	wm := NewWorktreeManagerWithRunner("", false, mock)

	branch, err := wm.GetDefaultBranch()
	if err != nil {
		t.Fatalf("GetDefaultBranch() error = %v, want nil", err)
	}

	if branch != "main" {
		t.Errorf("GetDefaultBranch() = %q, want %q", branch, "main")
	}
}

func TestGetDefaultBranch_FallbackToMaster(t *testing.T) {
	mock := &MockCommandRunner{
		OutputFunc: func(dir string, name string, args ...string) ([]byte, error) {
			if len(args) > 1 && args[0] == "rev-parse" && args[1] == "--git-common-dir" {
				return []byte("/repo\n"), nil
			}
			if len(args) > 0 && args[0] == "symbolic-ref" {
				return nil, errors.New("not found")
			}
			return []byte{}, nil
		},
		RunFunc: func(dir string, name string, args ...string) error {
			// Only "master" exists
			if len(args) > 3 && args[0] == "show-ref" && strings.Contains(args[3], "refs/heads/master") {
				return nil
			}
			return errors.New("not found")
		},
	}
	wm := NewWorktreeManagerWithRunner("", false, mock)

	branch, err := wm.GetDefaultBranch()
	if err != nil {
		t.Fatalf("GetDefaultBranch() error = %v, want nil", err)
	}

	if branch != "master" {
		t.Errorf("GetDefaultBranch() = %q, want %q", branch, "master")
	}
}

func TestFetchAndPull_Success(t *testing.T) {
	mock := &MockCommandRunner{}
	wm := NewWorktreeManagerWithRunner("main", false, mock)

	err := wm.fetchAndPull("/repo", "main")
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
	wm := NewWorktreeManagerWithRunner("main", false, mock)

	err := wm.fetchAndPull("/repo", "main")
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
	wm := NewWorktreeManagerWithRunner("main", false, mock)

	err := wm.fetchAndPull("/repo", "main")
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
	wm := NewWorktreeManagerWithRunner("develop", false, mock)

	err := wm.fetchAndPull("/repo", "develop")
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
	wm := NewWorktreeManagerWithRunner("main", false, mock)

	result := wm.branchExists("/repo", "main")
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
	wm := NewWorktreeManagerWithRunner("main", false, mock)

	result := wm.branchExists("/repo", "nonexistent")
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
			wm := NewWorktreeManagerWithRunner("main", false, mock)

			wm.branchExists("/repo", tt.branch)

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

func TestGetFirstRemoteBranch_Success(t *testing.T) {
	mock := &MockCommandRunner{
		OutputFunc: func(dir string, name string, args ...string) ([]byte, error) {
			return []byte("  origin/HEAD -> origin/main\n  origin/main\n  origin/develop\n"), nil
		},
	}
	wm := NewWorktreeManagerWithRunner("main", false, mock)

	branch, err := wm.getFirstRemoteBranch("/repo")
	if err != nil {
		t.Fatalf("getFirstRemoteBranch() error = %v, want nil", err)
	}

	if branch != "main" {
		t.Errorf("getFirstRemoteBranch() = %q, want %q", branch, "main")
	}
}

func TestGetFirstRemoteBranch_SkipsHEAD(t *testing.T) {
	mock := &MockCommandRunner{
		OutputFunc: func(dir string, name string, args ...string) ([]byte, error) {
			// HEAD -> line should be skipped
			return []byte("  origin/HEAD -> origin/main\n  origin/develop\n"), nil
		},
	}
	wm := NewWorktreeManagerWithRunner("main", false, mock)

	branch, err := wm.getFirstRemoteBranch("/repo")
	if err != nil {
		t.Fatalf("getFirstRemoteBranch() error = %v, want nil", err)
	}

	if branch != "develop" {
		t.Errorf("getFirstRemoteBranch() = %q, want %q", branch, "develop")
	}
}

func TestGetFirstRemoteBranch_NoBranches(t *testing.T) {
	mock := &MockCommandRunner{
		OutputFunc: func(dir string, name string, args ...string) ([]byte, error) {
			return []byte(""), nil
		},
	}
	wm := NewWorktreeManagerWithRunner("main", false, mock)

	_, err := wm.getFirstRemoteBranch("/repo")
	if err == nil {
		t.Fatal("getFirstRemoteBranch() expected error for empty branches, got nil")
	}

	if !strings.Contains(err.Error(), "no branches found") {
		t.Errorf("Error = %q, want to contain 'no branches found'", err.Error())
	}
}

func TestGetFirstRemoteBranch_GitError(t *testing.T) {
	mock := &MockCommandRunner{
		OutputFunc: func(dir string, name string, args ...string) ([]byte, error) {
			return nil, errors.New("fatal: not a git repository")
		},
	}
	wm := NewWorktreeManagerWithRunner("main", false, mock)

	_, err := wm.getFirstRemoteBranch("/repo")
	if err == nil {
		t.Fatal("getFirstRemoteBranch() expected error, got nil")
	}
}

func TestGetFirstRemoteBranch_OnlyHEAD(t *testing.T) {
	mock := &MockCommandRunner{
		OutputFunc: func(dir string, name string, args ...string) ([]byte, error) {
			// Only HEAD line, no actual branches
			return []byte("  origin/HEAD -> origin/main\n"), nil
		},
	}
	wm := NewWorktreeManagerWithRunner("main", false, mock)

	_, err := wm.getFirstRemoteBranch("/repo")
	if err == nil {
		t.Fatal("getFirstRemoteBranch() expected error when only HEAD exists, got nil")
	}
}

func TestCreateInitialBranch_Success(t *testing.T) {
	mock := &MockCommandRunner{}
	wm := NewWorktreeManagerWithRunner("main", false, mock)

	branch, err := wm.createInitialBranch("/repo")
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
	wm := NewWorktreeManagerWithRunner("main", false, mock)

	_, err := wm.createInitialBranch("/repo")
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
	wm := NewWorktreeManagerWithRunner("main", false, mock)

	_, err := wm.createInitialBranch("/repo")
	if err == nil {
		t.Fatal("createInitialBranch() expected error, got nil")
	}

	if !strings.Contains(err.Error(), "failed to create initial commit") {
		t.Errorf("Error = %q, want to contain 'failed to create initial commit'", err.Error())
	}
}

func TestListWorktrees_Success(t *testing.T) {
	mock := &MockCommandRunner{
		OutputFunc: func(dir string, name string, args ...string) ([]byte, error) {
			if len(args) > 1 && args[0] == "rev-parse" && args[1] == "--git-common-dir" {
				return []byte("/home/user/repo\n"), nil
			}
			if len(args) > 0 && args[0] == "worktree" {
				return []byte("worktree /home/user/repo\nHEAD abc123\nbranch refs/heads/main\n\nworktree /home/user/repo/fraas/FRAAS-123\nHEAD def456\nbranch refs/heads/FRAAS-123\n"), nil
			}
			return []byte{}, nil
		},
	}
	wm := NewWorktreeManagerWithRunner("main", false, mock)

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
			if len(args) > 1 && args[0] == "rev-parse" && args[1] == "--git-common-dir" {
				return []byte("/home/user/repo\n"), nil
			}
			return []byte(""), nil
		},
	}
	wm := NewWorktreeManagerWithRunner("main", false, mock)

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
			if len(args) > 1 && args[0] == "rev-parse" && args[1] == "--git-common-dir" {
				return []byte("/home/user/repo\n"), nil
			}
			return nil, errors.New("not a git repository")
		},
	}
	wm := NewWorktreeManagerWithRunner("main", false, mock)

	_, err := wm.ListWorktrees()
	if err == nil {
		t.Fatal("ListWorktrees() expected error, got nil")
	}

	if !strings.Contains(err.Error(), "failed to list worktrees") {
		t.Errorf("Error = %q, want to contain 'failed to list worktrees'", err.Error())
	}
}

func TestNewWorktreeManager(t *testing.T) {
	wm := NewWorktreeManager("develop", true)

	if wm.BaseBranchConfig != "develop" {
		t.Errorf("BaseBranchConfig = %q, want %q", wm.BaseBranchConfig, "develop")
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
	wm := NewWorktreeManagerWithRunner("develop", false, mock)

	if wm.BaseBranchConfig != "develop" {
		t.Errorf("BaseBranchConfig = %q, want %q", wm.BaseBranchConfig, "develop")
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
