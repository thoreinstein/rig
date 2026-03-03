package events

import (
	"encoding/json"
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
			name:     "access_token param",
			input:    "url?access_token=secret123",
			expected: "url?access_token=[REDACTED]",
		},
		{
			name:     "refresh_token param",
			input:    "refresh_token=abc&type=bearer",
			expected: "refresh_token=[REDACTED]&type=bearer",
		},
		{
			name:     "session_token param",
			input:    "session_token=xyz",
			expected: "session_token=[REDACTED]",
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
		{ //nolint:gosec // Fake credential for redaction testing.
			name:     "mongodb+srv compound scheme",
			input:    "mongodb+srv://user:pass@cluster.mongodb.net",
			expected: "mongodb+srv://[REDACTED]@cluster.mongodb.net",
		},
		{
			name:     "basic auth header",
			input:    "Authorization: Basic dXNlcjpwYXNz",
			expected: "Authorization: Basic [REDACTED]",
		},
		{ //nolint:gosec // Fake credential for redaction testing.
			name:     "AWS access key",
			input:    "key is AKIAIOSFODNN7EXAMPLE",
			expected: "key is [REDACTED]",
		},
		{ //nolint:gosec // Fake credential for redaction testing.
			name:     "GitHub personal access token",
			input:    "token: ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghij",
			expected: "token: [REDACTED]",
		},
		{
			name:     "GitHub fine-grained PAT",
			input:    "github_pat_ABCDEFGHIJ1234567890ab_extra_chars_here",
			expected: "[REDACTED]",
		},
		{
			name:     "Anthropic API key",
			input:    "sk-ant-api03-abcdefghijklmnopqrst",
			expected: "[REDACTED]",
		},
		{
			name:     "OpenAI project API key",
			input:    "sk-proj-abcdefghijklmnopqrstu",
			expected: "[REDACTED]",
		},
		{ //nolint:gosec // Fake credential for redaction testing.
			name:     "PEM private key block",
			input:    "-----BEGIN RSA PRIVATE KEY-----\nMIIBogIBAAJ...\n-----END RSA PRIVATE KEY-----",
			expected: "[REDACTED]",
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
		// useJSON enables order-independent JSON comparison
		useJSON bool
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
			name:    "sensitive key (token)",
			input:   `{"token":"shhh","ticket":"RIG-123"}`,
			useJSON: true,
			expected: func() string {
				m := map[string]any{"ticket": "RIG-123", "token": "[REDACTED]"}
				b, _ := json.Marshal(m)
				return string(b)
			}(),
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
		{
			name:    "nested object with sensitive key",
			input:   `{"config":{"password":"s3cret","host":"localhost"}}`,
			useJSON: true,
			expected: func() string {
				m := map[string]any{
					"config": map[string]any{
						"password": "[REDACTED]",
						"host":     "localhost",
					},
				}
				b, _ := json.Marshal(m)
				return string(b)
			}(),
		},
		{
			name:    "nested object with sensitive value pattern",
			input:   `{"inner":{"msg":"token=abc123"}}`,
			useJSON: true,
			expected: func() string {
				m := map[string]any{
					"inner": map[string]any{
						"msg": "token=[REDACTED]",
					},
				}
				b, _ := json.Marshal(m)
				return string(b)
			}(),
		},
		{
			name:     "array with sensitive strings",
			input:    `{"args":["safe","Bearer secret-tok"]}`,
			expected: `{"args":["safe","Bearer [REDACTED]"]}`,
		},
		{
			name:    "deeply nested array and object",
			input:   `{"outer":[{"api_key":"hidden"}]}`,
			useJSON: true,
			expected: func() string {
				m := map[string]any{
					"outer": []any{
						map[string]any{"api_key": "[REDACTED]"},
					},
				}
				b, _ := json.Marshal(m)
				return string(b)
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RedactMetadata(tt.input)
			if tt.useJSON {
				assertJSONEqual(t, got, tt.expected)
			} else if got != tt.expected {
				t.Errorf("RedactMetadata() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// assertJSONEqual compares two JSON strings for semantic equality,
// ignoring key ordering.
func assertJSONEqual(t *testing.T, got, want string) {
	t.Helper()
	var g, w any
	if err := json.Unmarshal([]byte(got), &g); err != nil {
		t.Fatalf("failed to parse got JSON: %v\nraw: %s", err, got)
	}
	if err := json.Unmarshal([]byte(want), &w); err != nil {
		t.Fatalf("failed to parse want JSON: %v\nraw: %s", err, want)
	}
	gotBytes, _ := json.Marshal(g)
	wantBytes, _ := json.Marshal(w)
	if string(gotBytes) != string(wantBytes) {
		t.Errorf("RedactMetadata() JSON mismatch\ngot:  %s\nwant: %s", gotBytes, wantBytes)
	}
}
