package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	rigerrors "thoreinstein.com/rig/pkg/errors"
)

func TestNewOllamaProvider(t *testing.T) {
	tests := []struct {
		name         string
		endpoint     string
		model        string
		wantEndpoint string
		wantModel    string
	}{
		{
			name:         "empty endpoint uses default",
			endpoint:     "",
			model:        "custom-model",
			wantEndpoint: ollamaDefaultEndpoint,
			wantModel:    "custom-model",
		},
		{
			name:         "empty model uses default",
			endpoint:     "http://custom:1234",
			model:        "",
			wantEndpoint: "http://custom:1234",
			wantModel:    ollamaDefaultModel,
		},
		{
			name:         "both empty use defaults",
			endpoint:     "",
			model:        "",
			wantEndpoint: ollamaDefaultEndpoint,
			wantModel:    ollamaDefaultModel,
		},
		{
			name:         "custom values preserved",
			endpoint:     "http://custom:1234",
			model:        "custom-model",
			wantEndpoint: "http://custom:1234",
			wantModel:    "custom-model",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewOllamaProvider(tt.endpoint, tt.model, nil)

			if p.endpoint != tt.wantEndpoint {
				t.Errorf("endpoint = %q, want %q", p.endpoint, tt.wantEndpoint)
			}
			if p.model != tt.wantModel {
				t.Errorf("model = %q, want %q", p.model, tt.wantModel)
			}
			if p.client == nil {
				t.Error("client should not be nil")
			}
		})
	}
}

func TestOllamaProvider_Name(t *testing.T) {
	p := NewOllamaProvider("", "", nil)
	if got := p.Name(); got != ProviderOllama {
		t.Errorf("Name() = %q, want %q", got, ProviderOllama)
	}
}

