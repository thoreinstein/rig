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

	rigerrors "thoreinstein.com/rig/pkg/errors"
)

// Ollama API configuration.
const (
	ollamaDefaultEndpoint = "http://localhost:11434"
	ollamaDefaultModel    = "llama3.2"
	ollamaChatPath        = "/api/chat"
)

// OllamaProvider implements Provider for Ollama API.
type OllamaProvider struct {
	endpoint string
	model    string
	logger   *slog.Logger
	client   *http.Client
}

// NewOllamaProvider creates a new Ollama provider.
func NewOllamaProvider(endpoint, model string, logger *slog.Logger) *OllamaProvider {
	if endpoint == "" {
		endpoint = ollamaDefaultEndpoint
	}
	if model == "" {
		model = ollamaDefaultModel
	}
	return &OllamaProvider{
		endpoint: endpoint,
		model:    model,
		logger:   logger,
		client:   &http.Client{},
	}
}

// Name returns the provider name.
func (p *OllamaProvider) Name() string {
	return ProviderOllama
}

// IsAvailable checks if the provider is configured and ready.
// For Ollama, we just need an endpoint (no API key required for local instances).
func (p *OllamaProvider) IsAvailable() bool {
	return p.endpoint != ""
}

// ollamaRequest represents an Ollama /api/chat request.
type ollamaRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
}

// ollamaMessage represents a message in the Ollama format.
type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ollamaResponse represents an Ollama /api/chat response.
type ollamaResponse struct {
	Model     string        `json:"model"`
	CreatedAt string        `json:"created_at"`
	Message   ollamaMessage `json:"message"`
	Done      bool          `json:"done"`
	// Token usage fields (only present when done=true)
	PromptEvalCount int `json:"prompt_eval_count,omitempty"`
	EvalCount       int `json:"eval_count,omitempty"`
}

// ollamaError represents an Ollama API error response.
type ollamaError struct {
	Error string `json:"error"`
}

// Chat performs a single-turn chat completion.
func (p *OllamaProvider) Chat(ctx context.Context, messages []Message) (*Response, error) {
	if !p.IsAvailable() {
		return nil, rigerrors.NewAIError(ProviderOllama, "Chat", "provider not configured")
	}

	apiMessages := p.convertMessages(messages)

	reqBody := ollamaRequest{
		Model:    p.model,
		Messages: apiMessages,
		Stream:   false,
	}

	p.logDebug("sending chat request", "model", p.model, "message_count", len(apiMessages))

	respBody, err := p.doRequest(ctx, reqBody)
	if err != nil {
		return nil, err
	}

	var resp ollamaResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, rigerrors.NewAIErrorWithCause(ProviderOllama, "Chat",
			"failed to parse response", err)
	}

	p.logDebug("received response",
		"prompt_tokens", resp.PromptEvalCount,
		"completion_tokens", resp.EvalCount)

	stopReason := "stop"
	if !resp.Done {
		stopReason = "incomplete"
	}

	return &Response{
		Content:      resp.Message.Content,
		StopReason:   stopReason,
		InputTokens:  resp.PromptEvalCount,
		OutputTokens: resp.EvalCount,
	}, nil
}

// StreamChat performs a streaming chat completion.
func (p *OllamaProvider) StreamChat(ctx context.Context, messages []Message) (<-chan StreamChunk, error) {
	if !p.IsAvailable() {
		return nil, rigerrors.NewAIError(ProviderOllama, "StreamChat", "provider not configured")
	}

	apiMessages := p.convertMessages(messages)

	reqBody := ollamaRequest{
		Model:    p.model,
		Messages: apiMessages,
		Stream:   true,
	}

	p.logDebug("sending streaming chat request", "model", p.model, "message_count", len(apiMessages))

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, rigerrors.NewAIErrorWithCause(ProviderOllama, "StreamChat",
			"failed to marshal request", err)
	}

	url := p.endpoint + ollamaChatPath
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, rigerrors.NewAIErrorWithCause(ProviderOllama, "StreamChat",
			"failed to create request", err)
	}

	p.setHeaders(req)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, rigerrors.NewAIErrorWithCause(ProviderOllama, "StreamChat",
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

// streamResponse reads newline-delimited JSON and sends chunks to the channel.
func (p *OllamaProvider) streamResponse(ctx context.Context, body io.ReadCloser, chunks chan<- StreamChunk) {
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
		if line == "" {
			continue
		}

		var resp ollamaResponse
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			p.logDebug("failed to parse stream chunk", "error", err, "line", line)
			continue
		}

		// Send content if present
		if resp.Message.Content != "" {
			chunks <- StreamChunk{Content: resp.Message.Content}
		}

		// Check for completion
		if resp.Done {
			chunks <- StreamChunk{Done: true}
			return
		}
	}

	if err := scanner.Err(); err != nil {
		chunks <- StreamChunk{
			Error: rigerrors.NewAIErrorWithCause(ProviderOllama, "StreamChat",
				"stream read error", err),
			Done: true,
		}
	}
}

// convertMessages converts rig messages to Ollama format.
func (p *OllamaProvider) convertMessages(messages []Message) []ollamaMessage {
	apiMessages := make([]ollamaMessage, 0, len(messages))
	for _, msg := range messages {
		apiMessages = append(apiMessages, ollamaMessage(msg))
	}
	return apiMessages
}

// doRequest performs an HTTP request and returns the response body.
func (p *OllamaProvider) doRequest(ctx context.Context, reqBody ollamaRequest) ([]byte, error) {
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, rigerrors.NewAIErrorWithCause(ProviderOllama, "Chat",
			"failed to marshal request", err)
	}

	url := p.endpoint + ollamaChatPath
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, rigerrors.NewAIErrorWithCause(ProviderOllama, "Chat",
			"failed to create request", err)
	}

	p.setHeaders(req)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, rigerrors.NewAIErrorWithCause(ProviderOllama, "Chat",
			"request failed", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, p.handleErrorResponse(resp, "Chat")
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, rigerrors.NewAIErrorWithCause(ProviderOllama, "Chat",
			"failed to read response", err)
	}

	return respBody, nil
}

// setHeaders sets the required headers for Ollama API requests.
func (p *OllamaProvider) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
}

// handleErrorResponse parses error responses from the Ollama API.
func (p *OllamaProvider) handleErrorResponse(resp *http.Response, operation string) error {
	body, _ := io.ReadAll(resp.Body)

	var apiErr ollamaError
	if err := json.Unmarshal(body, &apiErr); err == nil && apiErr.Error != "" {
		return rigerrors.NewAIErrorWithStatus(ProviderOllama, operation,
			resp.StatusCode, apiErr.Error)
	}

	return rigerrors.NewAIErrorWithStatus(ProviderOllama, operation,
		resp.StatusCode, fmt.Sprintf("HTTP %d: %s", resp.StatusCode, http.StatusText(resp.StatusCode)))
}

// logDebug logs a debug message if verbose logging is enabled.
func (p *OllamaProvider) logDebug(msg string, args ...any) {
	if p.logger != nil {
		p.logger.Debug(msg, args...)
	}
}
