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

// Anthropic API configuration.
const (
	anthropicAPIURL       = "https://api.anthropic.com/v1/messages"
	anthropicAPIVersion   = "2023-06-01"
	anthropicDefaultModel = "claude-sonnet-4-20250514"
	anthropicMaxTokens    = 4096
)

// AnthropicProvider implements Provider for Claude API.
type AnthropicProvider struct {
	apiKey string
	model  string
	logger *slog.Logger
	client *http.Client
}

// NewAnthropicProvider creates a new Anthropic provider.
func NewAnthropicProvider(apiKey, model string, logger *slog.Logger) *AnthropicProvider {
	if model == "" {
		model = anthropicDefaultModel
	}
	return &AnthropicProvider{
		apiKey: apiKey,
		model:  model,
		logger: logger,
		client: &http.Client{},
	}
}

// Name returns the provider name.
func (p *AnthropicProvider) Name() string {
	return ProviderAnthropic
}

// IsAvailable checks if the provider is configured and ready.
func (p *AnthropicProvider) IsAvailable() bool {
	return p.apiKey != ""
}

// anthropicRequest represents an Anthropic API request.
type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	Messages  []anthropicMessage `json:"messages"`
	System    string             `json:"system,omitempty"`
	Stream    bool               `json:"stream,omitempty"`
}

// anthropicMessage represents a message in the Anthropic format.
type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// anthropicResponse represents an Anthropic API response.
type anthropicResponse struct {
	ID           string             `json:"id"`
	Type         string             `json:"type"`
	Role         string             `json:"role"`
	Content      []anthropicContent `json:"content"`
	Model        string             `json:"model"`
	StopReason   string             `json:"stop_reason"`
	StopSequence string             `json:"stop_sequence,omitempty"`
	Usage        anthropicUsage     `json:"usage"`
}

// anthropicContent represents content in an Anthropic response.
type anthropicContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// anthropicUsage represents token usage in an Anthropic response.
type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// anthropicError represents an Anthropic API error response.
type anthropicError struct {
	Type  string `json:"type"`
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// anthropicStreamEvent represents a streaming event from Anthropic.
type anthropicStreamEvent struct {
	Type         string             `json:"type"`
	Index        int                `json:"index,omitempty"`
	ContentBlock *anthropicContent  `json:"content_block,omitempty"`
	Delta        *anthropicDelta    `json:"delta,omitempty"`
	Message      *anthropicResponse `json:"message,omitempty"`
	Usage        *anthropicUsage    `json:"usage,omitempty"`
}

// anthropicDelta represents incremental content in streaming.
type anthropicDelta struct {
	Type       string `json:"type"`
	Text       string `json:"text,omitempty"`
	StopReason string `json:"stop_reason,omitempty"`
}

// Chat performs a single-turn chat completion.
func (p *AnthropicProvider) Chat(ctx context.Context, messages []Message) (*Response, error) {
	if !p.IsAvailable() {
		return nil, rigerrors.NewAIError(ProviderAnthropic, "Chat", "provider not configured")
	}

	// Extract system message and convert messages
	systemPrompt, apiMessages := p.convertMessages(messages)

	reqBody := anthropicRequest{
		Model:     p.model,
		MaxTokens: anthropicMaxTokens,
		Messages:  apiMessages,
		System:    systemPrompt,
	}

	p.logDebug("sending chat request", "model", p.model, "message_count", len(apiMessages))

	respBody, err := p.doRequest(ctx, reqBody)
	if err != nil {
		return nil, err
	}

	var resp anthropicResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, rigerrors.NewAIErrorWithCause(ProviderAnthropic, "Chat",
			"failed to parse response", err)
	}

	// Extract text content
	var content strings.Builder
	for _, c := range resp.Content {
		if c.Type == "text" {
			content.WriteString(c.Text)
		}
	}

	p.logDebug("received response",
		"stop_reason", resp.StopReason,
		"input_tokens", resp.Usage.InputTokens,
		"output_tokens", resp.Usage.OutputTokens)

	return &Response{
		Content:      content.String(),
		StopReason:   resp.StopReason,
		InputTokens:  resp.Usage.InputTokens,
		OutputTokens: resp.Usage.OutputTokens,
	}, nil
}

