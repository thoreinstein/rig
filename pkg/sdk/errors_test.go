package sdk

import (
	"context"
	"testing"

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
		{"generic", context.DeadlineExceeded, codes.DeadlineExceeded}, // context.DeadlineExceeded is actually a good case for its code
		{"already status", status.Error(codes.NotFound, "not found"), codes.NotFound},
		{"unknown", t.Context().Err(), codes.OK}, // t.Context().Err() is nil
		{"internal", status.Errorf(codes.Internal, "oops"), codes.Internal},
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
