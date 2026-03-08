package plugin

import (
	"sync"
	"testing"
)

func TestTokenStore_ConcurrentRotate(t *testing.T) {
	store := newTokenStore()

	// Register an initial token.
	initialToken := "init-token"
	store.Register(initialToken, "stress-plugin")

	const goroutines = 32
	const rotationsPerGoroutine = 50

	var wg sync.WaitGroup
	wg.Add(goroutines)

	// Each goroutine tries to rotate whatever token it last knew about.
	// Only one should succeed per generation; the rest get ErrTokenNotFound.
	// The test asserts no panics and no data races (run with -race).
	var mu sync.Mutex
	currentToken := initialToken

	for i := range goroutines {
		go func(id int) {
			defer wg.Done()
			for j := range rotationsPerGoroutine {
				_ = j
				mu.Lock()
				tok := currentToken
				mu.Unlock()

				_, newTok, err := store.Rotate(tok)
				if err != nil {
					// Expected: another goroutine already rotated this token.
					continue
				}

				mu.Lock()
				if currentToken == tok {
					currentToken = newTok
				}
				mu.Unlock()
			}
			_ = id
		}(i)
	}

	wg.Wait()

	// After all rotations, exactly one token should resolve to the plugin.
	mu.Lock()
	finalToken := currentToken
	mu.Unlock()

	name, ok := store.Resolve(finalToken)
	if !ok {
		t.Fatal("final token does not resolve")
	}
	if name != "stress-plugin" {
		t.Errorf("resolved name = %q, want %q", name, "stress-plugin")
	}

	// The original token should no longer resolve.
	if _, ok := store.Resolve(initialToken); ok {
		t.Error("initial token still resolves after rotations")
	}
}
