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

	apiv1 "thoreinstein.com/rig/pkg/api/v1"
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
	ProviderGemini    = "gemini"
	ProviderPlugin    = "plugin"
)

// PluginManager is a minimal interface for starting and getting an assistant plugin.
// This avoids circular dependencies with pkg/plugin.
type PluginManager interface {
	GetAssistantClient(ctx context.Context, name string) (apiv1.AssistantServiceClient, error)
}

// NewProvider creates an AI provider based on config.
// Environment variables take precedence over config file values for API keys.
// When model is empty, provider-specific default models from config are used.
func NewProvider(cfg *config.AIConfig, verbose bool) (Provider, error) {
	return NewProviderWithManager(nil, cfg, verbose)
}

// NewProviderWithManager creates an AI provider with an optional PluginManager for plugin-based providers.
func NewProviderWithManager(mgr PluginManager, cfg *config.AIConfig, verbose bool) (Provider, error) {
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
		// Use global model if set, otherwise use provider-specific default
		model := cfg.Model
		if model == "" {
			model = cfg.AnthropicModel
		}
		return NewAnthropicProvider(apiKey, model, logger), nil

	case ProviderGroq:
		apiKey := resolveGroqAPIKey(cfg.APIKey)
		if apiKey == "" {
			return nil, rigerrors.NewConfigError("ai.api_key",
				"Groq API key not set (set GROQ_API_KEY or ai.api_key in config)")
		}
		// Use global model if set, otherwise use provider-specific default
		model := cfg.Model
		if model == "" {
			model = cfg.GroqModel
		}
		return NewGroqProvider(apiKey, model, logger), nil

	case ProviderOllama:
		// Use global model if set, otherwise use provider-specific default
		model := cfg.Model
		if model == "" {
			model = cfg.OllamaModel
		}
		// Use global endpoint if set, otherwise use provider-specific default
		endpoint := cfg.Endpoint
		if endpoint == "" {
			endpoint = cfg.OllamaEndpoint
		}
		return NewOllamaProvider(endpoint, model, logger), nil

	case ProviderGemini:
		apiKey := resolveGeminiAPIKey(cfg.GeminiAPIKey)
		if apiKey == "" {
			apiKey = cfg.APIKey // Provider-agnostic fallback to global key
		}
		if apiKey == "" {
			return nil, rigerrors.NewConfigError("ai.gemini_api_key",
				"Gemini API key not set (set GOOGLE_GENAI_API_KEY or ai.gemini_api_key in config)")
		}
		model := cfg.Model
		if model == "" {
			model = cfg.GeminiModel
		}
		return NewGeminiProvider(apiKey, model, logger), nil

	case ProviderPlugin:
		if mgr == nil {
			return nil, rigerrors.NewConfigError("ai.provider", "plugin provider requested but no plugin manager provided")
		}
		// In config, use model field to specify the plugin name/ID
		pluginName := cfg.Model
		if pluginName == "" {
			return nil, rigerrors.NewConfigError("ai.model", "plugin provider requires the plugin name to be specified in the 'model' field")
		}
		client, err := mgr.GetAssistantClient(context.Background(), pluginName)
		if err != nil {
			return nil, rigerrors.NewAIErrorWithCause(ProviderPlugin, "GetAssistantClient", "failed to get assistant client from plugin", err)
		}
		return NewPluginAssistantProvider(pluginName, client, logger), nil

	default:
		return nil, rigerrors.NewConfigError("ai.provider",
			"unsupported AI provider: "+cfg.Provider+" (supported: anthropic, groq, ollama, gemini, plugin)")
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

// resolveGeminiAPIKey returns the API key from GOOGLE_GENAI_API_KEY environment
// variable if set, otherwise falls back to the config value.
func resolveGeminiAPIKey(configKey string) string {
	if envKey := os.Getenv("GOOGLE_GENAI_API_KEY"); envKey != "" {
		return envKey
	}
	return configKey
}
