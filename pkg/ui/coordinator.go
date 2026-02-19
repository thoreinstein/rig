package ui

import (
	"sync"
)

// Coordinator manages exclusive access to the terminal for blocking UI operations.
// It ensures that only one plugin can interact with the user via stdin/stdout
// at a time, preventing terminal corruption and interleaved prompts.
type Coordinator struct {
	mu sync.Mutex
}

// NewCoordinator creates a new UI coordinator.
func NewCoordinator() *Coordinator {
	return &Coordinator{}
}

// Lock acquires exclusive access to the terminal. It returns an unlock function
// that MUST be called when the UI operation is complete.
// If another UI operation is active, this method will block until it finishes.
func (c *Coordinator) Lock() func() {
	c.mu.Lock()
	return func() {
		c.mu.Unlock()
	}
}