func TestOllamaProvider_IsAvailable(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		want     bool
	}{
		{
			name:     "available when endpoint set",
			endpoint: "http://localhost:11434",
			want:     true,
		},
		{
			name:     "not available when endpoint empty",
			endpoint: "",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &OllamaProvider{endpoint: tt.endpoint}
			if got := p.IsAvailable(); got != tt.want {
				t.Errorf("IsAvailable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOllamaProvider_Chat_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method and path
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if r.URL.Path != ollamaChatPath {
			t.Errorf("Expected path %s, got %s", ollamaChatPath, r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type 'application/json', got %s", r.Header.Get("Content-Type"))
		}

		// Verify request body
		var reqBody ollamaRequest
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Errorf("Failed to decode request body: %v", err)
		}
		if reqBody.Model != "llama3.2" {
			t.Errorf("Model = %q, want %q", reqBody.Model, "llama3.2")
		}
		if reqBody.Stream {
			t.Error("Stream should be false for Chat")
		}
		if len(reqBody.Messages) != 1 {
			t.Errorf("Messages count = %d, want 1", len(reqBody.Messages))
		}

		// Return mock response
		response := ollamaResponse{
			Model:           "llama3.2",
			CreatedAt:       "2024-01-01T00:00:00Z",
			Message:         ollamaMessage{Role: "assistant", Content: "Hello! How can I help?"},
			Done:            true,
			PromptEvalCount: 10,
			EvalCount:       20,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	p := NewOllamaProvider(server.URL, "llama3.2", nil)

	resp, err := p.Chat(t.Context(), []Message{
		{Role: "user", Content: "Hello"},
	})
	if err != nil {
		t.Fatalf("Chat() error = %v, want nil", err)
	}

	if resp.Content != "Hello! How can I help?" {
		t.Errorf("Content = %q, want %q", resp.Content, "Hello! How can I help?")
	}
	if resp.StopReason != "stop" {
		t.Errorf("StopReason = %q, want %q", resp.StopReason, "stop")
	}
	if resp.InputTokens != 10 {
		t.Errorf("InputTokens = %d, want %d", resp.InputTokens, 10)
	}
	if resp.OutputTokens != 20 {
		t.Errorf("OutputTokens = %d, want %d", resp.OutputTokens, 20)
	}
}

func TestOllamaProvider_Chat_IncompleteResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := ollamaResponse{
			Model:   "llama3.2",
			Message: ollamaMessage{Role: "assistant", Content: "Partial response..."},
			Done:    false, // Response not complete
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	p := NewOllamaProvider(server.URL, "llama3.2", nil)

	resp, err := p.Chat(t.Context(), []Message{
		{Role: "user", Content: "Hello"},
	})
	if err != nil {
		t.Fatalf("Chat() error = %v, want nil", err)
	}

	if resp.StopReason != "incomplete" {
		t.Errorf("StopReason = %q, want %q", resp.StopReason, "incomplete")
	}
}

func TestOllamaProvider_Chat_HTTPErrors(t *testing.T) {
	tests := []struct {
		name           string
		statusCode     int
		responseBody   string
		wantErrContain string
	}{
		{
			name:           "400 bad request with error message",
			statusCode:     http.StatusBadRequest,
			responseBody:   `{"error": "model not found"}`,
			wantErrContain: "model not found",
		},
		{
			name:           "500 internal server error",
			statusCode:     http.StatusInternalServerError,
			responseBody:   `{"error": "internal error"}`,
			wantErrContain: "internal error",
		},
		{
			name:           "503 service unavailable",
			statusCode:     http.StatusServiceUnavailable,
			responseBody:   `{}`,
			wantErrContain: "HTTP 503",
		},
		{
			name:           "error without json body",
			statusCode:     http.StatusBadGateway,
			responseBody:   `not json`,
			wantErrContain: "HTTP 502",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			p := NewOllamaProvider(server.URL, "llama3.2", nil)

			_, err := p.Chat(t.Context(), []Message{
				{Role: "user", Content: "Hello"},
			})
			if err == nil {
				t.Fatal("Chat() should return error")
			}

			if !strings.Contains(err.Error(), tt.wantErrContain) {
				t.Errorf("error = %q, should contain %q", err.Error(), tt.wantErrContain)
			}

			// Verify it's an AIError
			var aiErr *rigerrors.AIError
			if !rigerrors.As(err, &aiErr) {
				t.Errorf("error should be an AIError, got %T", err)
			}
		})
	}
}

func TestOllamaProvider_Chat_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{invalid json`))
	}))
	defer server.Close()

	p := NewOllamaProvider(server.URL, "llama3.2", nil)

	_, err := p.Chat(t.Context(), []Message{
		{Role: "user", Content: "Hello"},
	})
	if err == nil {
		t.Fatal("Chat() should return error for invalid JSON")
	}

	if !strings.Contains(err.Error(), "parse") {
		t.Errorf("error = %q, should contain 'parse'", err.Error())
	}
}

func TestOllamaProvider_Chat_NotConfigured(t *testing.T) {
	p := &OllamaProvider{endpoint: "", model: "test", client: &http.Client{}}

	_, err := p.Chat(t.Context(), []Message{
		{Role: "user", Content: "Hello"},
	})
	if err == nil {
		t.Fatal("Chat() should return error when not configured")
	}

	if !strings.Contains(err.Error(), "not configured") {
		t.Errorf("error = %q, should contain 'not configured'", err.Error())
	}
}

func TestOllamaProvider_StreamChat_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST request, got %s", r.Method)
		}

		var reqBody ollamaRequest
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Errorf("Failed to decode request body: %v", err)
		}
		if !reqBody.Stream {
			t.Error("Stream should be true for StreamChat")
		}

		// Return newline-delimited JSON chunks
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.WriteHeader(http.StatusOK)

		chunks := []ollamaResponse{
			{Message: ollamaMessage{Content: "Hello"}, Done: false},
			{Message: ollamaMessage{Content: " there"}, Done: false},
			{Message: ollamaMessage{Content: "!"}, Done: false},
			{Message: ollamaMessage{Content: ""}, Done: true},
		}

		for _, chunk := range chunks {
			data, _ := json.Marshal(chunk)
			_, _ = w.Write(data)
			_, _ = w.Write([]byte("\n"))
		}
	}))
	defer server.Close()

	p := NewOllamaProvider(server.URL, "llama3.2", nil)

	chunks, err := p.StreamChat(t.Context(), []Message{
		{Role: "user", Content: "Hello"},
	})
	if err != nil {
		t.Fatalf("StreamChat() error = %v, want nil", err)
	}

	var contentBuilder strings.Builder
	var gotDone bool
	for chunk := range chunks {
		if chunk.Error != nil {
			t.Fatalf("Chunk error = %v", chunk.Error)
		}
		contentBuilder.WriteString(chunk.Content)
		if chunk.Done {
			gotDone = true
		}
	}

	if contentBuilder.String() != "Hello there!" {
		t.Errorf("Content = %q, want %q", contentBuilder.String(), "Hello there!")
	}
	if !gotDone {
		t.Error("Should have received Done=true chunk")
	}
}

func TestOllamaProvider_StreamChat_ContextCancellation(t *testing.T) {
	// Use a channel to coordinate test timing
	serverReady := make(chan struct{})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.WriteHeader(http.StatusOK)

		// Signal that server is ready
		close(serverReady)

		// Keep sending chunks until context is cancelled
		for range 100 {
			chunk := ollamaResponse{Message: ollamaMessage{Content: "chunk "}, Done: false}
			data, _ := json.Marshal(chunk)
			_, err := w.Write(data)
			if err != nil {
				return
			}
			_, err = w.Write([]byte("\n"))
			if err != nil {
				return
			}
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			time.Sleep(10 * time.Millisecond)
		}
	}))
	defer server.Close()

	p := NewOllamaProvider(server.URL, "llama3.2", nil)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	chunks, err := p.StreamChat(ctx, []Message{
		{Role: "user", Content: "Hello"},
	})
	if err != nil {
		t.Fatalf("StreamChat() error = %v, want nil", err)
	}

	// Wait for server to start streaming
	<-serverReady

	// Read a few chunks then cancel
	chunkCount := 0
	for chunk := range chunks {
		chunkCount++
		if chunkCount >= 2 {
			cancel()
		}
		if chunk.Error == context.Canceled {
			// Expected: context was cancelled
			break
		}
		if chunk.Done {
			break
		}
	}

	// Drain any remaining chunks
	for range chunks {
	}
}

func TestOllamaProvider_StreamChat_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error": "model loading failed"}`))
	}))
	defer server.Close()

	p := NewOllamaProvider(server.URL, "llama3.2", nil)

	_, err := p.StreamChat(t.Context(), []Message{
		{Role: "user", Content: "Hello"},
	})
	if err == nil {
		t.Fatal("StreamChat() should return error")
	}

	if !strings.Contains(err.Error(), "model loading failed") {
		t.Errorf("error = %q, should contain 'model loading failed'", err.Error())
	}
}

