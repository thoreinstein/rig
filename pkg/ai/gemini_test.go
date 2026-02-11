package ai

import (
	"context"
	"errors"
	"testing"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/core/api"

	rigerrors "thoreinstein.com/rig/pkg/errors"
)

type mockModel struct {
	generateFn func(ctx context.Context, req *ai.ModelRequest, cb ai.ModelStreamCallback) (*ai.ModelResponse, error)
}

func (m *mockModel) Name() string { return "mock-model" }
func (m *mockModel) Generate(ctx context.Context, req *ai.ModelRequest, cb ai.ModelStreamCallback) (*ai.ModelResponse, error) {
	return m.generateFn(ctx, req, cb)
}
func (m *mockModel) Register(r api.Registry) {}

func TestGeminiProvider_IsAvailable(t *testing.T) {
	tests := []struct {
		name   string
		apiKey string
		want   bool
	}{
		{"available with key", "some-key", true},
		{"not available without key", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewGeminiProvider(tt.apiKey, "", nil)
			if got := p.IsAvailable(); got != tt.want {
				t.Errorf("IsAvailable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGeminiProvider_Name(t *testing.T) {
	p := NewGeminiProvider("key", "", nil)
	if got := p.Name(); got != ProviderGemini {
		t.Errorf("Name() = %q, want %q", got, ProviderGemini)
	}
}

func TestGeminiProvider_Chat_Success(t *testing.T) {
	mock := &mockModel{
		generateFn: func(ctx context.Context, req *ai.ModelRequest, cb ai.ModelStreamCallback) (*ai.ModelResponse, error) {
			// Verify request
			if len(req.Messages) != 1 {
				t.Errorf("expected 1 message, got %d", len(req.Messages))
			}
			if len(req.Messages[0].Content) != 1 {
				t.Errorf("expected 1 part, got %d", len(req.Messages[0].Content))
			}
			if req.Messages[0].Content[0].Text != "Hello" {
				t.Errorf("expected content 'Hello', got %q", req.Messages[0].Content[0].Text)
			}

			return &ai.ModelResponse{
				Message: &ai.Message{
					Role:    ai.RoleModel,
					Content: []*ai.Part{ai.NewTextPart("Hi there!")},
				},
				Usage: &ai.GenerationUsage{
					InputTokens:  10,
					OutputTokens: 20,
				},
			}, nil
		},
	}

	p := NewGeminiProvider("key", "gemini-pro", nil)
	p.model = mock // Inject mock

	resp, err := p.Chat(t.Context(), []Message{
		{Role: "user", Content: "Hello"},
	})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}

	if resp.Content != "Hi there!" {
		t.Errorf("Content = %q, want %q", resp.Content, "Hi there!")
	}
	if resp.InputTokens != 10 {
		t.Errorf("InputTokens = %d, want 10", resp.InputTokens)
	}
	if resp.OutputTokens != 20 {
		t.Errorf("OutputTokens = %d, want 20", resp.OutputTokens)
	}
}

func TestGeminiProvider_Chat_Error(t *testing.T) {
	mock := &mockModel{
		generateFn: func(ctx context.Context, req *ai.ModelRequest, cb ai.ModelStreamCallback) (*ai.ModelResponse, error) {
			return nil, errors.New("api error")
		},
	}

	p := NewGeminiProvider("key", "gemini-pro", nil)
	p.model = mock

	_, err := p.Chat(t.Context(), []Message{{Role: "user", Content: "Hello"}})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var aiErr *rigerrors.AIError
	if !rigerrors.As(err, &aiErr) {
		t.Errorf("expected AIError, got %T", err)
	}
}

func TestGeminiProvider_InitError(t *testing.T) {
	// Test without API key
	p := NewGeminiProvider("", "", nil)
	_, err := p.Chat(t.Context(), []Message{{Role: "user", Content: "Hello"}})
	if err == nil {
		t.Fatal("expected error due to missing API key, got nil")
	}
}

func TestGeminiProvider_toGenkitMessages(t *testing.T) {
	p := NewGeminiProvider("key", "", nil)
	msgs := []Message{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "usr"},
		{Role: "assistant", Content: "ast"},
	}
	genkitMsgs := p.toGenkitMessages(msgs)

	if len(genkitMsgs) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(genkitMsgs))
	}

	if genkitMsgs[0].Role != ai.RoleSystem {
		t.Errorf("expected RoleSystem, got %v", genkitMsgs[0].Role)
	}
	if genkitMsgs[1].Role != ai.RoleUser {
		t.Errorf("expected RoleUser, got %v", genkitMsgs[1].Role)
	}
	if genkitMsgs[2].Role != ai.RoleModel {
		t.Errorf("expected RoleModel, got %v", genkitMsgs[2].Role)
	}
}
