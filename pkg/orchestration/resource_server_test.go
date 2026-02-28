package orchestration

import (
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
)

func TestResourceServer_ReadFile(t *testing.T) {
	tmpDir := t.TempDir()
	wsDir := filepath.Join(tmpDir, "workspace")
	err := os.MkdirAll(wsDir, 0o755)
	if err != nil {
		t.Fatalf("failed to create workspace dir: %v", err)
	}

	testFile := filepath.Join(wsDir, "test.txt")
	err = os.WriteFile(testFile, []byte("hello world"), 0o644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	outsideFile := filepath.Join(tmpDir, "secret.txt")
	err = os.WriteFile(outsideFile, []byte("secret data"), 0o644)
	if err != nil {
		t.Fatalf("failed to write outside file: %v", err)
	}

	caps := &NodeCapabilities{
		Workspace: wsDir,
	}
	server := newResourceServer("node-1", caps)
	ctx := t.Context()

	// Test 1: Allowed read
	resp, err := server.ReadFile(ctx, &apiv1.ReadFileRequest{Path: testFile})
	if err != nil {
		t.Fatalf("expected no error for allowed read, got: %v", err)
	}
	if string(resp.Content) != "hello world" {
		t.Errorf("expected 'hello world', got %q", string(resp.Content))
	}

	// Test 2: Denied read (outside workspace)
	_, err = server.ReadFile(ctx, &apiv1.ReadFileRequest{Path: outsideFile})
	if err == nil {
		t.Fatalf("expected error for denied read, got nil")
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.PermissionDenied {
		t.Errorf("expected PermissionDenied, got: %v", err)
	}

	// Test 3: Directory traversal attempt
	traversalPath := filepath.Join(wsDir, "..", "secret.txt")
	_, err = server.ReadFile(ctx, &apiv1.ReadFileRequest{Path: traversalPath})
	if err == nil {
		t.Fatalf("expected error for directory traversal, got nil")
	}
	st, ok = status.FromError(err)
	if !ok || st.Code() != codes.PermissionDenied {
		t.Errorf("expected PermissionDenied, got: %v", err)
	}
}

func TestResourceServer_HttpRequest(t *testing.T) {
	// Note: We don't actually make network calls here to avoid flaky tests,
	// we just test the capability enforcement check before the call is made.
	// For the allowed case, we expect an InvalidArgument or Internal error
	// depending on how http.NewRequest/client.Do reacts to a fake URL,
	// but crucially NOT a PermissionDenied error.

	ctx := t.Context()

	t.Run("denied", func(t *testing.T) {
		caps := &NodeCapabilities{NetworkAccess: false}
		server := newResourceServer("node-1", caps)

		_, err := server.HttpRequest(ctx, &apiv1.HttpRequestRequest{
			Method: "GET",
			Url:    "http://example.com",
		})
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.PermissionDenied {
			t.Errorf("expected PermissionDenied, got: %v", err)
		}
	})

	t.Run("allowed", func(t *testing.T) {
		caps := &NodeCapabilities{NetworkAccess: true}
		server := newResourceServer("node-1", caps)

		_, err := server.HttpRequest(ctx, &apiv1.HttpRequestRequest{
			Method: "GET",
			Url:    "://invalid-url", // Use invalid URL so it fails quickly without networking
		})
		if err == nil {
			t.Fatalf("expected error due to invalid url, got nil")
		}
		st, ok := status.FromError(err)
		if ok && st.Code() == codes.PermissionDenied {
			t.Errorf("expected non-PermissionDenied error, got: %v", err)
		}
	})
}
