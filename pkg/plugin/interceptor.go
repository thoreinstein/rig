package plugin

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// pluginIdentityInterceptor injects the plugin name into the gRPC context.
//
// Security model: UDS directory isolation (0o700 parent, 0o600 socket) is
// the primary authentication boundary — only the current user can connect.
// PID validation is optional defense-in-depth, applied when the platform
// supports extracting the caller's PID from the UDS connection.
func pluginIdentityInterceptor(p *Plugin) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		pe, ok := peer.FromContext(ctx)
		if !ok {
			return nil, status.Errorf(codes.Unauthenticated, "missing peer info")
		}

		// Defense-in-depth: validate PID when platform supports extraction.
		if callerPID, err := extractPID(pe); err == nil {
			p.mu.Lock()
			expectedPID := 0
			if p.process != nil {
				expectedPID = p.process.Pid
			}
			p.mu.Unlock()

			if expectedPID > 0 && callerPID != expectedPID {
				return nil, status.Errorf(codes.PermissionDenied, "connection identity validation failed")
			}
		}

		// Plugin name is always derived from the server struct, never caller-supplied.
		ctx = context.WithValue(ctx, pluginNameKey, p.Name)
		return handler(ctx, req)
	}
}

// pluginNameFromContext extracts the plugin name from the context.
func pluginNameFromContext(ctx context.Context) (string, bool) {
	name, ok := ctx.Value(pluginNameKey).(string)
	return name, ok
}
