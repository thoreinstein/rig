package vcs

import (
	"context"
	"testing"
	"time"

	"github.com/cockroachdb/errors"
	"google.golang.org/grpc"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
)

// mockPluginManager tracks calls to GetVCSClient and ReleasePlugin.
type mockPluginManager struct {
	client       apiv1.VCSServiceClient
	err          error
	releaseCalls int
	lastReleased string
	lastCtx      context.Context
}

func (m *mockPluginManager) GetVCSClient(ctx context.Context, name string) (apiv1.VCSServiceClient, error) {
	m.lastCtx = ctx
	if m.err != nil {
		return nil, m.err
	}
	return m.client, nil
}

func (m *mockPluginManager) ReleasePlugin(name string) {
	m.releaseCalls++
	m.lastReleased = name
}

// mockVCSClient implements apiv1.VCSServiceClient.
type mockVCSClient struct {
	getRepoRootFn         func(ctx context.Context, in *apiv1.GetRepoRootRequest, opts ...grpc.CallOption) (*apiv1.GetRepoRootResponse, error)
	getRepoNameFn         func(ctx context.Context, in *apiv1.GetRepoNameRequest, opts ...grpc.CallOption) (*apiv1.GetRepoNameResponse, error)
	getDefaultBranchFn    func(ctx context.Context, in *apiv1.GetDefaultBranchRequest, opts ...grpc.CallOption) (*apiv1.GetDefaultBranchResponse, error)
	createWorktreeFn      func(ctx context.Context, in *apiv1.CreateWorktreeRequest, opts ...grpc.CallOption) (*apiv1.CreateWorktreeResponse, error)
	listWorktreesFn       func(ctx context.Context, in *apiv1.ListWorktreesRequest, opts ...grpc.CallOption) (*apiv1.ListWorktreesResponse, error)
	removeWorktreeFn      func(ctx context.Context, in *apiv1.RemoveWorktreeRequest, opts ...grpc.CallOption) (*apiv1.RemoveWorktreeResponse, error)
	forceRemoveWorktreeFn func(ctx context.Context, in *apiv1.ForceRemoveWorktreeRequest, opts ...grpc.CallOption) (*apiv1.ForceRemoveWorktreeResponse, error)
	getWorktreePathFn     func(ctx context.Context, in *apiv1.GetWorktreePathRequest, opts ...grpc.CallOption) (*apiv1.GetWorktreePathResponse, error)
	cloneFn               func(ctx context.Context, in *apiv1.CloneRequest, opts ...grpc.CallOption) (*apiv1.CloneResponse, error)
	isBranchMergedFn      func(ctx context.Context, in *apiv1.IsBranchMergedRequest, opts ...grpc.CallOption) (*apiv1.IsBranchMergedResponse, error)
}

// Ensure it implements the client interface.
var _ apiv1.VCSServiceClient = (*mockVCSClient)(nil)

func (m *mockVCSClient) GetRepoRoot(ctx context.Context, in *apiv1.GetRepoRootRequest, opts ...grpc.CallOption) (*apiv1.GetRepoRootResponse, error) {
	if m.getRepoRootFn != nil {
		return m.getRepoRootFn(ctx, in, opts...)
	}
	return nil, nil
}

func (m *mockVCSClient) GetRepoName(ctx context.Context, in *apiv1.GetRepoNameRequest, opts ...grpc.CallOption) (*apiv1.GetRepoNameResponse, error) {
	if m.getRepoNameFn != nil {
		return m.getRepoNameFn(ctx, in, opts...)
	}
	return nil, nil
}

func (m *mockVCSClient) GetDefaultBranch(ctx context.Context, in *apiv1.GetDefaultBranchRequest, opts ...grpc.CallOption) (*apiv1.GetDefaultBranchResponse, error) {
	if m.getDefaultBranchFn != nil {
		return m.getDefaultBranchFn(ctx, in, opts...)
	}
	return nil, nil
}

func (m *mockVCSClient) CreateWorktree(ctx context.Context, in *apiv1.CreateWorktreeRequest, opts ...grpc.CallOption) (*apiv1.CreateWorktreeResponse, error) {
	if m.createWorktreeFn != nil {
		return m.createWorktreeFn(ctx, in, opts...)
	}
	return nil, nil
}

