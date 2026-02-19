package ui

import (
	"context"
	"io"
	"testing"
	"time"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
)

func TestUIServer_Prompt(t *testing.T) {
	r, w := io.Pipe()
	go func() {
		defer w.Close()
		_, _ = io.WriteString(w, "my-input\n")
	}()

	srv := NewUIServerWithReader(r)
	resp, err := srv.Prompt(t.Context(), &apiv1.PromptRequest{
		Label: "Enter something:",
	})

	if err != nil {
		t.Fatalf("Prompt() failed: %v", err)
	}

	if resp.Value != "my-input" {
		t.Errorf("expected 'my-input', got %q", resp.Value)
	}
}

func TestUIServer_Confirm(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		want     bool
		defValue bool
	}{
		{"yes", "y\n", true, false},
		{"no", "n\n", false, true},
		{"empty-default-true", "\n", true, true},
		{"empty-default-false", "\n", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, w := io.Pipe()
			go func() {
				defer w.Close()
				_, _ = io.WriteString(w, tt.input)
			}()

			srv := NewUIServerWithReader(r)
			resp, err := srv.Confirm(t.Context(), &apiv1.ConfirmRequest{
				Label:        "Confirm?",
				DefaultValue: tt.defValue,
			})

			if err != nil {
				t.Fatalf("Confirm() failed: %v", err)
			}

			if resp.Confirmed != tt.want {
				t.Errorf("got %v, want %v", resp.Confirmed, tt.want)
			}
		})
	}
}

func TestUIServer_Select(t *testing.T) {
	r, w := io.Pipe()
	go func() {
		defer w.Close()
		_, _ = io.WriteString(w, "2\n") // Select second option
	}()

	srv := NewUIServerWithReader(r)
	resp, err := srv.Select(t.Context(), &apiv1.SelectRequest{
		Label:   "Pick one:",
		Options: []string{"A", "B", "C"},
	})

	if err != nil {
		t.Fatalf("Select() failed: %v", err)
	}

	if len(resp.SelectedIndices) != 1 || resp.SelectedIndices[0] != 1 {
		t.Errorf("expected index 1, got %v", resp.SelectedIndices)
	}
}

func TestUIServer_Coordination(t *testing.T) {
	// For coordination testing, we use a single server instance with its own reader
	r, w := io.Pipe()
	srv := NewUIServerWithReader(r)
	
	promptDone := make(chan struct{})
	go func() {
		_, _ = srv.Prompt(t.Context(), &apiv1.PromptRequest{Label: "Blocking:"})
		close(promptDone)
	}()

	// Try to start another prompt while first is active
	secondDone := make(chan bool)
	go func() {
		// This should block until we provide input to the first prompt
		_, _ = srv.Confirm(t.Context(), &apiv1.ConfirmRequest{Label: "Second:"})
		secondDone <- true
	}()

	// Ensure second is blocked
	select {
	case <-secondDone:
		t.Fatal("second UI operation did not block")
	case <-time.After(100 * time.Millisecond):
		// Success: it's blocked
	}

	// Unblock first prompt
	_, _ = io.WriteString(w, "done\n")
	<-promptDone

	// Now unblock second prompt
	go func() {
		_, _ = io.WriteString(w, "y\n")
		w.Close()
	}()

	select {
	case <-secondDone:
		// Success
	case <-time.After(500 * time.Millisecond):
		t.Fatal("second operation never unblocked")
	}
}

func TestUIServer_Cancellation(t *testing.T) {
	r, w := io.Pipe()
	srv := NewUIServerWithReader(r)
	
	// Start a prompt with a short-lived context
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	_, err := srv.Prompt(ctx, &apiv1.PromptRequest{Label: "Timed out prompt:"})
	
	if err == nil {
		t.Error("expected context timeout error, got nil")
	}
	if err != context.DeadlineExceeded {
		t.Errorf("expected context.DeadlineExceeded, got %v", err)
	}

	// Important: We must provide input to the background reader so it finishes its 
	// previous blocked read and is ready for the next one.
	_, _ = io.WriteString(w, "satisfied\n")

	// Now verify the server can handle a new request
	go func() {
		_, _ = io.WriteString(w, "y\n")
		w.Close()
	}()

	_, err = srv.Confirm(t.Context(), &apiv1.ConfirmRequest{Label: "Immediate follow-up:"})
	if err != nil {
		t.Errorf("subsequent UI call failed after cancellation: %v", err)
	}
}
