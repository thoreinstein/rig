package sdk

import (
	"context"
	"fmt"
	"testing"

	"github.com/cockroachdb/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestMapError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected codes.Code
	}{
		{"nil", nil, codes.OK},
		{"canceled", context.Canceled, codes.Canceled},
		{"deadline", context.DeadlineExceeded, codes.DeadlineExceeded},
		{"wrapped_canceled", fmt.Errorf("op failed: %w", context.Canceled), codes.Canceled},
		{"wrapped_deadline", fmt.Errorf("op failed: %w", context.DeadlineExceeded), codes.DeadlineExceeded},
		{"already_status", status.Error(codes.NotFound, "not found"), codes.NotFound},
		{"wrapped_status", errors.Wrap(status.Error(codes.AlreadyExists, "exists"), "wrapped"), codes.AlreadyExists},
		{"already_status_internal", status.Errorf(codes.Internal, "oops"), codes.Internal},
		{"generic_error", errors.New("something broke"), codes.Internal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapError(tt.err)
			if tt.err == nil {
				if got != nil {
					t.Errorf("mapError() got = %v, want nil", got)
				}
				return
			}

			s, ok := status.FromError(got)
			if !ok {
				t.Fatalf("mapError() returned non-status error: %v", got)
			}

			if s.Code() != tt.expected {
				t.Errorf("mapError() code = %v, want %v", s.Code(), tt.expected)
			}
		})
	}
}
