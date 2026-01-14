package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	rigerrors "thoreinstein.com/rig/pkg/errors"
)

// Groq API configuration.
const (
	groqAPIURL       = "https://api.groq.com/openai/v1/chat/completions"
	groqDefaultModel = "llama-3.3-70b-versatile"
	groqMaxTokens    = 4096
)

// GroqProvider implements Provider for Groq API (OpenAI-compatible).
type GroqProvider struct {
	apiKey string
	model  string
	logger *slog.Logger
	client *http.Client
}

// NewGroqProvider creates a new Groq provider.
func NewGroqProvider(apiKey, model string, logger *slog.Logger) *GroqProvider {
	if model == "" {
		model = groqDefaultModel
	}
	return &GroqProvider{
		apiKey: apiKey,
		model:  model,
		logger: logger,
		client: &http.Client{},
	}
}

// Name returns the provider name.
func (p *GroqProvider) Name() string {
	return ProviderGroq
}

// IsAvailable checks if the provider is configured and ready.
func (p *GroqProvider) IsAvailable() bool {
	return p.apiKey != ""
}

// openAIRequest represents an OpenAI-compatible API request.
type openAIRequest struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
}

// openAIMessage represents a message in the OpenAI format.
type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// openAIResponse represents an OpenAI-compatible API response.
type openAIResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []openAIChoice `json:"choices"`
	Usage   openAIUsage    `json:"usage"`
}

// openAIChoice represents a choice in the OpenAI response.
type openAIChoice struct {
	Index        int           `json:"index"`
	Message      openAIMessage `json:"message"`
	Delta        *openAIDelta  `json:"delta,omitempty"`
	FinishReason string        `json:"finish_reason"`
}

// openAIDelta represents incremental content in streaming.
type openAIDelta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

// openAIUsage represents token usage in the OpenAI response.
type openAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// openAIError represents an OpenAI-compatible API error response.
type openAIError struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code,omitempty"`
	} `json:"error"`
}

// Chat performs a single-turn chat completion.
func (p *GroqProvider) Chat(ctx context.Context, messages []Message) (*Response, error) {
	if !p.IsAvailable() {
		return nil, rigerrors.NewAIError(ProviderGroq, "Chat", "provider not configured")
	}

	apiMessages := p.convertMessages(messages)

	reqBody := openAIRequest{
		Model:     p.model,
		Messages:  apiMessages,
		MaxTokens: groqMaxTokens,
	}

	p.logDebug("sending chat request", "model", p.model, "message_count", len(apiMessages))

	respBody, err := p.doRequest(ctx, reqBody)
	if err != nil {
		return nil, err
	}

	var resp openAIResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, rigerrors.NewAIErrorWithCause(ProviderGroq, "Chat",
			"failed to parse response", err)
	}

	if len(resp.Choices) == 0 {
		return nil, rigerrors.NewAIError(ProviderGroq, "Chat", "no choices in response")
	}

	choice := resp.Choices[0]

	p.logDebug("received response",
		"finish_reason", choice.FinishReason,
		"prompt_tokens", resp.Usage.PromptTokens,
		"completion_tokens", resp.Usage.CompletionTokens)

	return &Response{
		Content:      choice.Message.Content,
		StopReason:   choice.FinishReason,
		InputTokens:  resp.Usage.PromptTokens,
		OutputTokens: resp.Usage.CompletionTokens,
	}, nil
}

// StreamChat performs a streaming chat completion.
func (p *GroqProvider) StreamChat(ctx context.Context, messages []Message) (<-chan StreamChunk, error) {
	if !p.IsAvailable() {
		return nil, rigerrors.NewAIError(ProviderGroq, "StreamChat", "provider not configured")
	}

	apiMessages := p.convertMessages(messages)

	reqBody := openAIRequest{
		Model:     p.model,
		Messages:  apiMessages,
		MaxTokens: groqMaxTokens,
		Stream:    true,
	}

	p.logDebug("sending streaming chat request", "model", p.model, "message_count", len(apiMessages))

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, rigerrors.NewAIErrorWithCause(ProviderGroq, "StreamChat",
			"failed to marshal request", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, groqAPIURL, bytes.NewReader(body))
	if err != nil {
		return nil, rigerrors.NewAIErrorWithCause(ProviderGroq, "StreamChat",
			"failed to create request", err)
	}

	p.setHeaders(req)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, rigerrors.NewAIErrorWithCause(ProviderGroq, "StreamChat",
			"request failed", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, p.handleErrorResponse(resp, "StreamChat")
	}

	chunks := make(chan StreamChunk)
	go p.streamResponse(ctx, resp.Body, chunks)

	return chunks, nil
}

