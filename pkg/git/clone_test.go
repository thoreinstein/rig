package git

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestCloneManager_Clone_SSH(t *testing.T) {
	t.Parallel()

	// Use temp directory for actual filesystem operations
	tmpDir := t.TempDir()

	worktreeCreated := false

	mock := &MockCommandRunner{
		RunFunc: func(dir string, name string, args ...string) error {
			if name == "git" {
				if len(args) > 0 && args[0] == "clone" {
					// Simulate successful bare clone by creating the directory
					if len(args) >= 4 {
						targetDir := args[3]
						if err := os.MkdirAll(targetDir, 0755); err != nil {
							return err
						}
					}
					return nil
				}
				if len(args) > 0 && args[0] == "config" {
					return nil
				}
				if len(args) > 0 && args[0] == "fetch" {
					return nil
				}
				if len(args) > 0 && args[0] == "worktree" && len(args) > 1 && args[1] == "add" {
					worktreeCreated = true
					return nil
				}
				if len(args) > 0 && args[0] == "show-ref" {
					// main branch exists
					if len(args) > 3 && strings.Contains(args[3], "origin/main") {
						return nil
					}
					return errors.New("not found")
				}
			}
			return nil
		},
		OutputFunc: func(dir string, name string, args ...string) ([]byte, error) {
			if name == "git" {
				if len(args) > 0 && args[0] == "config" {
					return []byte{}, errors.New("not found")
				}
				if len(args) > 0 && args[0] == "symbolic-ref" {
					return []byte("refs/remotes/origin/main\n"), nil
				}
				if len(args) > 0 && args[0] == "branch" {
					return []byte("  origin/main\n"), nil
				}
			}
			return []byte{}, nil
		},
	}

	cm := NewCloneManagerWithRunner(tmpDir, false, mock)

	url := &RepoURL{
		Original:  "git@github.com:owner/repo.git",
		Canonical: "git@github.com:owner/repo.git",
		Protocol:  "ssh",
		Owner:     "owner",
		Repo:      "repo",
	}

	path, err := cm.Clone(url)
	if err != nil {
		t.Fatalf("Clone() error = %v", err)
	}

	expectedPath := tmpDir + "/owner/repo"
	if path != expectedPath {
		t.Errorf("Clone() path = %q, want %q", path, expectedPath)
	}

	if !worktreeCreated {
		t.Error("Expected worktree to be created for SSH clone")
	}
}

func TestCloneManager_Clone_HTTPS(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cloneExecuted := false

	mock := &MockCommandRunner{
		RunFunc: func(dir string, name string, args ...string) error {
			if name == "git" && len(args) > 0 && args[0] == "clone" {
				// Verify it's NOT a bare clone
				for _, arg := range args {
					if arg == "--bare" {
						t.Error("HTTPS clone should not use --bare")
					}
				}
				// Simulate successful clone by creating the directory
				if len(args) >= 3 {
					targetDir := args[2]
					if err := os.MkdirAll(targetDir, 0755); err != nil {
						return err
					}
				}
				cloneExecuted = true
				return nil
			}
			return nil
		},
	}

	cm := NewCloneManagerWithRunner(tmpDir, false, mock)

	url := &RepoURL{
		Original:  "https://github.com/owner/repo",
		Canonical: "https://github.com/owner/repo.git",
		Protocol:  "https",
		Owner:     "owner",
		Repo:      "repo",
	}

	path, err := cm.Clone(url)
	if err != nil {
		t.Fatalf("Clone() error = %v", err)
	}

	expectedPath := tmpDir + "/owner/repo"
	if path != expectedPath {
		t.Errorf("Clone() path = %q, want %q", path, expectedPath)
	}

	if !cloneExecuted {
		t.Error("Expected git clone to be executed")
	}
}

