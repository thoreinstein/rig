package plugin

import (
	"errors"

	"google.golang.org/grpc/peer"
)

// extractPID is a stub — PID validation is NOT currently active on any platform.
// The interceptor treats any error from this function as "skip PID check," so
// returning an error here is safe and correct: the UDS directory isolation
// (0o700 parent, 0o600 socket) remains the sole authentication boundary.
//
// A future implementation would use LOCAL_PEERCRED on darwin and
// getsockopt(SO_PEERCRED) on linux, but requires raw fd access that gRPC
// does not expose through the interceptor peer info.
func extractPID(_ *peer.Peer) (int, error) {
	return 0, errors.New("PID extraction not implemented")
}