// streamResponse reads SSE events and sends chunks to the channel.
func (p *GroqProvider) streamResponse(ctx context.Context, body io.ReadCloser, chunks chan<- StreamChunk) {
	defer close(chunks)
	defer body.Close()

	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			chunks <- StreamChunk{Error: ctx.Err(), Done: true}
			return
		default:
		}

		line := scanner.Text()

		// Skip empty lines
		if line == "" {
			continue
		}

		// Parse SSE data lines
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")

		// Handle stream end
		if data == "[DONE]" {
			chunks <- StreamChunk{Done: true}
			return
		}

		var resp openAIResponse
		if err := json.Unmarshal([]byte(data), &resp); err != nil {
			p.logDebug("failed to parse stream event", "error", err, "data", data)
			continue
		}

		if len(resp.Choices) > 0 && resp.Choices[0].Delta != nil {
			delta := resp.Choices[0].Delta
			if delta.Content != "" {
				chunks <- StreamChunk{Content: delta.Content}
			}
			if resp.Choices[0].FinishReason != "" {
				chunks <- StreamChunk{Done: true}
				return
			}
		}
	}

	if err := scanner.Err(); err != nil {
		chunks <- StreamChunk{
			Error: rigerrors.NewAIErrorWithCause(ProviderGroq, "StreamChat",
				"stream read error", err),
			Done: true,
		}
	}
}

// convertMessages converts rig messages to OpenAI format.
func (p *GroqProvider) convertMessages(messages []Message) []openAIMessage {
	apiMessages := make([]openAIMessage, 0, len(messages))
	for _, msg := range messages {
		apiMessages = append(apiMessages, openAIMessage(msg))
	}
	return apiMessages
}

// doRequest performs an HTTP request and returns the response body.
func (p *GroqProvider) doRequest(ctx context.Context, reqBody openAIRequest) ([]byte, error) {
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, rigerrors.NewAIErrorWithCause(ProviderGroq, "Chat",
			"failed to marshal request", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, groqAPIURL, bytes.NewReader(body))
	if err != nil {
		return nil, rigerrors.NewAIErrorWithCause(ProviderGroq, "Chat",
			"failed to create request", err)
	}

	p.setHeaders(req)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, rigerrors.NewAIErrorWithCause(ProviderGroq, "Chat",
			"request failed", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, p.handleErrorResponse(resp, "Chat")
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, rigerrors.NewAIErrorWithCause(ProviderGroq, "Chat",
			"failed to read response", err)
	}

	return respBody, nil
}

// setHeaders sets the required headers for Groq API requests.
func (p *GroqProvider) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
}

// handleErrorResponse parses error responses from the Groq API.
func (p *GroqProvider) handleErrorResponse(resp *http.Response, operation string) error {
	body, _ := io.ReadAll(resp.Body)

	var apiErr openAIError
	if err := json.Unmarshal(body, &apiErr); err == nil && apiErr.Error.Message != "" {
		return rigerrors.NewAIErrorWithStatus(ProviderGroq, operation,
			resp.StatusCode, apiErr.Error.Message)
	}

	return rigerrors.NewAIErrorWithStatus(ProviderGroq, operation,
		resp.StatusCode, fmt.Sprintf("HTTP %d: %s", resp.StatusCode, http.StatusText(resp.StatusCode)))
}

// logDebug logs a debug message if verbose logging is enabled.
func (p *GroqProvider) logDebug(msg string, args ...any) {
	if p.logger != nil {
		p.logger.Debug(msg, args...)
	}
}