func TestCloneManager_Clone_CustomBasePath(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	mock := &MockCommandRunner{
		RunFunc: func(dir string, name string, args ...string) error {
			// Simulate successful clone by creating the directory
			if name == "git" && len(args) > 0 && args[0] == "clone" {
				if len(args) >= 3 {
					targetDir := args[2]
					if err := os.MkdirAll(targetDir, 0755); err != nil {
						return err
					}
				}
			}
			return nil
		},
	}

	cm := NewCloneManagerWithRunner(tmpDir, false, mock)

	url := &RepoURL{
		Original:  "https://github.com/owner/repo",
		Canonical: "https://github.com/owner/repo.git",
		Protocol:  "https",
		Owner:     "owner",
		Repo:      "repo",
	}

	path, err := cm.Clone(url)
	if err != nil {
		t.Fatalf("Clone() error = %v", err)
	}

	expectedPath := tmpDir + "/owner/repo"
	if path != expectedPath {
		t.Errorf("Clone() path = %q, want %q", path, expectedPath)
	}
}

func TestCloneManager_Clone_NilURL(t *testing.T) {
	t.Parallel()

	mock := &MockCommandRunner{}
	cm := NewCloneManagerWithRunner("", false, mock)

	_, err := cm.Clone(nil)
	if err == nil {
		t.Error("Clone(nil) should return error")
	}
	if !strings.Contains(err.Error(), "nil URL") {
		t.Errorf("Error = %q, should contain 'nil URL'", err.Error())
	}
}

func TestCloneManager_Clone_CloneError(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	mock := &MockCommandRunner{
		RunFunc: func(dir string, name string, args ...string) error {
			if name == "git" && len(args) > 0 && args[0] == "clone" {
				return errors.New("authentication failed")
			}
			return nil
		},
	}

	cm := NewCloneManagerWithRunner(tmpDir, false, mock)

	url := &RepoURL{
		Original:  "git@github.com:owner/repo.git",
		Canonical: "git@github.com:owner/repo.git",
		Protocol:  "ssh",
		Owner:     "owner",
		Repo:      "repo",
	}

	_, err := cm.Clone(url)
	if err == nil {
		t.Error("Clone() should return error on clone failure")
	}
	if !strings.Contains(err.Error(), "git clone") {
		t.Errorf("Error = %q, should contain 'git clone'", err.Error())
	}
}

func TestCloneManager_ensureFetchRefspec_AlreadyConfigured(t *testing.T) {
	t.Parallel()

	mock := &MockCommandRunner{
		OutputFunc: func(dir string, name string, args ...string) ([]byte, error) {
			return []byte("+refs/heads/*:refs/remotes/origin/*\n"), nil
		},
	}

	cm := NewCloneManagerWithRunner("", false, mock)
	err := cm.ensureFetchRefspec("/repo")
	if err != nil {
		t.Errorf("ensureFetchRefspec() error = %v", err)
	}

	// Should only check, not set
	if len(mock.Calls) != 1 {
		t.Errorf("Expected 1 call (check only), got %d", len(mock.Calls))
	}
}

func TestCloneManager_ensureFetchRefspec_NotConfigured(t *testing.T) {
	t.Parallel()

	runCalls := 0

	mock := &MockCommandRunner{
		OutputFunc: func(dir string, name string, args ...string) ([]byte, error) {
			return []byte{}, errors.New("not found")
		},
		RunFunc: func(dir string, name string, args ...string) error {
			runCalls++
			// Verify correct refspec is set
			if len(args) >= 3 && args[0] == "config" {
				if args[2] != "+refs/heads/*:refs/remotes/origin/*" {
					t.Errorf("Wrong refspec: %s", args[2])
				}
			}
			return nil
		},
	}

	cm := NewCloneManagerWithRunner("", false, mock)
	err := cm.ensureFetchRefspec("/repo")
	if err != nil {
		t.Errorf("ensureFetchRefspec() error = %v", err)
	}

	if runCalls != 1 {
		t.Errorf("Expected config to be set once, got %d calls", runCalls)
	}
}

func TestCloneManager_detectDefaultBranch_SymbolicRef(t *testing.T) {
	t.Parallel()

	mock := &MockCommandRunner{
		OutputFunc: func(dir string, name string, args ...string) ([]byte, error) {
			if len(args) > 0 && args[0] == "symbolic-ref" {
				return []byte("refs/remotes/origin/develop\n"), nil
			}
			return []byte{}, nil
		},
		RunFunc: func(dir string, name string, args ...string) error {
			if len(args) > 0 && args[0] == "show-ref" {
				// develop branch exists
				if strings.Contains(args[3], "origin/develop") {
					return nil
				}
			}
			return errors.New("not found")
		},
	}

	cm := NewCloneManagerWithRunner("", false, mock)
	branch, err := cm.detectDefaultBranch("/repo")
	if err != nil {
		t.Fatalf("detectDefaultBranch() error = %v", err)
	}
	if branch != "develop" {
		t.Errorf("detectDefaultBranch() = %q, want %q", branch, "develop")
	}
}