func TestOllamaProvider_StreamChat_NotConfigured(t *testing.T) {
	p := &OllamaProvider{endpoint: "", model: "test", client: &http.Client{}}

	_, err := p.StreamChat(t.Context(), []Message{
		{Role: "user", Content: "Hello"},
	})
	if err == nil {
		t.Fatal("StreamChat() should return error when not configured")
	}

	if !strings.Contains(err.Error(), "not configured") {
		t.Errorf("error = %q, should contain 'not configured'", err.Error())
	}
}

func TestOllamaProvider_StreamChat_EmptyLines(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.WriteHeader(http.StatusOK)

		// Include empty lines that should be skipped
		_, _ = w.Write([]byte("\n"))
		chunk1, _ := json.Marshal(ollamaResponse{Message: ollamaMessage{Content: "Hello"}, Done: false})
		_, _ = w.Write(chunk1)
		_, _ = w.Write([]byte("\n\n\n"))
		chunk2, _ := json.Marshal(ollamaResponse{Message: ollamaMessage{Content: ""}, Done: true})
		_, _ = w.Write(chunk2)
		_, _ = w.Write([]byte("\n"))
	}))
	defer server.Close()

	p := NewOllamaProvider(server.URL, "llama3.2", nil)

	chunks, err := p.StreamChat(t.Context(), []Message{
		{Role: "user", Content: "Hello"},
	})
	if err != nil {
		t.Fatalf("StreamChat() error = %v, want nil", err)
	}

	var contentBuilder strings.Builder
	for chunk := range chunks {
		if chunk.Error != nil {
			t.Fatalf("Chunk error = %v", chunk.Error)
		}
		contentBuilder.WriteString(chunk.Content)
	}

	if contentBuilder.String() != "Hello" {
		t.Errorf("Content = %q, want %q", contentBuilder.String(), "Hello")
	}
}

