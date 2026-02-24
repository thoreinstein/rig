package sdk

import (
	"context"

	"github.com/cockroachdb/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ErrNoEndpoint is returned when a required endpoint environment variable is not set.
var ErrNoEndpoint = errors.New("sdk: endpoint not configured (check RIG_PLUGIN_ENDPOINT or RIG_HOST_ENDPOINT)")

// mapError maps Go errors to gRPC status codes.
// It handles context cancellation, deadline exceeded, and existing gRPC status codes.
// Wrapped context errors (e.g. fmt.Errorf("wrap: %w", context.Canceled)) are also detected.
// Fallback is codes.Internal.
func mapError(err error) error {
	if err == nil {
		return nil
	}

	// Iterate the error chain to find a gRPC status.
	for curr := err; curr != nil; curr = errors.Unwrap(curr) {
		if _, ok := status.FromError(curr); ok {
			return curr
		}
	}

	if errors.Is(err, context.Canceled) {
		return status.Error(codes.Canceled, err.Error())
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return status.Error(codes.DeadlineExceeded, err.Error())
	}

	return status.Error(codes.Internal, err.Error())
}