func TestCloneManager_detectDefaultBranch_FallbackMain(t *testing.T) {
	t.Parallel()

	mock := &MockCommandRunner{
		OutputFunc: func(dir string, name string, args ...string) ([]byte, error) {
			if len(args) > 0 && args[0] == "symbolic-ref" {
				return []byte{}, errors.New("not found")
			}
			return []byte{}, nil
		},
		RunFunc: func(dir string, name string, args ...string) error {
			if len(args) > 0 && args[0] == "show-ref" {
				if strings.Contains(args[3], "origin/main") {
					return nil
				}
			}
			return errors.New("not found")
		},
	}

	cm := NewCloneManagerWithRunner("", false, mock)
	branch, err := cm.detectDefaultBranch("/repo")
	if err != nil {
		t.Fatalf("detectDefaultBranch() error = %v", err)
	}
	if branch != "main" {
		t.Errorf("detectDefaultBranch() = %q, want %q", branch, "main")
	}
}

func TestCloneManager_detectDefaultBranch_FallbackMaster(t *testing.T) {
	t.Parallel()

	mock := &MockCommandRunner{
		OutputFunc: func(dir string, name string, args ...string) ([]byte, error) {
			if len(args) > 0 && args[0] == "symbolic-ref" {
				return []byte{}, errors.New("not found")
			}
			return []byte{}, nil
		},
		RunFunc: func(dir string, name string, args ...string) error {
			if len(args) > 0 && args[0] == "show-ref" {
				if strings.Contains(args[3], "origin/master") {
					return nil
				}
			}
			return errors.New("not found")
		},
	}

	cm := NewCloneManagerWithRunner("", false, mock)
	branch, err := cm.detectDefaultBranch("/repo")
	if err != nil {
		t.Fatalf("detectDefaultBranch() error = %v", err)
	}
	if branch != "master" {
		t.Errorf("detectDefaultBranch() = %q, want %q", branch, "master")
	}
}

func TestCloneManager_detectDefaultBranch_FirstRemote(t *testing.T) {
	t.Parallel()

	mock := &MockCommandRunner{
		OutputFunc: func(dir string, name string, args ...string) ([]byte, error) {
			if len(args) > 0 && args[0] == "symbolic-ref" {
				return []byte{}, errors.New("not found")
			}
			if len(args) > 0 && args[0] == "branch" {
				return []byte("  origin/HEAD -> origin/custom\n  origin/custom\n"), nil
			}
			return []byte{}, nil
		},
		RunFunc: func(dir string, name string, args ...string) error {
			// Neither main nor master exist
			return errors.New("not found")
		},
	}

	cm := NewCloneManagerWithRunner("", false, mock)
	branch, err := cm.detectDefaultBranch("/repo")
	if err != nil {
		t.Fatalf("detectDefaultBranch() error = %v", err)
	}
	if branch != "custom" {
		t.Errorf("detectDefaultBranch() = %q, want %q", branch, "custom")
	}
}

func TestNewCloneManager(t *testing.T) {
	cm := NewCloneManager("/base/path", true)

	if cm.BasePath != "/base/path" {
		t.Errorf("BasePath = %q, want %q", cm.BasePath, "/base/path")
	}
	if !cm.Verbose {
		t.Error("Verbose = false, want true")
	}
	if cm.runner == nil {
		t.Error("runner should not be nil")
	}
}

func TestNewCloneManagerWithRunner(t *testing.T) {
	mock := &MockCommandRunner{}
	cm := NewCloneManagerWithRunner("/base/path", false, mock)

	if cm.BasePath != "/base/path" {
		t.Errorf("BasePath = %q, want %q", cm.BasePath, "/base/path")
	}
	if cm.Verbose {
		t.Error("Verbose = true, want false")
	}
	if cm.runner != mock {
		t.Error("runner should be the provided mock")
	}
}
