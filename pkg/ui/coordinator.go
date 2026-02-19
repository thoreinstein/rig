package ui

import (
	"context"
)

// Coordinator manages exclusive access to the terminal for blocking UI operations.
// It ensures that only one plugin can interact with the user via stdin/stdout
// at a time, preventing terminal corruption and interleaved prompts.
type Coordinator struct {
	sem chan struct{}
}

// NewCoordinator creates a new UI coordinator.
func NewCoordinator() *Coordinator {
	return &Coordinator{
		sem: make(chan struct{}, 1),
	}
}

// Lock acquires exclusive access to the terminal, respecting the provided context.
// It returns an unlock function and an error (if the context was canceled while waiting).
func (c *Coordinator) Lock(ctx context.Context) (func(), error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case c.sem <- struct{}{}:
		return func() { <-c.sem }, nil
	}
}
