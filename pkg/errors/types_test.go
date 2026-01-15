package errors

import (
	"testing"

	"github.com/cockroachdb/errors"
)

func TestBeadsError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *BeadsError
		expected string
	}{
		{
			name: "with issue ID",
			err: &BeadsError{
				Operation: "update",
				IssueID:   "BEADS-123",
				Message:   "status transition failed",
			},
			expected: "beads update for BEADS-123 failed: status transition failed",
		},
		{
			name: "without issue ID",
			err: &BeadsError{
				Operation: "list",
				Message:   "database connection failed",
			},
			expected: "beads list failed: database connection failed",
		},
		{
			name: "empty message",
			err: &BeadsError{
				Operation: "create",
				IssueID:   "BEADS-456",
				Message:   "",
			},
			expected: "beads create for BEADS-456 failed: ",
		},
		{
			name: "empty operation",
			err: &BeadsError{
				Operation: "",
				Message:   "something went wrong",
			},
			expected: "beads  failed: something went wrong",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.err.Error()
			if result != tt.expected {
				t.Errorf("Error() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestBeadsError_Unwrap(t *testing.T) {
	cause := errors.New("underlying cause")

	tests := []struct {
		name     string
		err      *BeadsError
		hasCause bool
	}{
		{
			name: "with cause",
			err: &BeadsError{
				Operation: "update",
				IssueID:   "BEADS-123",
				Message:   "failed",
				Cause:     cause,
			},
			hasCause: true,
		},
		{
			name: "without cause",
			err: &BeadsError{
				Operation: "update",
				IssueID:   "BEADS-123",
				Message:   "failed",
			},
			hasCause: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unwrapped := tt.err.Unwrap()
			if tt.hasCause {
				if unwrapped != cause {
					t.Errorf("Unwrap() = %v, want %v", unwrapped, cause)
				}
			} else {
				if unwrapped != nil {
					t.Errorf("Unwrap() = %v, want nil", unwrapped)
				}
			}
		})
	}
}

func TestBeadsError_ErrorsAs(t *testing.T) {
	beadsErr := &BeadsError{
		Operation: "close",
		IssueID:   "BEADS-789",
		Message:   "cannot close blocked issue",
	}

	// Wrap the error to test errors.As traversal
	wrappedErr := errors.Wrap(beadsErr, "operation failed")

	var target *BeadsError
	if !errors.As(wrappedErr, &target) {
		t.Error("errors.As() should find BeadsError in wrapped error chain")
	}

	if target.Operation != "close" {
		t.Errorf("Operation = %q, want %q", target.Operation, "close")
	}
	if target.IssueID != "BEADS-789" {
		t.Errorf("IssueID = %q, want %q", target.IssueID, "BEADS-789")
	}
}

func TestBeadsError_ErrorsIs(t *testing.T) {
	sentinelErr := errors.New("sentinel error")
	beadsErr := &BeadsError{
		Operation: "update",
		Message:   "failed",
		Cause:     sentinelErr,
	}

	// errors.Is should find the sentinel in the chain
	if !errors.Is(beadsErr, sentinelErr) {
		t.Error("errors.Is() should find sentinel error through Unwrap chain")
	}
}

func TestNewBeadsError(t *testing.T) {
	tests := []struct {
		name      string
		operation string
		message   string
	}{
		{
			name:      "typical error",
			operation: "create",
			message:   "invalid title",
		},
		{
			name:      "empty operation",
			operation: "",
			message:   "something wrong",
		},
		{
			name:      "empty message",
			operation: "list",
			message:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewBeadsError(tt.operation, tt.message)

			if err.Operation != tt.operation {
				t.Errorf("Operation = %q, want %q", err.Operation, tt.operation)
			}
			if err.Message != tt.message {
				t.Errorf("Message = %q, want %q", err.Message, tt.message)
			}
			if err.IssueID != "" {
				t.Errorf("IssueID = %q, want empty", err.IssueID)
			}
			if err.Retryable {
				t.Error("Retryable should be false")
			}
			if err.Cause != nil {
				t.Errorf("Cause = %v, want nil", err.Cause)
			}
		})
	}
}

func TestNewBeadsErrorWithIssue(t *testing.T) {
	tests := []struct {
		name      string
		operation string
		issueID   string
		message   string
	}{
		{
			name:      "typical error with issue",
			operation: "update",
			issueID:   "BEADS-123",
			message:   "status invalid",
		},
		{
			name:      "empty issue ID",
			operation: "close",
			issueID:   "",
			message:   "not found",
		},
		{
			name:      "all fields populated",
			operation: "transition",
			issueID:   "BEADS-456",
			message:   "blocked by dependency",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewBeadsErrorWithIssue(tt.operation, tt.issueID, tt.message)

			if err.Operation != tt.operation {
				t.Errorf("Operation = %q, want %q", err.Operation, tt.operation)
			}
			if err.IssueID != tt.issueID {
				t.Errorf("IssueID = %q, want %q", err.IssueID, tt.issueID)
			}
			if err.Message != tt.message {
				t.Errorf("Message = %q, want %q", err.Message, tt.message)
			}
			if err.Retryable {
				t.Error("Retryable should be false")
			}
			if err.Cause != nil {
				t.Errorf("Cause = %v, want nil", err.Cause)
			}
		})
	}
}

