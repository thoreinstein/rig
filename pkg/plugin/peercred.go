package plugin

import (
	"errors"

	"google.golang.org/grpc/peer"
)

// extractPID attempts to extract the caller's PID from the gRPC peer connection.
// Currently returns an error on all platforms — PID extraction requires raw fd
// access that gRPC does not expose in the interceptor peer info.
//
// When implemented, darwin would use LOCAL_PEERCRED and linux would use
// getsockopt(SO_PEERCRED). Until then, UDS directory isolation is the
// sole authentication boundary.
func extractPID(_ *peer.Peer) (int, error) {
	return 0, errors.New("PID extraction not implemented")
}