func (m *mockVCSClient) ListWorktrees(ctx context.Context, in *apiv1.ListWorktreesRequest, opts ...grpc.CallOption) (*apiv1.ListWorktreesResponse, error) {
	if m.listWorktreesFn != nil {
		return m.listWorktreesFn(ctx, in, opts...)
	}
	return nil, nil
}

func (m *mockVCSClient) RemoveWorktree(ctx context.Context, in *apiv1.RemoveWorktreeRequest, opts ...grpc.CallOption) (*apiv1.RemoveWorktreeResponse, error) {
	if m.removeWorktreeFn != nil {
		return m.removeWorktreeFn(ctx, in, opts...)
	}
	return nil, nil
}

func (m *mockVCSClient) ForceRemoveWorktree(ctx context.Context, in *apiv1.ForceRemoveWorktreeRequest, opts ...grpc.CallOption) (*apiv1.ForceRemoveWorktreeResponse, error) {
	if m.forceRemoveWorktreeFn != nil {
		return m.forceRemoveWorktreeFn(ctx, in, opts...)
	}
	return nil, nil
}

func (m *mockVCSClient) GetWorktreePath(ctx context.Context, in *apiv1.GetWorktreePathRequest, opts ...grpc.CallOption) (*apiv1.GetWorktreePathResponse, error) {
	if m.getWorktreePathFn != nil {
		return m.getWorktreePathFn(ctx, in, opts...)
	}
	return nil, nil
}

func (m *mockVCSClient) Clone(ctx context.Context, in *apiv1.CloneRequest, opts ...grpc.CallOption) (*apiv1.CloneResponse, error) {
	if m.cloneFn != nil {
		return m.cloneFn(ctx, in, opts...)
	}
	return nil, nil
}

func (m *mockVCSClient) IsBranchMerged(ctx context.Context, in *apiv1.IsBranchMergedRequest, opts ...grpc.CallOption) (*apiv1.IsBranchMergedResponse, error) {
	if m.isBranchMergedFn != nil {
		return m.isBranchMergedFn(ctx, in, opts...)
	}
	return nil, nil
}

func assertReleaseCalled(t *testing.T, m *mockPluginManager, expected int) {
	t.Helper()
	if m.releaseCalls != expected {
		t.Errorf("expected ReleasePlugin to be called %d times, got %d", expected, m.releaseCalls)
	}
}

func assertTimeoutApplied(t *testing.T, ctx context.Context, expectedTimeout time.Duration) {
	t.Helper()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Error("expected context to have a deadline")
		return
	}

	remaining := time.Until(deadline)
	// upperSlack accounts for minor clock drift or scheduling jitter.
	const upperSlack = 1 * time.Second
	// lowerSlack scales with expectedTimeout so short timeouts are checked tightly.
	lowerSlack := min(max(expectedTimeout/10, 50*time.Millisecond), 5*time.Second)

	if remaining < expectedTimeout-lowerSlack || remaining > expectedTimeout+upperSlack {
		t.Errorf("expected deadline around %v from now, remaining: %v", expectedTimeout, remaining)
	}
}

