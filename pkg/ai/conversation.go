package ai

import (
	"context"
)

// Conversation manages multi-turn conversation state.
type Conversation struct {
	provider Provider
	messages []Message
	system   string
}

// NewConversation creates a new conversation with a system prompt.
func NewConversation(provider Provider, systemPrompt string) *Conversation {
	return &Conversation{
		provider: provider,
		messages: make([]Message, 0),
		system:   systemPrompt,
	}
}

// AddUserMessage adds a user message to the conversation.
func (c *Conversation) AddUserMessage(content string) {
	c.messages = append(c.messages, Message{
		Role:    "user",
		Content: content,
	})
}

// AddAssistantMessage adds an assistant message to the conversation.
func (c *Conversation) AddAssistantMessage(content string) {
	c.messages = append(c.messages, Message{
		Role:    "assistant",
		Content: content,
	})
}

// Send sends the conversation and gets a response.
// The response is automatically appended to the conversation history.
func (c *Conversation) Send(ctx context.Context) (*Response, error) {
	messages := c.buildMessages()

	resp, err := c.provider.Chat(ctx, messages)
	if err != nil {
		return nil, err
	}

	// Append the assistant's response to the conversation history
	c.AddAssistantMessage(resp.Content)

	return resp, nil
}

// Stream sends the conversation with streaming response.
// The complete response is automatically appended to the conversation history
// when streaming completes.
func (c *Conversation) Stream(ctx context.Context) (<-chan StreamChunk, error) {
	messages := c.buildMessages()

	chunks, err := c.provider.StreamChat(ctx, messages)
	if err != nil {
		return nil, err
	}

	// Wrap the channel to collect the full response
	wrapped := make(chan StreamChunk)
	go c.collectStreamResponse(ctx, chunks, wrapped)

	return wrapped, nil
}

// collectStreamResponse forwards chunks and collects the complete response.
func (c *Conversation) collectStreamResponse(ctx context.Context, in <-chan StreamChunk, out chan<- StreamChunk) {
	defer close(out)

	var fullContent string
	for chunk := range in {
		select {
		case <-ctx.Done():
			out <- StreamChunk{Error: ctx.Err(), Done: true}
			return
		default:
		}

		if chunk.Content != "" {
			fullContent += chunk.Content
		}
		out <- chunk

		if chunk.Done {
			// Only add to history if we got content and no error
			if chunk.Error == nil && fullContent != "" {
				c.AddAssistantMessage(fullContent)
			}
			return
		}
	}
}

// Clear resets the conversation history while keeping the system prompt.
func (c *Conversation) Clear() {
	c.messages = make([]Message, 0)
}

// History returns all messages in the conversation (excluding system prompt).
func (c *Conversation) History() []Message {
	// Return a copy to prevent external modification
	result := make([]Message, len(c.messages))
	copy(result, c.messages)
	return result
}

// SetSystemPrompt updates the system prompt.
func (c *Conversation) SetSystemPrompt(prompt string) {
	c.system = prompt
}

// SystemPrompt returns the current system prompt.
func (c *Conversation) SystemPrompt() string {
	return c.system
}

// MessageCount returns the number of messages in the conversation history.
func (c *Conversation) MessageCount() int {
	return len(c.messages)
}

// buildMessages constructs the full message list with system prompt.
func (c *Conversation) buildMessages() []Message {
	// Calculate capacity: system prompt (if any) + conversation messages
	capacity := len(c.messages)
	if c.system != "" {
		capacity++
	}

	messages := make([]Message, 0, capacity)

	// Add system prompt first if present
	if c.system != "" {
		messages = append(messages, Message{
			Role:    "system",
			Content: c.system,
		})
	}

	// Add conversation messages
	messages = append(messages, c.messages...)

	return messages
}
