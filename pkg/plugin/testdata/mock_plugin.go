package main

import (
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

func main() {
	endpoint := os.Getenv("RIG_PLUGIN_ENDPOINT")
	if endpoint == "" {
		fmt.Fprintf(os.Stderr, "RIG_PLUGIN_ENDPOINT is empty\n")
		os.Exit(1)
	}

	// Optional check for environment variables
	expectedEnv := os.Getenv("EXPECTED_ENV_VARS")
	if expectedEnv != "" {
		for _, key := range strings.Split(expectedEnv, ",") {
			if os.Getenv(key) == "" {
				fmt.Fprintf(os.Stderr, "Expected environment variable %q is empty or not set\n", key)
				os.Exit(1)
			}
		}
	}

	blockedEnv := os.Getenv("BLOCKED_ENV_VARS")
	if blockedEnv != "" {
		for _, key := range strings.Split(blockedEnv, ",") {
			if os.Getenv(key) != "" {
				fmt.Fprintf(os.Stderr, "Environment variable %q should be blocked but is set to %q\n", key, os.Getenv(key))
				os.Exit(1)
			}
		}
	}

	// Verify RIG_HOST_ENDPOINT matches expected value if specified
	if expected := os.Getenv("EXPECTED_HOST_ENDPOINT"); expected != "" {
		actual := os.Getenv("RIG_HOST_ENDPOINT")
		if actual != expected {
			fmt.Fprintf(os.Stderr, "RIG_HOST_ENDPOINT mismatch: got %q, want %q\n", actual, expected)
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
