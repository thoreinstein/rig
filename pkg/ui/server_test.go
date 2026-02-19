package ui

import (
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
)

// mockReader allows controlled line-by-line input for tests.
type mockReader struct {
	lines       chan string
	readStarted chan struct{}
}

func (m *mockReader) Read(p []byte) (n int, err error) {
	if m.readStarted != nil {
		select {
		case m.readStarted <- struct{}{}:
		default:
			// Buffer full, signal ignored to avoid deadlock
		}
	}
	line, ok := <-m.lines
	if !ok {
		return 0, io.EOF
	}
	return copy(p, line), nil
}

func TestUIServer_StandardOps(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		op      func(context.Context, *UIServer) (any, error)
		want    any
		wantErr bool
	}{
		{
			name:  "Prompt basic",
			input: "my-input\n",
			op: func(ctx context.Context, s *UIServer) (any, error) {
				return s.Prompt(ctx, &apiv1.PromptRequest{Label: "Enter:"})
			},
			want: "my-input",
		},
		{
			name:  "Prompt sensitive",
			input: "secret\n",
			op: func(ctx context.Context, s *UIServer) (any, error) {
				return s.Prompt(ctx, &apiv1.PromptRequest{Label: "Pass:", Sensitive: true})
			},
			want: "secret",
		},
		{
			name:  "Confirm yes",
			input: "y\n",
			op: func(ctx context.Context, s *UIServer) (any, error) {
				return s.Confirm(ctx, &apiv1.ConfirmRequest{Label: "Yes?"})
			},
			want: true,
		},
		{
			name:  "Confirm no",
			input: "n\n",
			op: func(ctx context.Context, s *UIServer) (any, error) {
				return s.Confirm(ctx, &apiv1.ConfirmRequest{Label: "No?"})
			},
			want: false,
		},
		{
			name:  "Select option",
			input: "2\n",
			op: func(ctx context.Context, s *UIServer) (any, error) {
				return s.Select(ctx, &apiv1.SelectRequest{Label: "Pick:", Options: []string{"A", "B"}})
			},
			want: []uint32{1},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mr := &mockReader{lines: make(chan string, 1), readStarted: make(chan struct{}, 10)}
			srv := NewUIServerWithReader(mr)
			defer srv.Stop()

			mr.lines <- tc.input

			ctx, cancel := context.WithTimeout(t.Context(), 1*time.Second)
			defer cancel()

			res, err := tc.op(ctx, srv)
			if (err != nil) != tc.wantErr {
				t.Fatalf("op() error = %v, wantErr %v", err, tc.wantErr)
			}

			if tc.wantErr {
				return
			}

			switch v := res.(type) {
			case *apiv1.PromptResponse:
				if v.Value != tc.want.(string) {
					t.Errorf("got %q, want %q", v.Value, tc.want)
				}
			case *apiv1.ConfirmResponse:
				if v.Confirmed != tc.want.(bool) {
					t.Errorf("got %v, want %v", v.Confirmed, tc.want)
				}
			case *apiv1.SelectResponse:
				if fmt.Sprint(v.SelectedIndices) != fmt.Sprint(tc.want) {
					t.Errorf("got %v, want %v", v.SelectedIndices, tc.want)
				}
			}
		})
	}
}

func TestUIServer_Coordination(t *testing.T) {
	mr := &mockReader{lines: make(chan string, 2), readStarted: make(chan struct{}, 10)}
	srv := NewUIServerWithReader(mr)
	defer srv.Stop()

	t.Log("Starting first prompt (blocking)")
	promptStarted := make(chan struct{})
	promptDone := make(chan struct{})
	go func() {
		close(promptStarted)
		_, _ = srv.Prompt(t.Context(), &apiv1.PromptRequest{Label: "Blocking:"})
		close(promptDone)
	}()

	// Ensure first prompt has started and theoretically has the lock
	<-promptStarted

	// Wait for the mock reader to confirm it has been invoked
	select {
	case <-mr.readStarted:
		t.Log("First prompt has acquired lock and started reading")
	case <-time.After(1 * time.Second):
		t.Fatal("first prompt failed to start reading")
	}

	t.Log("Starting second prompt (should block)")
	secondDone := make(chan bool, 1)
	go func() {
		_, _ = srv.Confirm(t.Context(), &apiv1.ConfirmRequest{Label: "Second:"})
		secondDone <- true
	}()

	// Ensure second is blocked
	select {
	case <-secondDone:
		t.Fatal("second UI operation did not block")
	case <-time.After(200 * time.Millisecond):
		t.Log("Confirmed second operation is blocked")
	}

	t.Log("Unblocking first prompt")
	mr.lines <- "done\n"

	select {
	case <-promptDone:
		t.Log("First prompt finished")
	case <-time.After(1 * time.Second):
		t.Fatal("first prompt timed out while unblocking")
	}

	t.Log("Unblocking second prompt")
	mr.lines <- "y\n"

	select {
	case <-secondDone:
		t.Log("Second prompt finished")
	case <-time.After(1 * time.Second):
		t.Fatal("second operation never unblocked")
	}
}

func TestUIServer_Cancellation(t *testing.T) {
	mr := &mockReader{lines: make(chan string, 2), readStarted: make(chan struct{}, 10)}
	srv := NewUIServerWithReader(mr)
	defer srv.Stop()

	t.Log("Starting prompt with short timeout")
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	_, err := srv.Prompt(ctx, &apiv1.PromptRequest{Label: "Timed out prompt:"})

	if err == nil {
		t.Error("expected context timeout error, got nil")
	}
	t.Logf("Caught expected error: %v", err)

	t.Log("Satisfying the stale background read")
	mr.lines <- "satisfied\n"

	t.Log("Starting follow-up prompt")
	go func() {
		mr.lines <- "y\n"
	}()

	_, err = srv.Confirm(t.Context(), &apiv1.ConfirmRequest{Label: "Immediate follow-up:"})
	if err != nil {
		t.Errorf("subsequent UI call failed after cancellation: %v", err)
	}
	t.Log("Follow-up prompt succeeded")
}

func TestUIServer_UpdateProgress(t *testing.T) {
	srv := NewUIServer()
	defer srv.Stop()

	t.Log("Sending progress update")
	_, err := srv.UpdateProgress(t.Context(), &apiv1.UpdateProgressRequest{
		Progress: &apiv1.ProgressUpdate{Message: "Testing progress"},
	})
	if err != nil {
		t.Fatalf("UpdateProgress() failed: %v", err)
	}
	t.Log("Progress update finished")
}
