package plugin

import (
	"sync"

	"github.com/google/uuid"
)

// tokenStore manages session tokens and their mapping to plugin names.
// It is shared across host-side proxy services (Secret, Context).
type tokenStore struct {
	mu     sync.RWMutex
	tokens map[string]string // token -> plugin name
}

func newTokenStore() *tokenStore {
	return &tokenStore{
		tokens: make(map[string]string),
	}
}

func (s *tokenStore) Register(token, name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens[token] = name
}

func (s *tokenStore) Unregister(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.tokens, token)
}

func (s *tokenStore) Resolve(token string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	name, ok := s.tokens[token]
	return name, ok
}

func (s *tokenStore) Rotate(currentToken string) (string, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	name, ok := s.tokens[currentToken]
	if !ok {
		return "", "", nil // not found
	}

	u, err := uuid.NewRandom()
	if err != nil {
		return "", "", err
	}
	newToken := u.String()

	delete(s.tokens, currentToken)
	s.tokens[newToken] = name

	return name, newToken, nil
}
