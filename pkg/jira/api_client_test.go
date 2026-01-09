package jira

// This test file provides comprehensive integration-style testing for the Jira API client
// using Go's standard httptest.Server pattern. The tests cover:
//
//   - Full end-to-end request/response cycles with realistic Jira API responses
//   - Multiple sequential requests via rate limit retry logic
//   - Error handling and recovery scenarios (HTTP errors, invalid JSON, missing fields)
//   - Request validation (method, path, headers, authentication)
//   - ADF document parsing and custom field extraction
//
// These tests effectively serve as integration tests without requiring a live Jira instance,
// following Go's idiomatic approach for HTTP client testing.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"thoreinstein.com/rig/pkg/config"
)

func TestNewAPIClient(t *testing.T) {
	cfg := &config.JiraConfig{
		BaseURL: "https://example.atlassian.net",
		Email:   "test@example.com",
		Token:   "test-token",
	}

	client, err := NewAPIClient(cfg, false)
	if err != nil {
		t.Fatalf("NewAPIClient() error = %v, want nil", err)
	}

	if client.baseURL != "https://example.atlassian.net" {
		t.Errorf("baseURL = %q, want %q", client.baseURL, "https://example.atlassian.net")
	}
	if client.email != "test@example.com" {
		t.Errorf("email = %q, want %q", client.email, "test@example.com")
	}
	if client.token != "test-token" {
		t.Errorf("token = %q, want %q", client.token, "test-token")
	}
}

func TestNewAPIClient_TrimsTrailingSlash(t *testing.T) {
	cfg := &config.JiraConfig{
		BaseURL: "https://example.atlassian.net/",
		Email:   "test@example.com",
		Token:   "test-token",
	}

	client, err := NewAPIClient(cfg, false)
	if err != nil {
		t.Fatalf("NewAPIClient() error = %v, want nil", err)
	}

	if client.baseURL != "https://example.atlassian.net" {
		t.Errorf("baseURL = %q, want %q (trailing slash should be trimmed)", client.baseURL, "https://example.atlassian.net")
	}
}

func TestNewAPIClient_EnvVarPrecedence(t *testing.T) {
	// Set env var using t.Setenv which automatically cleans up
	t.Setenv("JIRA_TOKEN", "env-token")

	cfg := &config.JiraConfig{
		BaseURL: "https://example.atlassian.net",
		Email:   "test@example.com",
		Token:   "config-token",
	}

	client, err := NewAPIClient(cfg, false)
	if err != nil {
		t.Fatalf("NewAPIClient() error = %v, want nil", err)
	}

	// Env var should take precedence
	if client.token != "env-token" {
		t.Errorf("token = %q, want %q (env var should take precedence)", client.token, "env-token")
	}
}

func TestNewAPIClient_EnvVarFallback(t *testing.T) {
	// Set env var when config token is empty
	t.Setenv("JIRA_TOKEN", "env-token")

	cfg := &config.JiraConfig{
		BaseURL: "https://example.atlassian.net",
		Email:   "test@example.com",
		Token:   "", // Empty config token
	}

	client, err := NewAPIClient(cfg, false)
	if err != nil {
		t.Fatalf("NewAPIClient() error = %v, want nil", err)
	}

	// Env var should be used when config is empty
	if client.token != "env-token" {
		t.Errorf("token = %q, want %q (env var should be used when config empty)", client.token, "env-token")
	}
}

func TestNewAPIClient_MissingFields(t *testing.T) {
	tests := []struct {
		name       string
		cfg        *config.JiraConfig
		wantErrMsg string // substring to look for in error
	}{
		{
			name: "missing base_url",
			cfg: &config.JiraConfig{
				Email: "test@example.com",
				Token: "test-token",
			},
			wantErrMsg: "base_url is required",
		},
		{
			name: "missing email",
			cfg: &config.JiraConfig{
				BaseURL: "https://example.atlassian.net",
				Token:   "test-token",
			},
			wantErrMsg: "email is required",
		},
		{
			name: "missing token",
			cfg: &config.JiraConfig{
				BaseURL: "https://example.atlassian.net",
				Email:   "test@example.com",
			},
			wantErrMsg: "token is required",
		},
	}

	// Ensure JIRA_TOKEN env var is not set
	os.Unsetenv("JIRA_TOKEN")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewAPIClient(tt.cfg, false)
			if err == nil {
				t.Errorf("NewAPIClient() should return error for %s", tt.name)
				return
			}
			if !contains(err.Error(), tt.wantErrMsg) {
				t.Errorf("error = %q, should contain %q", err.Error(), tt.wantErrMsg)
			}
		})
	}
}

