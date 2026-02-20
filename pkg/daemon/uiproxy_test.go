package daemon

import (
	"testing"
	"time"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
)

func TestDaemonUIProxy_Prompt(t *testing.T) {
	proxy := NewDaemonUIProxy()

	// Should fail if no session active
	_, err := proxy.Prompt(t.Context(), &apiv1.PromptRequest{})
	if err == nil {
		t.Fatal("expected error when no session is active")
	}

	sendCh := make(chan *apiv1.InteractResponse, 1)
	bridge := proxy.SetActiveSession(func(resp *apiv1.InteractResponse) error {
		sendCh <- resp
		return nil
	})

	// Run Prompt in background
	errCh := make(chan error, 1)
	go func() {
		resp, err := proxy.Prompt(t.Context(), &apiv1.PromptRequest{Label: "Test"})
		if err != nil {
			errCh <- err
			return
		}
		if resp.Value != "Response" {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	// Wait for request
	var req *apiv1.InteractResponse
	select {
	case req = <-sendCh:
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for UI request")
	}

	// Send response back
	bridge.HandleResponse(&apiv1.InteractRequest{
		ResponseTo: req.Id,
		Payload: &apiv1.InteractRequest_Prompt{
			Prompt: &apiv1.PromptResponse{Value: "Response"},
		},
	})

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Prompt failed: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for Prompt to return")
	}

	proxy.ClearActiveSession()
	_, err = proxy.Prompt(t.Context(), &apiv1.PromptRequest{})
	if err == nil {
		t.Fatal("expected error after clearing session")
	}
}

func TestDaemonUIProxy_Progress(t *testing.T) {
	proxy := NewDaemonUIProxy()

	// Should NOT fail if no session active (fire and forget)
	_, err := proxy.UpdateProgress(t.Context(), &apiv1.UpdateProgressRequest{})
	if err != nil {
		t.Fatalf("UpdateProgress failed without active session: %v", err)
	}

	sendCh := make(chan *apiv1.InteractResponse, 1)
	_ = proxy.SetActiveSession(func(resp *apiv1.InteractResponse) error {
		sendCh <- resp
		return nil
	})

	_, err = proxy.UpdateProgress(t.Context(), &apiv1.UpdateProgressRequest{
		Progress: &apiv1.ProgressUpdate{Message: "Test"},
	})
	if err != nil {
		t.Fatalf("UpdateProgress failed with active session: %v", err)
	}

	select {
	case <-sendCh:
		// Success
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for progress update")
	}
}
