package events

import (
	"testing"
)

func TestRedactMessage(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "no sensitive info",
			input:    "execution started",
			expected: "execution started",
		},
		{
			name:     "bearer token",
			input:    "Authorization: Bearer my-secret-token",
			expected: "Authorization: Bearer [REDACTED]",
		},
		{
			name:     "bearer token lowercase",
			input:    "bearer abc123def456==",
			expected: "Bearer [REDACTED]",
		},
		{
			name:     "token param",
			input:    "https://api.com?token=my-secret&user=jim",
			expected: "https://api.com?token=[REDACTED]&user=jim",
		},
		{
			name:     "api_key param",
			input:    "Failed with api_key=xyz-789",
			expected: "Failed with api_key=[REDACTED]",
		},
		{
			name:     "url credentials",
			input:    "connecting to postgres://admin:password123@localhost:5432/db",
			expected: "connecting to postgres://[REDACTED]@localhost:5432/db",
		},
		{
			name:     "mixed case token",
			input:    "Password=secret-pass",
			expected: "Password=[REDACTED]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RedactMessage(tt.input)
			if got != tt.expected {
				t.Errorf("RedactMessage() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestRedactMetadata(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty metadata",
			input:    "{}",
			expected: "{}",
		},
		{
			name:     "no sensitive keys",
			input:    `{"ticket":"RIG-123","status":"OK"}`,
			expected: `{"ticket":"RIG-123","status":"OK"}`,
		},
		{
			name:     "sensitive key (token)",
			input:    `{"token":"shhh","ticket":"RIG-123"}`,
			expected: `{"ticket":"RIG-123","token":"[REDACTED]"}`,
		},
		{
			name:     "sensitive key (api_key)",
			input:    `{"api_key":"xyz-123"}`,
			expected: `{"api_key":"[REDACTED]"}`,
		},
		{
			name:     "value containing sensitive info",
			input:    `{"error":"failed with Bearer abc-123"}`,
			expected: `{"error":"failed with Bearer [REDACTED]"}`,
		},
		{
			name:     "invalid json fallback",
			input:    "Bearer abc-123",
			expected: "Bearer [REDACTED]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RedactMetadata(tt.input)
			// For JSON comparison, it's safer to check if the content is correct since field order can vary.
			// But for these small tests, simple string comparison should work if we are careful.
			// If it fails due to order, I'll switch to json.Unmarshal.
			if got != tt.expected {
				t.Errorf("RedactMetadata() = %q, want %q", got, tt.expected)
			}
		})
	}
}