func TestAPIClient_IsAvailable(t *testing.T) {
	cfg := &config.JiraConfig{
		BaseURL: "https://example.atlassian.net",
		Email:   "test@example.com",
		Token:   "test-token",
	}

	client, err := NewAPIClient(cfg, false)
	if err != nil {
		t.Fatalf("NewAPIClient() error = %v, want nil", err)
	}

	if !client.IsAvailable() {
		t.Error("IsAvailable() should return true when all fields are set")
	}
}

func TestAPIClient_FetchTicketDetails_Success(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != http.MethodGet {
			t.Errorf("Expected GET request, got %s", r.Method)
		}
		if r.URL.Path != "/rest/api/3/issue/TEST-123" {
			t.Errorf("Expected path /rest/api/3/issue/TEST-123, got %s", r.URL.Path)
		}
		if r.Header.Get("Accept") != "application/json" {
			t.Errorf("Expected Accept header 'application/json', got %s", r.Header.Get("Accept"))
		}
		if r.Header.Get("Authorization") == "" {
			t.Error("Expected Authorization header to be set")
		}

		// Return mock response
		response := map[string]interface{}{
			"fields": map[string]interface{}{
				"issuetype": map[string]string{"name": "Bug"},
				"summary":   "Fix login issue",
				"status":    map[string]string{"name": "In Progress"},
				"priority":  map[string]string{"name": "High"},
				"description": map[string]interface{}{
					"type": "doc",
					"content": []map[string]interface{}{
						{
							"type": "paragraph",
							"content": []map[string]interface{}{
								{"type": "text", "text": "Description here"},
							},
						},
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	cfg := &config.JiraConfig{
		BaseURL: server.URL,
		Email:   "test@example.com",
		Token:   "test-token",
	}

	client, err := NewAPIClient(cfg, false)
	if err != nil {
		t.Fatalf("NewAPIClient() error = %v, want nil", err)
	}

	info, err := client.FetchTicketDetails("TEST-123")
	if err != nil {
		t.Fatalf("FetchTicketDetails() error = %v, want nil", err)
	}

	if info.Type != "Bug" {
		t.Errorf("Type = %q, want %q", info.Type, "Bug")
	}
	if info.Summary != "Fix login issue" {
		t.Errorf("Summary = %q, want %q", info.Summary, "Fix login issue")
	}
	if info.Status != "In Progress" {
		t.Errorf("Status = %q, want %q", info.Status, "In Progress")
	}
	if info.Priority != "High" {
		t.Errorf("Priority = %q, want %q", info.Priority, "High")
	}
	if info.Description != "Description here" {
		t.Errorf("Description = %q, want %q", info.Description, "Description here")
	}
}

func TestAPIClient_FetchTicketDetails_NullDescription(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"fields": map[string]interface{}{
				"issuetype":   map[string]string{"name": "Task"},
				"summary":     "Simple task",
				"status":      map[string]string{"name": "Open"},
				"priority":    map[string]string{"name": "Medium"},
				"description": nil,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	cfg := &config.JiraConfig{
		BaseURL: server.URL,
		Email:   "test@example.com",
		Token:   "test-token",
	}

	client, err := NewAPIClient(cfg, false)
	if err != nil {
		t.Fatalf("NewAPIClient() error = %v, want nil", err)
	}

	info, err := client.FetchTicketDetails("TEST-456")
	if err != nil {
		t.Fatalf("FetchTicketDetails() error = %v, want nil", err)
	}

	if info.Description != "" {
		t.Errorf("Description = %q, want empty", info.Description)
	}
}

func TestAPIClient_FetchTicketDetails_HTTPErrors(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantErr    string
	}{
		{
			name:       "401 unauthorized",
			statusCode: http.StatusUnauthorized,
			body:       `{"errorMessages":["Unauthorized"]}`,
			wantErr:    "authentication failed",
		},
		{
			name:       "403 forbidden",
			statusCode: http.StatusForbidden,
			body:       `{"errorMessages":["Forbidden"]}`,
			wantErr:    "access denied",
		},
		{
			name:       "404 not found",
			statusCode: http.StatusNotFound,
			body:       `{"errorMessages":["Issue does not exist"]}`,
			wantErr:    "not found",
		},
		// Note: 429 rate limit is tested separately in TestAPIClient_FetchTicketDetails_RateLimit*
		// tests since it now involves retry logic with exponential backoff
		{
			name:       "500 server error",
			statusCode: http.StatusInternalServerError,
			body:       `{"errorMessages":["Internal server error"]}`,
			wantErr:    "HTTP 500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			cfg := &config.JiraConfig{
				BaseURL: server.URL,
				Email:   "test@example.com",
				Token:   "test-token",
			}

			client, err := NewAPIClient(cfg, false)
			if err != nil {
				t.Fatalf("NewAPIClient() error = %v, want nil", err)
			}

			_, err = client.FetchTicketDetails("TEST-123")
			if err == nil {
				t.Fatal("FetchTicketDetails() should return error")
			}

			if !contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, should contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestExtractADFText_SimpleDocument(t *testing.T) {
	doc := &jiraADFDocument{
		Type: "doc",
		Content: []jiraADFContent{
			{
				Type: "paragraph",
				Content: []jiraADFContent{
					{Type: "text", Text: "Hello world"},
				},
			},
		},
	}

	result := extractADFText(doc)
	if result != "Hello world" {
		t.Errorf("extractADFText() = %q, want %q", result, "Hello world")
	}
}

func TestExtractADFText_MultipleParagraphs(t *testing.T) {
	doc := &jiraADFDocument{
		Type: "doc",
		Content: []jiraADFContent{
			{
				Type: "paragraph",
				Content: []jiraADFContent{
					{Type: "text", Text: "First paragraph"},
				},
			},
			{
				Type: "paragraph",
				Content: []jiraADFContent{
					{Type: "text", Text: "Second paragraph"},
				},
			},
		},
	}

	result := extractADFText(doc)
	expected := "First paragraph\nSecond paragraph"
	if result != expected {
		t.Errorf("extractADFText() = %q, want %q", result, expected)
	}
}

func TestExtractADFText_BulletList(t *testing.T) {
	doc := &jiraADFDocument{
		Type: "doc",
		Content: []jiraADFContent{
			{
				Type: "bulletList",
				Content: []jiraADFContent{
					{
						Type: "listItem",
						Content: []jiraADFContent{
							{
								Type: "paragraph",
								Content: []jiraADFContent{
									{Type: "text", Text: "Item 1"},
								},
							},
						},
					},
					{
						Type: "listItem",
						Content: []jiraADFContent{
							{
								Type: "paragraph",
								Content: []jiraADFContent{
									{Type: "text", Text: "Item 2"},
								},
							},
						},
					},
				},
			},
		},
	}

	result := extractADFText(doc)
	expected := "Item 1\nItem 2"
	if result != expected {
		t.Errorf("extractADFText() = %q, want %q", result, expected)
	}
}

func TestExtractADFText_NilDocument(t *testing.T) {
	result := extractADFText(nil)
	if result != "" {
		t.Errorf("extractADFText(nil) = %q, want empty", result)
	}
}

func TestExtractADFText_OrderedList(t *testing.T) {
	doc := &jiraADFDocument{
		Type: "doc",
		Content: []jiraADFContent{
			{
				Type: "orderedList",
				Content: []jiraADFContent{
					{
						Type: "listItem",
						Content: []jiraADFContent{
							{
								Type: "paragraph",
								Content: []jiraADFContent{
									{Type: "text", Text: "First item"},
								},
							},
						},
					},
					{
						Type: "listItem",
						Content: []jiraADFContent{
							{
								Type: "paragraph",
								Content: []jiraADFContent{
									{Type: "text", Text: "Second item"},
								},
							},
						},
					},
				},
			},
		},
	}

	result := extractADFText(doc)
	expected := "First item\nSecond item"
	if result != expected {
		t.Errorf("extractADFText() = %q, want %q", result, expected)
	}
}

func TestExtractADFText_Heading(t *testing.T) {
	doc := &jiraADFDocument{
		Type: "doc",
		Content: []jiraADFContent{
			{
				Type: "heading",
				Content: []jiraADFContent{
					{Type: "text", Text: "Main Heading"},
				},
			},
			{
				Type: "paragraph",
				Content: []jiraADFContent{
					{Type: "text", Text: "Content below heading"},
				},
			},
		},
	}

	result := extractADFText(doc)
	expected := "Main Heading\nContent below heading"
	if result != expected {
		t.Errorf("extractADFText() = %q, want %q", result, expected)
	}
}

func TestExtractADFText_MixedContent(t *testing.T) {
	doc := &jiraADFDocument{
		Type: "doc",
		Content: []jiraADFContent{
			{
				Type: "paragraph",
				Content: []jiraADFContent{
					{Type: "text", Text: "Bold and "},
					{Type: "text", Text: "normal text"},
				},
			},
		},
	}

	result := extractADFText(doc)
	expected := "Bold and normal text"
	if result != expected {
		t.Errorf("extractADFText() = %q, want %q", result, expected)
	}
}

func TestExtractADFText_UnknownContentType(t *testing.T) {
	// Test with an unknown content type that has children
	doc := &jiraADFDocument{
		Type: "doc",
		Content: []jiraADFContent{
			{
				Type: "unknownType",
				Content: []jiraADFContent{
					{Type: "text", Text: "Text inside unknown type"},
				},
			},
		},
	}

	result := extractADFText(doc)
	expected := "Text inside unknown type"
	if result != expected {
		t.Errorf("extractADFText() = %q, want %q", result, expected)
	}
}

func TestExtractADFContentText_NilContent(t *testing.T) {
	result := extractADFContentText(nil)
	if result != "" {
		t.Errorf("extractADFContentText(nil) = %q, want empty", result)
	}
}

func TestExtractADFText_EmptyDocument(t *testing.T) {
	doc := &jiraADFDocument{
		Type:    "doc",
		Content: []jiraADFContent{},
	}

	result := extractADFText(doc)
	if result != "" {
		t.Errorf("extractADFText() = %q, want empty", result)
	}
}

func TestNewJiraClient_APIMode(t *testing.T) {
	cfg := &config.JiraConfig{
		Mode:    "api",
		BaseURL: "https://example.atlassian.net",
		Email:   "test@example.com",
		Token:   "test-token",
	}

	client, err := NewJiraClient(cfg, false)
	if err != nil {
		t.Fatalf("NewJiraClient() error = %v, want nil", err)
	}

	_, ok := client.(*APIClient)
	if !ok {
		t.Errorf("NewJiraClient(mode=api) should return *APIClient, got %T", client)
	}
}

func TestNewJiraClient_CLIMode(t *testing.T) {
	cfg := &config.JiraConfig{
		Mode:       "acli",
		CliCommand: "acli",
	}

	client, err := NewJiraClient(cfg, false)
	if err != nil {
		t.Fatalf("NewJiraClient() error = %v, want nil", err)
	}

	_, ok := client.(*CLIClient)
	if !ok {
		t.Errorf("NewJiraClient(mode=acli) should return *CLIClient, got %T", client)
	}
}

func TestNewJiraClient_DefaultMode(t *testing.T) {
	cfg := &config.JiraConfig{
		Mode:       "",
		CliCommand: "acli",
	}

	client, err := NewJiraClient(cfg, false)
	if err != nil {
		t.Fatalf("NewJiraClient() error = %v, want nil", err)
	}

	_, ok := client.(*CLIClient)
	if !ok {
		t.Errorf("NewJiraClient(mode='') should return *CLIClient, got %T", client)
	}
}

func TestNewJiraClient_UnknownMode(t *testing.T) {
	cfg := &config.JiraConfig{
		Mode: "unknown",
	}

	_, err := NewJiraClient(cfg, false)
	if err == nil {
		t.Fatal("NewJiraClient() should return error for unknown mode")
	}

	if !contains(err.Error(), "unknown jira mode") {
		t.Errorf("error = %q, should contain 'unknown jira mode'", err.Error())
	}
}

func TestNewJiraClient_NilConfig(t *testing.T) {
	_, err := NewJiraClient(nil, false)
	if err == nil {
		t.Fatal("NewJiraClient(nil) should return error")
	}
}

// contains checks if s contains substr
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestCalculateBackoff(t *testing.T) {
	tests := []struct {
		name    string
		base    time.Duration
		max     time.Duration
		attempt int
		wantMin time.Duration
		wantMax time.Duration
	}{
		{
			name:    "attempt 0",
			base:    time.Second,
			max:     30 * time.Second,
			attempt: 0,
			wantMin: 800 * time.Millisecond,  // 1s * 2^0 * 0.8
			wantMax: 1200 * time.Millisecond, // 1s * 2^0 * 1.2
		},
		{
			name:    "attempt 1",
			base:    time.Second,
			max:     30 * time.Second,
			attempt: 1,
			wantMin: 1600 * time.Millisecond, // 1s * 2^1 * 0.8
			wantMax: 2400 * time.Millisecond, // 1s * 2^1 * 1.2
		},
		{
			name:    "attempt 2",
			base:    time.Second,
			max:     30 * time.Second,
			attempt: 2,
			wantMin: 3200 * time.Millisecond, // 1s * 2^2 * 0.8
			wantMax: 4800 * time.Millisecond, // 1s * 2^2 * 1.2
		},
		{
			name:    "capped at max",
			base:    time.Second,
			max:     2 * time.Second,
			attempt: 5,
			wantMin: 1600 * time.Millisecond, // max (2s) * 0.8
			wantMax: 2400 * time.Millisecond, // max (2s) * 1.2
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Run multiple times to account for random jitter
			for range 100 {
				got := calculateBackoff(tt.base, tt.max, tt.attempt)
				if got < tt.wantMin || got > tt.wantMax {
					t.Errorf("calculateBackoff() = %v, want between %v and %v",
						got, tt.wantMin, tt.wantMax)
				}
			}
		})
	}
}

func TestParseRetryAfter(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   time.Duration
	}{
		{
			name:   "empty string",
			header: "",
			want:   0,
		},
		{
			name:   "integer seconds",
			header: "5",
			want:   5 * time.Second,
		},
		{
			name:   "zero seconds",
			header: "0",
			want:   0,
		},
		{
			name:   "invalid string",
			header: "invalid",
			want:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseRetryAfter(tt.header)
			if got != tt.want {
				t.Errorf("parseRetryAfter(%q) = %v, want %v", tt.header, got, tt.want)
			}
		})
	}
}

func TestParseRetryAfter_RFC1123Date(t *testing.T) {
	// Create a time 5 seconds in the future
	futureTime := time.Now().Add(5 * time.Second)
	header := futureTime.Format(time.RFC1123)

	got := parseRetryAfter(header)

	// Should return approximately 5 seconds (allow margin for test execution time)
	if got < 4*time.Second || got > 6*time.Second {
		t.Errorf("parseRetryAfter(%q) = %v, want approximately 5s", header, got)
	}
}

func TestParseRetryAfter_RFC1123DateInPast(t *testing.T) {
	// Create a time in the past
	pastTime := time.Now().Add(-5 * time.Second)
	header := pastTime.Format(time.RFC1123)

	got := parseRetryAfter(header)

	// Past dates should return 0
	if got != 0 {
		t.Errorf("parseRetryAfter(%q) = %v, want 0 for past date", header, got)
	}
}

func TestAPIClient_FetchTicketDetails_RateLimitRetrySuccess(t *testing.T) {
	requestCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		// Return 429 for first two requests, then succeed
		if requestCount <= 2 {
			w.Header().Set("Retry-After", "0") // Use 0 to avoid actual delay in tests
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}

		// Return success on third request
		response := map[string]interface{}{
			"fields": map[string]interface{}{
				"issuetype": map[string]string{"name": "Task"},
				"summary":   "Test task after retry",
				"status":    map[string]string{"name": "Open"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	cfg := &config.JiraConfig{
		BaseURL: server.URL,
		Email:   "test@example.com",
		Token:   "test-token",
	}

	client, err := NewAPIClient(cfg, false)
	if err != nil {
		t.Fatalf("NewAPIClient() error = %v, want nil", err)
	}

	info, err := client.FetchTicketDetails("TEST-123")
	if err != nil {
		t.Fatalf("FetchTicketDetails() error = %v, want nil", err)
	}

	if info.Summary != "Test task after retry" {
		t.Errorf("Summary = %q, want %q", info.Summary, "Test task after retry")
	}

	if requestCount != 3 {
		t.Errorf("Request count = %d, want 3", requestCount)
	}
}

func TestAPIClient_FetchTicketDetails_RateLimitExhausted(t *testing.T) {
	requestCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Retry-After", "0") // Use 0 to avoid actual delay in tests
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	cfg := &config.JiraConfig{
		BaseURL: server.URL,
		Email:   "test@example.com",
		Token:   "test-token",
	}

	client, err := NewAPIClient(cfg, false)
	if err != nil {
		t.Fatalf("NewAPIClient() error = %v, want nil", err)
	}

	_, err = client.FetchTicketDetails("TEST-123")
	if err == nil {
		t.Fatal("FetchTicketDetails() should return error after exhausting retries")
	}

	if !contains(err.Error(), "rate limited after") {
		t.Errorf("error = %q, should contain 'rate limited after'", err.Error())
	}

	// Should have made maxRetries + 1 requests (initial + retries)
	expectedRequests := 4 // 1 initial + 3 retries
	if requestCount != expectedRequests {
		t.Errorf("Request count = %d, want %d", requestCount, expectedRequests)
	}
}

func TestAPIClient_FetchTicketDetails_RespectsRetryAfterHeader(t *testing.T) {
	requestCount := 0
	var requestTimes []time.Time

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		requestTimes = append(requestTimes, time.Now())

		// Return 429 with Retry-After header on first request
		if requestCount == 1 {
			w.Header().Set("Retry-After", "1") // 1 second
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}

		// Return success on second request
		response := map[string]interface{}{
			"fields": map[string]interface{}{
				"issuetype": map[string]string{"name": "Task"},
				"summary":   "Test task",
				"status":    map[string]string{"name": "Open"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	cfg := &config.JiraConfig{
		BaseURL: server.URL,
		Email:   "test@example.com",
		Token:   "test-token",
	}

	client, err := NewAPIClient(cfg, false)
	if err != nil {
		t.Fatalf("NewAPIClient() error = %v, want nil", err)
	}

	_, err = client.FetchTicketDetails("TEST-123")
	if err != nil {
		t.Fatalf("FetchTicketDetails() error = %v, want nil", err)
	}

	if requestCount != 2 {
		t.Errorf("Request count = %d, want 2", requestCount)
	}

	// Verify that the delay was at least close to 1 second (allowing some margin)
	if len(requestTimes) >= 2 {
		delay := requestTimes[1].Sub(requestTimes[0])
		minExpectedDelay := 900 * time.Millisecond // Allow 100ms margin
		if delay < minExpectedDelay {
			t.Errorf("Delay between requests = %v, want at least %v", delay, minExpectedDelay)
		}
	}
}

func TestExtractCustomFieldValue(t *testing.T) {
	tests := []struct {
		name string
		json string
		want string
	}{
		{
			name: "string value",
			json: `"hello world"`,
			want: "hello world",
		},
		{
			name: "integer number",
			json: `42`,
			want: "42",
		},
		{
			name: "float number",
			json: `3.14`,
			want: "3.14",
		},
		{
			name: "object with value field",
			json: `{"value": "option1"}`,
			want: "option1",
		},
		{
			name: "object with name field",
			json: `{"name": "Team Alpha"}`,
			want: "Team Alpha",
		},
		{
			name: "object with both value and name prefers value",
			json: `{"value": "val", "name": "nm"}`,
			want: "val",
		},
		{
			name: "array of objects with value",
			json: `[{"value": "a"}, {"value": "b"}, {"value": "c"}]`,
			want: "a, b, c",
		},
		{
			name: "array of objects with name",
			json: `[{"name": "x"}, {"name": "y"}]`,
			want: "x, y",
		},
		{
			name: "array of strings",
			json: `["one", "two", "three"]`,
			want: "one, two, three",
		},
		{
			name: "null value",
			json: `null`,
			want: "",
		},
		{
			name: "empty object",
			json: `{}`,
			want: "",
		},
		{
			name: "empty array",
			json: `[]`,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractCustomFieldValue(json.RawMessage(tt.json))
			if got != tt.want {
				t.Errorf("extractCustomFieldValue() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAPIClient_FetchTicketDetails_WithCustomFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"fields": map[string]interface{}{
				"issuetype":         map[string]string{"name": "Story"},
				"summary":           "Test story with custom fields",
				"status":            map[string]string{"name": "In Progress"},
				"priority":          map[string]string{"name": "Medium"},
				"description":       nil,
				"customfield_10016": 5,                                     // story points (number)
				"customfield_10017": map[string]string{"name": "Platform"}, // team (object with name)
				"customfield_10018": "Sprint 42",                           // sprint name (string)
				"customfield_10019": []map[string]string{ // labels (array of objects)
					{"value": "backend"},
					{"value": "api"},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	cfg := &config.JiraConfig{
		BaseURL: server.URL,
		Email:   "test@example.com",
		Token:   "test-token",
		CustomFields: map[string]string{
			"story_points": "customfield_10016",
			"team":         "customfield_10017",
			"sprint":       "customfield_10018",
			"labels":       "customfield_10019",
		},
	}

	client, err := NewAPIClient(cfg, false)
	if err != nil {
		t.Fatalf("NewAPIClient() error = %v, want nil", err)
	}

	info, err := client.FetchTicketDetails("TEST-123")
	if err != nil {
		t.Fatalf("FetchTicketDetails() error = %v, want nil", err)
	}

	// Verify standard fields
	if info.Type != "Story" {
		t.Errorf("Type = %q, want %q", info.Type, "Story")
	}
	if info.Summary != "Test story with custom fields" {
		t.Errorf("Summary = %q, want %q", info.Summary, "Test story with custom fields")
	}

	// Verify custom fields
	if info.CustomFields == nil {
		t.Fatal("CustomFields is nil, want non-nil map")
	}

	expectedCustomFields := map[string]string{
		"story_points": "5",
		"team":         "Platform",
		"sprint":       "Sprint 42",
		"labels":       "backend, api",
	}

	for name, want := range expectedCustomFields {
		got, ok := info.CustomFields[name]
		if !ok {
			t.Errorf("CustomFields[%q] not found", name)
			continue
		}
		if got != want {
			t.Errorf("CustomFields[%q] = %q, want %q", name, got, want)
		}
	}
}

func TestAPIClient_FetchTicketDetails_NoCustomFieldsConfigured(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"fields": map[string]interface{}{
				"issuetype":         map[string]string{"name": "Bug"},
				"summary":           "Simple bug",
				"status":            map[string]string{"name": "Open"},
				"priority":          map[string]string{"name": "Low"},
				"description":       nil,
				"customfield_10016": 3,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	cfg := &config.JiraConfig{
		BaseURL: server.URL,
		Email:   "test@example.com",
		Token:   "test-token",
		// No CustomFields configured
	}

	client, err := NewAPIClient(cfg, false)
	if err != nil {
		t.Fatalf("NewAPIClient() error = %v, want nil", err)
	}

	info, err := client.FetchTicketDetails("TEST-456")
	if err != nil {
		t.Fatalf("FetchTicketDetails() error = %v, want nil", err)
	}

	// CustomFields should be nil when none configured
	if info.CustomFields != nil {
		t.Errorf("CustomFields = %v, want nil when no custom fields configured", info.CustomFields)
	}
}

func TestAPIClient_FetchTicketDetails_MissingCustomField(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"fields": map[string]interface{}{
				"issuetype":         map[string]string{"name": "Task"},
				"summary":           "Task without story points",
				"status":            map[string]string{"name": "Done"},
				"priority":          map[string]string{"name": "High"},
				"description":       nil,
				"customfield_10017": map[string]string{"name": "Backend Team"},
				// customfield_10016 (story_points) is missing
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	cfg := &config.JiraConfig{
		BaseURL: server.URL,
		Email:   "test@example.com",
		Token:   "test-token",
		CustomFields: map[string]string{
			"story_points": "customfield_10016",
			"team":         "customfield_10017",
		},
	}

	client, err := NewAPIClient(cfg, false)
	if err != nil {
		t.Fatalf("NewAPIClient() error = %v, want nil", err)
	}

	info, err := client.FetchTicketDetails("TEST-789")
	if err != nil {
		t.Fatalf("FetchTicketDetails() error = %v, want nil", err)
	}

	// Only team should be present, story_points should be missing
	if info.CustomFields == nil {
		t.Fatal("CustomFields is nil, want non-nil map")
	}

	if _, ok := info.CustomFields["story_points"]; ok {
		t.Error("CustomFields[\"story_points\"] should not be present for missing field")
	}

	if got := info.CustomFields["team"]; got != "Backend Team" {
		t.Errorf("CustomFields[\"team\"] = %q, want %q", got, "Backend Team")
	}
}

func TestAPIClient_FetchTicketDetails_NullCustomField(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"fields": map[string]interface{}{
				"issuetype":         map[string]string{"name": "Epic"},
				"summary":           "Epic with null custom field",
				"status":            map[string]string{"name": "Planning"},
				"priority":          map[string]string{"name": "Critical"},
				"description":       nil,
				"customfield_10016": nil, // null story points
				"customfield_10017": map[string]string{"name": "Frontend"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	cfg := &config.JiraConfig{
		BaseURL: server.URL,
		Email:   "test@example.com",
		Token:   "test-token",
		CustomFields: map[string]string{
			"story_points": "customfield_10016",
			"team":         "customfield_10017",
		},
	}

	client, err := NewAPIClient(cfg, false)
	if err != nil {
		t.Fatalf("NewAPIClient() error = %v, want nil", err)
	}

	info, err := client.FetchTicketDetails("TEST-999")
	if err != nil {
		t.Fatalf("FetchTicketDetails() error = %v, want nil", err)
	}

	// story_points should not be present (null value)
	if _, ok := info.CustomFields["story_points"]; ok {
		t.Error("CustomFields[\"story_points\"] should not be present for null field")
	}

	// team should be present
	if got := info.CustomFields["team"]; got != "Frontend" {
		t.Errorf("CustomFields[\"team\"] = %q, want %q", got, "Frontend")
	}
}

func TestAPIClient_FetchTicketDetails_ServerErrorWithMessages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		// Return error with errorMessages array
		_, _ = w.Write([]byte(`{"errorMessages":["Database connection failed", "Please try again later"]}`))
	}))
	defer server.Close()

	cfg := &config.JiraConfig{
		BaseURL: server.URL,
		Email:   "test@example.com",
		Token:   "test-token",
	}

	client, err := NewAPIClient(cfg, false)
	if err != nil {
		t.Fatalf("NewAPIClient() error = %v, want nil", err)
	}

	_, err = client.FetchTicketDetails("TEST-123")
	if err == nil {
		t.Fatal("FetchTicketDetails() should return error")
	}

	// Error should contain the error messages from response
	if !contains(err.Error(), "Database connection failed") {
		t.Errorf("error = %q, should contain 'Database connection failed'", err.Error())
	}
}

func TestAPIClient_FetchTicketDetails_ServerErrorWithoutMessages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`invalid json`))
	}))
	defer server.Close()

	cfg := &config.JiraConfig{
		BaseURL: server.URL,
		Email:   "test@example.com",
		Token:   "test-token",
	}

	client, err := NewAPIClient(cfg, false)
	if err != nil {
		t.Fatalf("NewAPIClient() error = %v, want nil", err)
	}

	_, err = client.FetchTicketDetails("TEST-123")
	if err == nil {
		t.Fatal("FetchTicketDetails() should return error")
	}

	// Error should contain the status code
	if !contains(err.Error(), "HTTP 502") {
		t.Errorf("error = %q, should contain 'HTTP 502'", err.Error())
	}
}

func TestAPIClient_FetchTicketDetails_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{invalid json`))
	}))
	defer server.Close()

	cfg := &config.JiraConfig{
		BaseURL: server.URL,
		Email:   "test@example.com",
		Token:   "test-token",
	}

	client, err := NewAPIClient(cfg, false)
	if err != nil {
		t.Fatalf("NewAPIClient() error = %v, want nil", err)
	}

	_, err = client.FetchTicketDetails("TEST-123")
	if err == nil {
		t.Fatal("FetchTicketDetails() should return error for invalid JSON")
	}

	if !contains(err.Error(), "parse") {
		t.Errorf("error = %q, should contain 'parse'", err.Error())
	}
}

func TestAPIClient_FetchTicketDetails_NotConfigured(t *testing.T) {
	// Create a client and then clear its configuration
	cfg := &config.JiraConfig{
		BaseURL: "https://example.atlassian.net",
		Email:   "test@example.com",
		Token:   "test-token",
	}

	client, err := NewAPIClient(cfg, false)
	if err != nil {
		t.Fatalf("NewAPIClient() error = %v, want nil", err)
	}

	// Clear the baseURL to make IsAvailable() return false
	client.baseURL = ""

	_, err = client.FetchTicketDetails("TEST-123")
	if err == nil {
		t.Fatal("FetchTicketDetails() should return error when client is not configured")
	}

	if !contains(err.Error(), "not configured") {
		t.Errorf("error = %q, should contain 'not configured'", err.Error())
	}
}

func TestAPIClient_FetchTicketDetails_MinimalFields(t *testing.T) {
	// Test response with only summary, all other fields missing/nil
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"fields": map[string]interface{}{
				"summary": "Minimal ticket",
				// No issuetype, status, priority, or description
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	cfg := &config.JiraConfig{
		BaseURL: server.URL,
		Email:   "test@example.com",
		Token:   "test-token",
	}

	client, err := NewAPIClient(cfg, false)
	if err != nil {
		t.Fatalf("NewAPIClient() error = %v, want nil", err)
	}

	info, err := client.FetchTicketDetails("TEST-123")
	if err != nil {
		t.Fatalf("FetchTicketDetails() error = %v, want nil", err)
	}

	if info.Summary != "Minimal ticket" {
		t.Errorf("Summary = %q, want %q", info.Summary, "Minimal ticket")
	}
	if info.Type != "" {
		t.Errorf("Type = %q, want empty", info.Type)
	}
	if info.Status != "" {
		t.Errorf("Status = %q, want empty", info.Status)
	}
	if info.Priority != "" {
		t.Errorf("Priority = %q, want empty", info.Priority)
	}
	if info.Description != "" {
		t.Errorf("Description = %q, want empty", info.Description)
	}
}
