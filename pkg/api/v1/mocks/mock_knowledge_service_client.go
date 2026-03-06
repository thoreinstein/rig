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
	return ret.Get(0).(*apiv1.CreateTicketNoteResponse), ret.Error(1)
}

func (m *MockKnowledgeServiceClient) UpdateDailyNote(ctx context.Context, in *apiv1.UpdateDailyNoteRequest, opts ...grpc.CallOption) (*apiv1.UpdateDailyNoteResponse, error) {
	args := m.expandArgs(ctx, in, opts)
	ret := m.Called(args...)
	return ret.Get(0).(*apiv1.UpdateDailyNoteResponse), ret.Error(1)
}

func (m *MockKnowledgeServiceClient) GetNotePath(ctx context.Context, in *apiv1.GetNotePathRequest, opts ...grpc.CallOption) (*apiv1.GetNotePathResponse, error) {
	args := m.expandArgs(ctx, in, opts)
	ret := m.Called(args...)
	return ret.Get(0).(*apiv1.GetNotePathResponse), ret.Error(1)
}

func (m *MockKnowledgeServiceClient) GetDailyNotePath(ctx context.Context, in *apiv1.GetDailyNotePathRequest, opts ...grpc.CallOption) (*apiv1.GetDailyNotePathResponse, error) {
	args := m.expandArgs(ctx, in, opts)
	ret := m.Called(args...)
	return ret.Get(0).(*apiv1.GetDailyNotePathResponse), ret.Error(1)
}

func (m *MockKnowledgeServiceClient) expandArgs(ctx context.Context, in interface{}, opts []grpc.CallOption) []interface{} {
	args := make([]interface{}, 0, 2+len(opts))
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

func (e *MockKnowledgeServiceClient_Expecter) CreateTicketNote(ctx, in interface{}, opts ...interface{}) *mock.Call {
	return e.mock.On("CreateTicketNote", append([]interface{}{ctx, in}, opts...)...)
}

func (e *MockKnowledgeServiceClient_Expecter) UpdateDailyNote(ctx, in interface{}, opts ...interface{}) *mock.Call {
	return e.mock.On("UpdateDailyNote", append([]interface{}{ctx, in}, opts...)...)
}

func (e *MockKnowledgeServiceClient_Expecter) GetNotePath(ctx, in interface{}, opts ...interface{}) *mock.Call {
	return e.mock.On("GetNotePath", append([]interface{}{ctx, in}, opts...)...)
}

func (e *MockKnowledgeServiceClient_Expecter) GetDailyNotePath(ctx, in interface{}, opts ...interface{}) *mock.Call {
	return e.mock.On("GetDailyNotePath", append([]interface{}{ctx, in}, opts...)...)
}
