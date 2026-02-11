package ai

import (
	"context"
	"log/slog"
	"strings"
	"sync"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/googlegenai"

	rigerrors "thoreinstein.com/rig/pkg/errors"
)

// GeminiProvider implements Provider using the Genkit SDK.
type GeminiProvider struct {
	apiKey    string
	modelName string
	logger    *slog.Logger

	initOnce sync.Once
	model    ai.Model
	initErr  error
}

// NewGeminiProvider creates a new Gemini provider.
func NewGeminiProvider(apiKey, modelName string, logger *slog.Logger) *GeminiProvider {
	return &GeminiProvider{
		apiKey:    apiKey,
		modelName: modelName,
		logger:    logger,
	}
}

// Name returns the provider name.
func (p *GeminiProvider) Name() string {
	return ProviderGemini
}

// IsAvailable checks if the provider is configured.
func (p *GeminiProvider) IsAvailable() bool {
	return p.apiKey != ""
}

// init initializes the Genkit client and model.
func (p *GeminiProvider) init(ctx context.Context) error {
	p.initOnce.Do(func() {
		// If model is already set (e.g. by a test), skip initialization
		if p.model != nil {
			return
		}

		if p.apiKey == "" {
			p.initErr = rigerrors.NewAIError(ProviderGemini, "init", "API key not set")
			return
		}

		// Initialize Genkit with the Google AI plugin
		g := genkit.Init(ctx, genkit.WithPlugins(&googlegenai.GoogleAI{APIKey: p.apiKey}))

		// Get the model
		modelName := p.modelName
		if modelName == "" {
			modelName = "gemini-1.5-flash" // Default model
		}

		// Ensure model name has the provider prefix
		fullModelName := modelName
		if !strings.Contains(fullModelName, "/") {
			fullModelName = "googleai/" + fullModelName
		}

		p.model = googlegenai.GoogleAIModel(g, fullModelName)
		if p.model == nil {
			p.initErr = rigerrors.NewAIError(ProviderGemini, "init", "failed to get model: "+fullModelName)
			return
		}

		p.logDebug("gemini provider initialized", "model", fullModelName)
	})

	return p.initErr
}

// Chat performs a single-turn chat completion using the Genkit SDK.
func (p *GeminiProvider) Chat(ctx context.Context, messages []Message) (*Response, error) {
	if err := p.init(ctx); err != nil {
		return nil, err
	}

	genkitMessages := p.toGenkitMessages(messages)

	p.logDebug("sending chat request to gemini", "message_count", len(genkitMessages))

	resp, err := p.model.Generate(ctx, &ai.ModelRequest{
		Messages: genkitMessages,
	}, nil)
	if err != nil {
		return nil, rigerrors.NewAIErrorWithCause(ProviderGemini, "Chat", "genkit generate failed", err)
	}

	if resp.Message == nil {
		return nil, rigerrors.NewAIError(ProviderGemini, "Chat", "received empty response from gemini")
	}

	var content strings.Builder
	for _, part := range resp.Message.Content {
		if part.IsText() {
			content.WriteString(part.Text)
		}
	}

	res := &Response{
		Content: content.String(),
	}
	if resp.Usage != nil {
		res.InputTokens = resp.Usage.InputTokens
		res.OutputTokens = resp.Usage.OutputTokens
	}

	return res, nil
}

// StreamChat performs a streaming chat completion. (To be implemented in next phase)
func (p *GeminiProvider) StreamChat(ctx context.Context, messages []Message) (<-chan StreamChunk, error) {
	// Temporary stub for Phase 2
	return nil, rigerrors.NewAIError(ProviderGemini, "StreamChat", "streaming not yet implemented for SDK provider")
}

func (p *GeminiProvider) toGenkitMessages(messages []Message) []*ai.Message {
	genkitMessages := make([]*ai.Message, len(messages))
	for i, m := range messages {
		role := ai.RoleUser
		switch m.Role {
		case "system":
			role = ai.RoleSystem
		case "assistant":
			role = ai.RoleModel
		}
		genkitMessages[i] = &ai.Message{
			Role:    role,
			Content: []*ai.Part{ai.NewTextPart(m.Content)},
		}
	}
	return genkitMessages
}

func (p *GeminiProvider) logDebug(msg string, args ...any) {
	if p.logger != nil {
		p.logger.Debug(msg, args...)
	}
}
