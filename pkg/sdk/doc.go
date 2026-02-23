// Package sdk provides a high-level library for building Rig plugins in Go.
//
// It abstracts the underlying gRPC-over-UDS protocol, handles the Handshake process,
// and provides a simplified interface for implementing Assistant and Command capabilities.
//
// Basic usage:
//
//	type myPlugin struct{}
//	func (p *myPlugin) Info() sdk.Info { ... }
//	func (p *myPlugin) Chat(...) { ... }
//
//	func main() {
//		if err := sdk.Serve(&myPlugin{}); err != nil {
//			log.Fatal(err)
//		}
//	}
package sdk
