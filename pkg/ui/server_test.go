package ui

import (
	"context"
	"io"
	"os"
	"testing"
	"time"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
)

func TestUIServer_Prompt(t *testing.T) {
	// Setup mock stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	oldStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = oldStdin }()

	// Write response to pipe
	go func() {
		defer w.Close()
		_, _ = io.WriteString(w, "my-input\n")
	}()

	srv := NewUIServer()
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
			r, w, _ := os.Pipe()
			oldStdin := os.Stdin
			os.Stdin = r
			defer func() { os.Stdin = oldStdin; r.Close() }()

			go func() {
				defer w.Close()
				_, _ = io.WriteString(w, tt.input)
			}()

			srv := NewUIServer()
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
	r, w, _ := os.Pipe()
	oldStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = oldStdin; r.Close() }()

	go func() {
		defer w.Close()
		_, _ = io.WriteString(w, "2\n") // Select second option
	}()

	srv := NewUIServer()
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
	srv := NewUIServer()
	
	// Start a prompt that we'll block
	r, w, _ := os.Pipe()
	oldStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = oldStdin; r.Close() }()

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
	w.Close()
	<-promptDone

	// Now unblock second prompt
	r2, w2, _ := os.Pipe()
	os.Stdin = r2 // Switch stdin for the second operation
	go func() {
		defer w2.Close()
		_, _ = io.WriteString(w2, "y\n")
	}()

	select {
	case <-secondDone:
		// Success
	case <-time.After(500 * time.Millisecond):
		t.Fatal("second operation never unblocked")
	}
}

func TestUIServer_Cancellation(t *testing.T) {
	srv := NewUIServer()
	
	// Setup mock stdin but don't write anything yet
	r, w, _ := os.Pipe()
	oldStdin := os.Stdin
	os.Stdin = r
	defer func() { 
		os.Stdin = oldStdin
		w.Close()
		r.Close() 
	}()

	// Start a prompt with a short-lived context
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	// This should return a context deadline exceeded error without blocking indefinitely
	_, err := srv.Prompt(ctx, &apiv1.PromptRequest{Label: "Timed out prompt:"})
	
	if err == nil {
		t.Error("expected context timeout error, got nil")
	}
	if err != context.DeadlineExceeded {
		t.Errorf("expected context.DeadlineExceeded, got %v", err)
	}

	// Verify the terminal is still usable (lock was released or never held)
	r2, w2, _ := os.Pipe()
	os.Stdin = r2
	go func() {
		defer w2.Close()
		_, _ = io.WriteString(w2, "y\n")
	}()

	_, err = srv.Confirm(t.Context(), &apiv1.ConfirmRequest{Label: "Immediate follow-up:"})
	if err != nil {
		t.Errorf("subsequent UI call failed after cancellation: %v", err)
	}
}
