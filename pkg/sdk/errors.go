package sdk

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// mapError maps Go errors to gRPC status codes.
// It handles context cancellation, deadline exceeded, and existing gRPC status codes.
// Fallback is codes.Internal.
func mapError(err error) error {
	if err == nil {
		return nil
	}

	// If it's already a gRPC status, pass it through
	if _, ok := status.FromError(err); ok {
		return err
	}

	switch err {
	case context.Canceled:
		return status.Error(codes.Canceled, err.Error())
	case context.DeadlineExceeded:
		return status.Error(codes.DeadlineExceeded, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}
