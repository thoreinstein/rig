package daemon

import (
	"context"
	"fmt"
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
			errCh <- fmt.Errorf("unexpected resp.Value: %q", resp.Value)
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

func TestDaemonUIProxy_TableDriven(t *testing.T) {
	tests := []struct {
		name     string
		call     func(context.Context, *DaemonUIProxy) (any, error)
		response func(string) *apiv1.InteractRequest
		validate func(any) error
	}{
		{
			name: "Confirm",
			call: func(ctx context.Context, p *DaemonUIProxy) (any, error) {
				return p.Confirm(ctx, &apiv1.ConfirmRequest{Label: "Yes?"})
			},
			response: func(id string) *apiv1.InteractRequest {
				return &apiv1.InteractRequest{
					ResponseTo: id,
					Payload:    &apiv1.InteractRequest_Confirm{Confirm: &apiv1.ConfirmResponse{Confirmed: true}},
				}
			},
			validate: func(resp any) error {
				if r, ok := resp.(*apiv1.ConfirmResponse); !ok || !r.Confirmed {
					return fmt.Errorf("unexpected confirm response: %v", resp)
				}
				return nil
			},
		},
		{
			name: "Select",
			call: func(ctx context.Context, p *DaemonUIProxy) (any, error) {
				return p.Select(ctx, &apiv1.SelectRequest{Label: "Pick", Options: []string{"A", "B"}})
			},
			response: func(id string) *apiv1.InteractRequest {
				return &apiv1.InteractRequest{
					ResponseTo: id,
					Payload:    &apiv1.InteractRequest_Select{Select: &apiv1.SelectResponse{SelectedIndices: []uint32{1}}},
				}
			},
			validate: func(resp any) error {
				if r, ok := resp.(*apiv1.SelectResponse); !ok || len(r.SelectedIndices) != 1 || r.SelectedIndices[0] != 1 {
					return fmt.Errorf("unexpected select response: %v", resp)
				}
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proxy := NewDaemonUIProxy()

			// 1. Error when no session
			_, err := tt.call(t.Context(), proxy)
			if err == nil {
				t.Fatal("expected error when no session active")
			}

			// 2. Successful round-trip
			sendCh := make(chan *apiv1.InteractResponse, 1)
			bridge := proxy.SetActiveSession(func(resp *apiv1.InteractResponse) error {
				sendCh <- resp
				return nil
			})

			resCh := make(chan any, 1)
			errCh := make(chan error, 1)
			go func() {
				res, err := tt.call(t.Context(), proxy)
				if err != nil {
					errCh <- err
					return
				}
				resCh <- res
			}()

			var req *apiv1.InteractResponse
			select {
			case req = <-sendCh:
			case <-time.After(1 * time.Second):
				t.Fatal("timeout waiting for UI request")
			}

			bridge.HandleResponse(tt.response(req.Id))

			select {
			case res := <-resCh:
				if err := tt.validate(res); err != nil {
					t.Fatal(err)
				}
			case err := <-errCh:
				t.Fatalf("call failed: %v", err)
			case <-time.After(1 * time.Second):
				t.Fatal("timeout waiting for response")
			}

			// 3. Error after clear
			proxy.ClearActiveSession()
			_, err = tt.call(t.Context(), proxy)
			if err == nil {
				t.Fatal("expected error after clearing session")
			}
		})
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
