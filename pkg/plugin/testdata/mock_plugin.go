package main

import (
	"net"
	"os"
	"time"
)

func main() {
	endpoint := os.Getenv("RIG_PLUGIN_ENDPOINT")
	if endpoint == "" {
		os.Exit(1)
	}
	lis, err := net.Listen("unix", endpoint)
	if err != nil {
		os.Exit(1)
	}
	defer lis.Close()

	// Handle handshake dial
	conn, err := lis.Accept()
	if err != nil {
		os.Exit(1)
	}
	_ = conn.Close()

	// Wait a tiny bit to avoid WaitDelay issues in the host
	time.Sleep(100 * time.Millisecond)
}