// StreamChat performs a streaming chat completion.
func (p *AnthropicProvider) StreamChat(ctx context.Context, messages []Message) (<-chan StreamChunk, error) {
	if !p.IsAvailable() {
		return nil, rigerrors.NewAIError(ProviderAnthropic, "StreamChat", "provider not configured")
	}

	// Extract system message and convert messages
	systemPrompt, apiMessages := p.convertMessages(messages)

	reqBody := anthropicRequest{
		Model:     p.model,
		MaxTokens: anthropicMaxTokens,
		Messages:  apiMessages,
		System:    systemPrompt,
		Stream:    true,
	}

	p.logDebug("sending streaming chat request", "model", p.model, "message_count", len(apiMessages))

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, rigerrors.NewAIErrorWithCause(ProviderAnthropic, "StreamChat",
			"failed to marshal request", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, anthropicAPIURL, bytes.NewReader(body))
	if err != nil {
		return nil, rigerrors.NewAIErrorWithCause(ProviderAnthropic, "StreamChat",
			"failed to create request", err)
	}

	p.setHeaders(req)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, rigerrors.NewAIErrorWithCause(ProviderAnthropic, "StreamChat",
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
func (p *AnthropicProvider) streamResponse(ctx context.Context, body io.ReadCloser, chunks chan<- StreamChunk) {
	defer close(chunks)

	// Close body on context cancellation to unblock scanner.Scan().
	// This prevents goroutine leaks when the context is cancelled while
	// the scanner is blocked waiting for network data.
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			body.Close()
		case <-done:
			// Normal completion
		}
	}()
	defer body.Close() // Ensure body is closed on normal exit too

	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			chunks <- StreamChunk{Error: ctx.Err(), Done: true}
			return
		default:
		}

		line := scanner.Text()

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, ":") {
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

		var event anthropicStreamEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			p.logDebug("failed to parse stream event", "error", err, "data", data)
			continue
		}

		switch event.Type {
		case "content_block_delta":
			if event.Delta != nil && event.Delta.Text != "" {
				chunks <- StreamChunk{Content: event.Delta.Text}
			}
		case "message_stop":
			chunks <- StreamChunk{Done: true}
			return
		case "error":
			chunks <- StreamChunk{
				Error: rigerrors.NewAIError(ProviderAnthropic, "StreamChat", "stream error"),
				Done:  true,
			}
			return
		}
	}

	// Check for scanner errors, but ignore errors caused by closing the body
	if err := scanner.Err(); err != nil {
		// If context was cancelled, the error is expected (body was closed)
		if ctx.Err() != nil {
			chunks <- StreamChunk{Error: ctx.Err(), Done: true}
			return
		}
		chunks <- StreamChunk{
			Error: rigerrors.NewAIErrorWithCause(ProviderAnthropic, "StreamChat",
				"stream read error", err),
			Done: true,
		}
	}
}

// convertMessages extracts the system message and converts to Anthropic format.
func (p *AnthropicProvider) convertMessages(messages []Message) (string, []anthropicMessage) {
	var systemPrompt string
	apiMessages := make([]anthropicMessage, 0, len(messages))

	for _, msg := range messages {
		if msg.Role == "system" {
			systemPrompt = msg.Content
			continue
		}
		apiMessages = append(apiMessages, anthropicMessage(msg))
	}

	return systemPrompt, apiMessages
}

// doRequest performs an HTTP request and returns the response body.
func (p *AnthropicProvider) doRequest(ctx context.Context, reqBody anthropicRequest) ([]byte, error) {
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, rigerrors.NewAIErrorWithCause(ProviderAnthropic, "Chat",
			"failed to marshal request", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, anthropicAPIURL, bytes.NewReader(body))
	if err != nil {
		return nil, rigerrors.NewAIErrorWithCause(ProviderAnthropic, "Chat",
			"failed to create request", err)
	}

	p.setHeaders(req)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, rigerrors.NewAIErrorWithCause(ProviderAnthropic, "Chat",
			"request failed", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, p.handleErrorResponse(resp, "Chat")
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, rigerrors.NewAIErrorWithCause(ProviderAnthropic, "Chat",
			"failed to read response", err)
	}

	return respBody, nil
}

// setHeaders sets the required headers for Anthropic API requests.
func (p *AnthropicProvider) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", anthropicAPIVersion)
}

// handleErrorResponse parses error responses from the Anthropic API.
func (p *AnthropicProvider) handleErrorResponse(resp *http.Response, operation string) error {
	body, _ := io.ReadAll(resp.Body)

	var apiErr anthropicError
	if err := json.Unmarshal(body, &apiErr); err == nil && apiErr.Error.Message != "" {
		return rigerrors.NewAIErrorWithStatus(ProviderAnthropic, operation,
			resp.StatusCode, apiErr.Error.Message)
	}

	return rigerrors.NewAIErrorWithStatus(ProviderAnthropic, operation,
		resp.StatusCode, fmt.Sprintf("HTTP %d: %s", resp.StatusCode, http.StatusText(resp.StatusCode)))
}

// logDebug logs a debug message if verbose logging is enabled.
func (p *AnthropicProvider) logDebug(msg string, args ...any) {
	if p.logger != nil {
		p.logger.Debug(msg, args...)
	}
}