func TestPluginProvider_Lifecycle(t *testing.T) {
	t.Run("GetRepoRoot", func(t *testing.T) {
		mockClient := &mockVCSClient{
			getRepoRootFn: func(ctx context.Context, in *apiv1.GetRepoRootRequest, opts ...grpc.CallOption) (*apiv1.GetRepoRootResponse, error) {
				assertTimeoutApplied(t, ctx, rpcTimeout)
				if in.Path != "test-path" {
					t.Errorf("expected path test-path, got %s", in.Path)
				}
				return &apiv1.GetRepoRootResponse{Root: "repo-root"}, nil
			},
		}
		mockMgr := &mockPluginManager{client: mockClient}
		provider := NewPluginProvider(mockMgr, "test-plugin")

		root, err := provider.GetRepoRoot("test-path")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if root != "repo-root" {
			t.Errorf("expected repo-root, got %s", root)
		}
		assertReleaseCalled(t, mockMgr, 1)
	})

	t.Run("GetRepoName", func(t *testing.T) {
		mockClient := &mockVCSClient{
			getRepoNameFn: func(ctx context.Context, in *apiv1.GetRepoNameRequest, opts ...grpc.CallOption) (*apiv1.GetRepoNameResponse, error) {
				assertTimeoutApplied(t, ctx, rpcTimeout)
				return &apiv1.GetRepoNameResponse{Name: "repo-name"}, nil
			},
		}
		mockMgr := &mockPluginManager{client: mockClient}
		provider := NewPluginProvider(mockMgr, "test-plugin")

		name, err := provider.GetRepoName("test-path")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if name != "repo-name" {
			t.Errorf("expected repo-name, got %s", name)
		}
		assertReleaseCalled(t, mockMgr, 1)
	})

	t.Run("GetDefaultBranch", func(t *testing.T) {
		mockClient := &mockVCSClient{
			getDefaultBranchFn: func(ctx context.Context, in *apiv1.GetDefaultBranchRequest, opts ...grpc.CallOption) (*apiv1.GetDefaultBranchResponse, error) {
				assertTimeoutApplied(t, ctx, rpcTimeout)
				return &apiv1.GetDefaultBranchResponse{Branch: "main"}, nil
			},
		}
		mockMgr := &mockPluginManager{client: mockClient}
		provider := NewPluginProvider(mockMgr, "test-plugin")

		branch, err := provider.GetDefaultBranch("test-path", "base-config")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if branch != "main" {
			t.Errorf("expected main, got %s", branch)
		}
		assertReleaseCalled(t, mockMgr, 1)
	})

	t.Run("CreateWorktree", func(t *testing.T) {
		mockClient := &mockVCSClient{
			createWorktreeFn: func(ctx context.Context, in *apiv1.CreateWorktreeRequest, opts ...grpc.CallOption) (*apiv1.CreateWorktreeResponse, error) {
				assertTimeoutApplied(t, ctx, rpcTimeout)
				return &apiv1.CreateWorktreeResponse{WorktreePath: "wt-path"}, nil
			},
		}
		mockMgr := &mockPluginManager{client: mockClient}
		provider := NewPluginProvider(mockMgr, "test-plugin")

		path, err := provider.CreateWorktree("test-path", "task", "name", "branch", "base")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if path != "wt-path" {
			t.Errorf("expected wt-path, got %s", path)
		}
		assertReleaseCalled(t, mockMgr, 1)
	})

	t.Run("ListWorktrees", func(t *testing.T) {
		mockClient := &mockVCSClient{
			listWorktreesFn: func(ctx context.Context, in *apiv1.ListWorktreesRequest, opts ...grpc.CallOption) (*apiv1.ListWorktreesResponse, error) {
				assertTimeoutApplied(t, ctx, rpcTimeout)
				return &apiv1.ListWorktreesResponse{
					Worktrees: []*apiv1.WorktreeInfo{
						{Path: "wt1", Branch: "b1"},
						{Path: "wt2", Branch: "b2"},
					},
				}, nil
			},
		}
		mockMgr := &mockPluginManager{client: mockClient}
		provider := NewPluginProvider(mockMgr, "test-plugin")

		wts, err := provider.ListWorktrees("test-path")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(wts) != 2 || wts[0].Path != "wt1" {
			t.Errorf("unexpected worktrees: %+v", wts)
		}
		assertReleaseCalled(t, mockMgr, 1)
	})

	t.Run("RemoveWorktree", func(t *testing.T) {
		mockClient := &mockVCSClient{
			removeWorktreeFn: func(ctx context.Context, in *apiv1.RemoveWorktreeRequest, opts ...grpc.CallOption) (*apiv1.RemoveWorktreeResponse, error) {
				assertTimeoutApplied(t, ctx, rpcTimeout)
				return &apiv1.RemoveWorktreeResponse{}, nil
			},
		}
		mockMgr := &mockPluginManager{client: mockClient}
		provider := NewPluginProvider(mockMgr, "test-plugin")

		err := provider.RemoveWorktree("test-path", "task", "ticket")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertReleaseCalled(t, mockMgr, 1)
	})

	t.Run("ForceRemoveWorktree", func(t *testing.T) {
		mockClient := &mockVCSClient{
			forceRemoveWorktreeFn: func(ctx context.Context, in *apiv1.ForceRemoveWorktreeRequest, opts ...grpc.CallOption) (*apiv1.ForceRemoveWorktreeResponse, error) {
				assertTimeoutApplied(t, ctx, rpcTimeout)
				return &apiv1.ForceRemoveWorktreeResponse{}, nil
			},
		}
		mockMgr := &mockPluginManager{client: mockClient}
		provider := NewPluginProvider(mockMgr, "test-plugin")

		err := provider.ForceRemoveWorktree("test-path", "wt-path")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertReleaseCalled(t, mockMgr, 1)
	})

	t.Run("GetWorktreePath", func(t *testing.T) {
		mockClient := &mockVCSClient{
			getWorktreePathFn: func(ctx context.Context, in *apiv1.GetWorktreePathRequest, opts ...grpc.CallOption) (*apiv1.GetWorktreePathResponse, error) {
				assertTimeoutApplied(t, ctx, rpcTimeout)
				return &apiv1.GetWorktreePathResponse{WorktreePath: "wt-path"}, nil
			},
		}
		mockMgr := &mockPluginManager{client: mockClient}
		provider := NewPluginProvider(mockMgr, "test-plugin")

		path, err := provider.GetWorktreePath("test-path", "task", "ticket")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if path != "wt-path" {
			t.Errorf("expected wt-path, got %s", path)
		}
		assertReleaseCalled(t, mockMgr, 1)
	})

	t.Run("Clone", func(t *testing.T) {
		mockClient := &mockVCSClient{
			cloneFn: func(ctx context.Context, in *apiv1.CloneRequest, opts ...grpc.CallOption) (*apiv1.CloneResponse, error) {
				assertTimeoutApplied(t, ctx, rpcLongTimeout)
				return &apiv1.CloneResponse{RepoPath: "repo-path"}, nil
			},
		}
		mockMgr := &mockPluginManager{client: mockClient}
		provider := NewPluginProvider(mockMgr, "test-plugin")

		path, err := provider.Clone("url", "base")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if path != "repo-path" {
			t.Errorf("expected repo-path, got %s", path)
		}
		assertReleaseCalled(t, mockMgr, 1)
	})

	t.Run("IsBranchMerged", func(t *testing.T) {
		mockClient := &mockVCSClient{
			isBranchMergedFn: func(ctx context.Context, in *apiv1.IsBranchMergedRequest, opts ...grpc.CallOption) (*apiv1.IsBranchMergedResponse, error) {
				assertTimeoutApplied(t, ctx, rpcTimeout)
				return &apiv1.IsBranchMergedResponse{IsMerged: true}, nil
			},
		}
		mockMgr := &mockPluginManager{client: mockClient}
		provider := NewPluginProvider(mockMgr, "test-plugin")

		merged, err := provider.IsBranchMerged("test-path", "branch", "base")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !merged {
			t.Error("expected true")
		}
		assertReleaseCalled(t, mockMgr, 1)
	})
}

