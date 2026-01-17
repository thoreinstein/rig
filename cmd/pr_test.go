package cmd

import (
	"context"

	"thoreinstein.com/rig/pkg/github"
)

type mockGHClient struct {
	github.Client
	isAuthenticated bool
	createPRFunc    func(ctx context.Context, opts github.CreatePROptions) (*github.PRInfo, error)
	listPRsFunc     func(ctx context.Context, state, author string) ([]github.PRInfo, error)
	getPRFunc       func(ctx context.Context, number int) (*github.PRInfo, error)
	mergePRFunc     func(ctx context.Context, number int, opts github.MergeOptions) error
}

func (m *mockGHClient) IsAuthenticated() bool {
	return m.isAuthenticated
}

func (m *mockGHClient) CreatePR(ctx context.Context, opts github.CreatePROptions) (*github.PRInfo, error) {
	if m.createPRFunc != nil {
		return m.createPRFunc(ctx, opts)
	}
	return &github.PRInfo{
		Number: 1,
		Title:  opts.Title,
		URL:    "https://github.com/test/repo/pull/1",
		Draft:  opts.Draft,
	}, nil
}

func (m *mockGHClient) ListPRs(ctx context.Context, state, author string) ([]github.PRInfo, error) {
	if m.listPRsFunc != nil {
		return m.listPRsFunc(ctx, state, author)
	}
	return []github.PRInfo{
		{Number: 1, Title: "PR 1", State: "open", HeadBranch: "feat-1"},
		{Number: 2, Title: "PR 2", State: "open", HeadBranch: "feat-2"},
	}, nil
}

func (m *mockGHClient) GetPR(ctx context.Context, number int) (*github.PRInfo, error) {
	if m.getPRFunc != nil {
		return m.getPRFunc(ctx, number)
	}
	return &github.PRInfo{Number: number, Title: "Test PR", State: "open"}, nil
}

func (m *mockGHClient) MergePR(ctx context.Context, number int, opts github.MergeOptions) error {
	if m.mergePRFunc != nil {
		return m.mergePRFunc(ctx, number, opts)
	}
	return nil
}
