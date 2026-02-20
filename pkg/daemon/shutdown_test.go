//go:build integration

package daemon

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"google.golang.org/grpc"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
	"thoreinstein.com/rig/pkg/plugin"
)

func TestDaemon_ShutdownRPC(t *testing.T) {
	// On Darwin, t.TempDir() can be very long, exceeding AF_UNIX limit.
	daemonBase := filepath.Join("/tmp", fmt.Sprintf("rig-shutdown-test-%d", os.Getpid()))
	if err := os.MkdirAll(daemonBase, 0700); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(daemonBase)

	t.Setenv("XDG_RUNTIME_DIR", daemonBase)

	if err := EnsureDir(); err != nil {
		t.Fatal(err)
	}

	// 1. Setup Server
	executor := plugin.NewExecutor("")
	scanner, _ := plugin.NewScanner()
	uiProxy := NewDaemonUIProxy()
	mgr, err := plugin.NewManager(executor, scanner, "1.0.0", nil, nil, plugin.WithUIServer(uiProxy))
	if err != nil {
		t.Fatal(err)
	}
	server := NewDaemonServer(mgr, uiProxy, "1.0.0", nil)

	path := SocketPath()
	lis, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}

	s := grpc.NewServer()
	apiv1.RegisterDaemonServiceServer(s, server)

	serverExit := make(chan error, 1)
	go func() {
		serverExit <- s.Serve(lis)
	}()

	// Watch for shutdown signal
	shutdownTriggered := make(chan struct{})
	go func() {
		<-server.ShutdownCh()
		close(shutdownTriggered)
		s.GracefulStop()
	}()

	// 2. Setup Client and Call Shutdown
	client, err := NewClient(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	err = client.Shutdown(t.Context(), false)
	if err != nil {
		t.Fatalf("Shutdown RPC failed: %v", err)
	}

	// 3. Verify Server Exited
	select {
	case <-shutdownTriggered:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for daemon to trigger shutdown channel")
	}

	select {
	case err := <-serverExit:
		if err != nil && err != grpc.ErrServerStopped {
			t.Errorf("server exited with error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for server to stop")
	}
}