func TestOllamaProvider_convertMessages(t *testing.T) {
	p := NewOllamaProvider("", "", nil)

	messages := []Message{
		{Role: "system", Content: "You are a helpful assistant."},
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there!"},
		{Role: "user", Content: "How are you?"},
	}

	result := p.convertMessages(messages)

	if len(result) != len(messages) {
		t.Fatalf("len(result) = %d, want %d", len(result), len(messages))
	}

	for i, msg := range messages {
		if result[i].Role != msg.Role {
			t.Errorf("result[%d].Role = %q, want %q", i, result[i].Role, msg.Role)
		}
		if result[i].Content != msg.Content {
			t.Errorf("result[%d].Content = %q, want %q", i, result[i].Content, msg.Content)
		}
	}
}

func TestOllamaProvider_convertMessages_Empty(t *testing.T) {
	p := NewOllamaProvider("", "", nil)

	result := p.convertMessages([]Message{})

	if len(result) != 0 {
		t.Errorf("len(result) = %d, want 0", len(result))
	}
}

func TestOllamaProvider_Chat_MultipleMessages(t *testing.T) {
	var receivedMessages []ollamaMessage

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody ollamaRequest
		_ = json.NewDecoder(r.Body).Decode(&reqBody)
		receivedMessages = reqBody.Messages

		response := ollamaResponse{
			Message: ollamaMessage{Role: "assistant", Content: "Response"},
			Done:    true,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	p := NewOllamaProvider(server.URL, "llama3.2", nil)

	messages := []Message{
		{Role: "system", Content: "You are helpful."},
		{Role: "user", Content: "First message"},
		{Role: "assistant", Content: "First response"},
		{Role: "user", Content: "Second message"},
	}

	_, err := p.Chat(t.Context(), messages)
	if err != nil {
		t.Fatalf("Chat() error = %v, want nil", err)
	}

	if len(receivedMessages) != 4 {
		t.Fatalf("Server received %d messages, want 4", len(receivedMessages))
	}

	expectedRoles := []string{"system", "user", "assistant", "user"}
	for i, role := range expectedRoles {
		if receivedMessages[i].Role != role {
			t.Errorf("receivedMessages[%d].Role = %q, want %q", i, receivedMessages[i].Role, role)
		}
	}
}

func TestOllamaProvider_handleErrorResponse(t *testing.T) {
	tests := []struct {
		name           string
		statusCode     int
		body           string
		wantErrContain string
		wantRetryable  bool
	}{
		{
			name:           "error with message",
			statusCode:     http.StatusBadRequest,
			body:           `{"error": "invalid model name"}`,
			wantErrContain: "invalid model name",
			wantRetryable:  false,
		},
		{
			name:           "error without message",
			statusCode:     http.StatusNotFound,
			body:           `{}`,
			wantErrContain: "HTTP 404",
			wantRetryable:  false,
		},
		{
			name:           "retryable 503 error",
			statusCode:     http.StatusServiceUnavailable,
			body:           `{"error": "service overloaded"}`,
			wantErrContain: "service overloaded",
			wantRetryable:  true,
		},
		{
			name:           "retryable 429 error",
			statusCode:     http.StatusTooManyRequests,
			body:           `{}`,
			wantErrContain: "HTTP 429",
			wantRetryable:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			p := NewOllamaProvider(server.URL, "llama3.2", nil)

			_, err := p.Chat(t.Context(), []Message{
				{Role: "user", Content: "Hello"},
			})
			if err == nil {
				t.Fatal("Chat() should return error")
			}

			if !strings.Contains(err.Error(), tt.wantErrContain) {
				t.Errorf("error = %q, should contain %q", err.Error(), tt.wantErrContain)
			}

			var aiErr *rigerrors.AIError
			if rigerrors.As(err, &aiErr) {
				if aiErr.Retryable != tt.wantRetryable {
					t.Errorf("Retryable = %v, want %v", aiErr.Retryable, tt.wantRetryable)
				}
				if aiErr.StatusCode != tt.statusCode {
					t.Errorf("StatusCode = %d, want %d", aiErr.StatusCode, tt.statusCode)
				}
			} else {
				t.Errorf("error should be an AIError, got %T", err)
			}
		})
	}
}
