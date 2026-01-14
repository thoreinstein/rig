// Package ai provides AI provider integration for the rig project.
//
// This package implements a provider-agnostic interface for interacting with
// AI services like Anthropic (Claude) and Groq. It supports both single-turn
// and streaming chat completions with proper error handling and retry logic.
package ai

import (
	"context"
	"log/slog"
	"os"

	"thoreinstein.com/rig/pkg/config"
	rigerrors "thoreinstein.com/rig/pkg/errors"
)

// Message represents a conversation message.
type Message struct {
	Role    string // "user", "assistant", "system"
	Content string
}

// Response from AI provider.
type Response struct {
	Content      string
	StopReason   string // "end_turn", "max_tokens", etc.
	InputTokens  int
	OutputTokens int
}

// StreamChunk for streaming responses.
type StreamChunk struct {
	Content string
	Done    bool
	Error   error
}

// Provider interface for AI operations.
type Provider interface {
	// IsAvailable checks if provider is available and configured.
	IsAvailable() bool

	// Chat performs a single-turn chat completion.
	Chat(ctx context.Context, messages []Message) (*Response, error)

	// StreamChat performs a streaming chat completion.
	// Returns a channel that receives chunks until Done is true or Error is set.
	StreamChat(ctx context.Context, messages []Message) (<-chan StreamChunk, error)

	// Name returns the provider name.
	Name() string
}

// Provider name constants.
const (
	ProviderAnthropic = "anthropic"
	ProviderGroq      = "groq"
	ProviderOllama    = "ollama"
)

// NewProvider creates an AI provider based on config.
// Environment variables take precedence over config file values for API keys.
func NewProvider(cfg *config.AIConfig, verbose bool) (Provider, error) {
	if cfg == nil {
		return nil, rigerrors.NewConfigError("ai", "config is nil")
	}

	if !cfg.Enabled {
		return nil, rigerrors.NewConfigError("ai.enabled", "AI is disabled in configuration")
	}

	var logger *slog.Logger
	if verbose {
		logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	}

	switch cfg.Provider {
	case ProviderAnthropic:
		apiKey := resolveAnthropicAPIKey(cfg.APIKey)
		if apiKey == "" {
			return nil, rigerrors.NewConfigError("ai.api_key",
				"Anthropic API key not set (set ANTHROPIC_API_KEY or ai.api_key in config)")
		}
		return NewAnthropicProvider(apiKey, cfg.Model, logger), nil

	case ProviderGroq:
		apiKey := resolveGroqAPIKey(cfg.APIKey)
		if apiKey == "" {
			return nil, rigerrors.NewConfigError("ai.api_key",
				"Groq API key not set (set GROQ_API_KEY or ai.api_key in config)")
		}
		return NewGroqProvider(apiKey, cfg.Model, logger), nil

	case ProviderOllama:
		return NewOllamaProvider(cfg.Endpoint, cfg.Model, logger), nil

	default:
		return nil, rigerrors.NewConfigError("ai.provider",
			"unsupported AI provider: "+cfg.Provider+" (supported: anthropic, groq, ollama)")
	}
}

// resolveAnthropicAPIKey returns the API key from ANTHROPIC_API_KEY environment
// variable if set, otherwise falls back to the config value.
func resolveAnthropicAPIKey(configKey string) string {
	if envKey := os.Getenv("ANTHROPIC_API_KEY"); envKey != "" {
		return envKey
	}
	return configKey
}

// resolveGroqAPIKey returns the API key from GROQ_API_KEY environment
// variable if set, otherwise falls back to the config value.
func resolveGroqAPIKey(configKey string) string {
	if envKey := os.Getenv("GROQ_API_KEY"); envKey != "" {
		return envKey
	}
	return configKey
}
