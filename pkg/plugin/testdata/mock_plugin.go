package main

import (
	"fmt"
	"net"
	"os"
	"time"
)

func main() {
	endpoint := os.Getenv("RIG_PLUGIN_ENDPOINT")
	if endpoint == "" {
		fmt.Fprintf(os.Stderr, "RIG_PLUGIN_ENDPOINT is empty\n")
		os.Exit(1)
	}

	// Optional check for RIG_HOST_ENDPOINT
	expectedHost := os.Getenv("EXPECTED_HOST_ENDPOINT")
	if expectedHost != "" {
		actualHost := os.Getenv("RIG_HOST_ENDPOINT")
		if actualHost != expectedHost {
			fmt.Fprintf(os.Stderr, "Expected RIG_HOST_ENDPOINT=%q, got %q\n", expectedHost, actualHost)
			os.Exit(1)
		}
	}

	lis, err := net.Listen("unix", endpoint)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to listen: %v\n", err)
		os.Exit(1)
	}
	defer lis.Close()

	// Handle handshake dial
	conn, err := lis.Accept()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to accept: %v\n", err)
		os.Exit(1)
	}
	_ = conn.Close()

	// Wait a tiny bit to avoid WaitDelay issues in the host
	time.Sleep(100 * time.Millisecond)
}