func TestPluginProvider_Robustness(t *testing.T) {
	t.Run("Acquisition failure", func(t *testing.T) {
		mockMgr := &mockPluginManager{err: errors.New("failed to acquire plugin")}
		provider := NewPluginProvider(mockMgr, "test-plugin")

		if _, err := provider.GetRepoRoot("test-path"); err == nil {
			t.Error("expected error for acquisition failure")
		}
		assertReleaseCalled(t, mockMgr, 0)
	})

	t.Run("RPC failure coverage", func(t *testing.T) {
		rpcErr := errors.New("gRPC error")
		mockClient := &mockVCSClient{
			getRepoRootFn: func(ctx context.Context, in *apiv1.GetRepoRootRequest, opts ...grpc.CallOption) (*apiv1.GetRepoRootResponse, error) {
				return nil, rpcErr
			},
		}
		mockMgr := &mockPluginManager{client: mockClient}
		provider := NewPluginProvider(mockMgr, "test-plugin")

		if _, err := provider.GetRepoRoot("test-path"); err == nil {
			t.Error("expected error for RPC failure")
		}
		assertReleaseCalled(t, mockMgr, 1)
	})

	t.Run("ListWorktrees empty response", func(t *testing.T) {
		mockClient := &mockVCSClient{
			listWorktreesFn: func(ctx context.Context, in *apiv1.ListWorktreesRequest, opts ...grpc.CallOption) (*apiv1.ListWorktreesResponse, error) {
				return &apiv1.ListWorktreesResponse{Worktrees: nil}, nil
			},
		}
		mockMgr := &mockPluginManager{client: mockClient}
		provider := NewPluginProvider(mockMgr, "test-plugin")

		wts, err := provider.ListWorktrees("test-path")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if wts == nil {
			t.Error("expected non-nil empty slice")
		}
		if len(wts) != 0 {
			t.Errorf("expected 0 worktrees, got %d", len(wts))
		}
		assertReleaseCalled(t, mockMgr, 1)
	})
}
