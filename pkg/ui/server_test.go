package ui

import (
	"io"
	"os"
	"testing"

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
	case <-t.Context().Done():
		t.Fatal("test timed out")
	default:
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
		_, _ = io.WriteString(w2, "y\n")
		w2.Close()
	}()

	select {
	case <-secondDone:
		// Success
	case <-t.Context().Done():
		t.Fatal("second operation never unblocked")
	}
}
