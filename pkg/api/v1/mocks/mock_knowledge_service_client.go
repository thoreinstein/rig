package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc"
	apiv1 "thoreinstein.com/rig/pkg/api/v1"
)

// MockKnowledgeServiceClient is a hand-written mock that correctly handles variadic gRPC options.
// The generated mockery v3 template currently mishandles variadic argument expansion in Called(),
// leading to mismatches with its own EXPECT() helpers.
type MockKnowledgeServiceClient struct {
	mock.Mock
}

func (m *MockKnowledgeServiceClient) CreateTicketNote(ctx context.Context, in *apiv1.CreateTicketNoteRequest, opts ...grpc.CallOption) (*apiv1.CreateTicketNoteResponse, error) {
	args := m.expandArgs(ctx, in, opts)
	ret := m.Called(args...)
	val, _ := ret.Get(0).(*apiv1.CreateTicketNoteResponse)
	return val, ret.Error(1)
}

func (m *MockKnowledgeServiceClient) UpdateDailyNote(ctx context.Context, in *apiv1.UpdateDailyNoteRequest, opts ...grpc.CallOption) (*apiv1.UpdateDailyNoteResponse, error) {
	args := m.expandArgs(ctx, in, opts)
	ret := m.Called(args...)
	val, _ := ret.Get(0).(*apiv1.UpdateDailyNoteResponse)
	return val, ret.Error(1)
}

func (m *MockKnowledgeServiceClient) GetNotePath(ctx context.Context, in *apiv1.GetNotePathRequest, opts ...grpc.CallOption) (*apiv1.GetNotePathResponse, error) {
	args := m.expandArgs(ctx, in, opts)
	ret := m.Called(args...)
	val, _ := ret.Get(0).(*apiv1.GetNotePathResponse)
	return val, ret.Error(1)
}

func (m *MockKnowledgeServiceClient) GetDailyNotePath(ctx context.Context, in *apiv1.GetDailyNotePathRequest, opts ...grpc.CallOption) (*apiv1.GetDailyNotePathResponse, error) {
	args := m.expandArgs(ctx, in, opts)
	ret := m.Called(args...)
	val, _ := ret.Get(0).(*apiv1.GetDailyNotePathResponse)
	return val, ret.Error(1)
}

func (m *MockKnowledgeServiceClient) expandArgs(ctx context.Context, in any, opts []grpc.CallOption) []any {
	args := make([]any, 0, 2+len(opts))
	args = append(args, ctx, in)
	for _, opt := range opts {
		args = append(args, opt)
	}
	return args
}

// EXPECT returns an expecter for the mock.
func (m *MockKnowledgeServiceClient) EXPECT() *MockKnowledgeServiceClient_Expecter {
	return &MockKnowledgeServiceClient_Expecter{mock: &m.Mock}
}

type MockKnowledgeServiceClient_Expecter struct {
	mock *mock.Mock
}

func (e *MockKnowledgeServiceClient_Expecter) CreateTicketNote(ctx, in any, opts ...any) *mock.Call {
	return e.mock.On("CreateTicketNote", append([]any{ctx, in}, opts...)...)
}

func (e *MockKnowledgeServiceClient_Expecter) UpdateDailyNote(ctx, in any, opts ...any) *mock.Call {
	return e.mock.On("UpdateDailyNote", append([]any{ctx, in}, opts...)...)
}

func (e *MockKnowledgeServiceClient_Expecter) GetNotePath(ctx, in any, opts ...any) *mock.Call {
	return e.mock.On("GetNotePath", append([]any{ctx, in}, opts...)...)
}

func (e *MockKnowledgeServiceClient_Expecter) GetDailyNotePath(ctx, in any, opts ...any) *mock.Call {
	return e.mock.On("GetDailyNotePath", append([]any{ctx, in}, opts...)...)
}