func TestNewBeadsErrorWithCause(t *testing.T) {
	retryableCause := &GitHubError{
		Operation: "api",
		Message:   "rate limited",
		Retryable: true,
	}
	nonRetryableCause := &GitHubError{
		Operation: "api",
		Message:   "not found",
		Retryable: false,
	}
	plainCause := errors.New("plain error")

	tests := []struct {
		name              string
		operation         string
		issueID           string
		message           string
		cause             error
		expectedRetryable bool
	}{
		{
			name:              "retryable cause",
			operation:         "sync",
			issueID:           "BEADS-123",
			message:           "sync failed",
			cause:             retryableCause,
			expectedRetryable: true,
		},
		{
			name:              "non-retryable cause",
			operation:         "fetch",
			issueID:           "BEADS-456",
			message:           "fetch failed",
			cause:             nonRetryableCause,
			expectedRetryable: false,
		},
		{
			name:              "plain error cause",
			operation:         "update",
			issueID:           "",
			message:           "update failed",
			cause:             plainCause,
			expectedRetryable: false,
		},
		{
			name:              "nil cause",
			operation:         "create",
			issueID:           "BEADS-789",
			message:           "create failed",
			cause:             nil,
			expectedRetryable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewBeadsErrorWithCause(tt.operation, tt.issueID, tt.message, tt.cause)

			if err.Operation != tt.operation {
				t.Errorf("Operation = %q, want %q", err.Operation, tt.operation)
			}
			if err.IssueID != tt.issueID {
				t.Errorf("IssueID = %q, want %q", err.IssueID, tt.issueID)
			}
			if err.Message != tt.message {
				t.Errorf("Message = %q, want %q", err.Message, tt.message)
			}
			if err.Retryable != tt.expectedRetryable {
				t.Errorf("Retryable = %v, want %v", err.Retryable, tt.expectedRetryable)
			}
			if err.Cause != tt.cause {
				t.Errorf("Cause = %v, want %v", err.Cause, tt.cause)
			}
		})
	}
}

func TestNewBeadsErrorWithCause_PreservesCauseForUnwrapping(t *testing.T) {
	originalCause := errors.New("original cause")
	err := NewBeadsErrorWithCause("update", "BEADS-123", "failed", originalCause)

	// Verify we can unwrap to get the original cause
	unwrapped := err.Unwrap()
	if unwrapped != originalCause {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, originalCause)
	}

	// Verify errors.Is works through the chain
	if !errors.Is(err, originalCause) {
		t.Error("errors.Is() should find original cause through unwrap chain")
	}
}

func TestIsBeadsError(t *testing.T) {
	beadsErr := NewBeadsError("test", "test message")
	wrappedBeadsErr := errors.Wrap(beadsErr, "wrapped")
	doubleWrappedBeadsErr := errors.Wrap(wrappedBeadsErr, "double wrapped")

	configErr := NewConfigError("field", "message")
	githubErr := NewGitHubError("operation", "message")
	jiraErr := NewJiraError("operation", "message")
	plainErr := errors.New("plain error")

	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "direct BeadsError",
			err:      beadsErr,
			expected: true,
		},
		{
			name:     "wrapped BeadsError",
			err:      wrappedBeadsErr,
			expected: true,
		},
		{
			name:     "double wrapped BeadsError",
			err:      doubleWrappedBeadsErr,
			expected: true,
		},
		{
			name:     "ConfigError",
			err:      configErr,
			expected: false,
		},
		{
			name:     "GitHubError",
			err:      githubErr,
			expected: false,
		},
		{
			name:     "JiraError",
			err:      jiraErr,
			expected: false,
		},
		{
			name:     "plain error",
			err:      plainErr,
			expected: false,
		},
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsBeadsError(tt.err)
			if result != tt.expected {
				t.Errorf("IsBeadsError() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestIsRetryable_BeadsError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name: "retryable BeadsError",
			err: &BeadsError{
				Operation: "sync",
				Message:   "timeout",
				Retryable: true,
			},
			expected: true,
		},
		{
			name: "non-retryable BeadsError",
			err: &BeadsError{
				Operation: "create",
				Message:   "invalid data",
				Retryable: false,
			},
			expected: false,
		},
		{
			name: "wrapped retryable BeadsError",
			err: errors.Wrap(&BeadsError{
				Operation: "update",
				Message:   "network error",
				Retryable: true,
			}, "operation failed"),
			expected: true,
		},
		{
			name: "wrapped non-retryable BeadsError",
			err: errors.Wrap(&BeadsError{
				Operation: "delete",
				Message:   "permission denied",
				Retryable: false,
			}, "operation failed"),
			expected: false,
		},
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRetryable(tt.err)
			if result != tt.expected {
				t.Errorf("IsRetryable() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestIsRetryable_BeadsErrorWithRetryableCause(t *testing.T) {
	// Create a retryable AIError as the cause
	aiErr := &AIError{
		Provider:  "anthropic",
		Operation: "chat",
		Message:   "rate limited",
		Retryable: true,
	}

	// Create BeadsError with that cause using the constructor
	// The constructor should set Retryable based on the cause
	beadsErr := NewBeadsErrorWithCause("sync", "BEADS-123", "AI call failed", aiErr)

	if !beadsErr.Retryable {
		t.Error("BeadsError should be retryable when cause is retryable")
	}

	if !IsRetryable(beadsErr) {
		t.Error("IsRetryable() should return true for BeadsError with retryable flag set")
	}
}
