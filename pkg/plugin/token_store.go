package plugin

import (
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

// ErrTokenNotFound is returned when a token cannot be resolved to a plugin.
var ErrTokenNotFound = errors.New("token not found")

// tokenStore manages session tokens and their mapping to plugin names.
// It is shared across host-side proxy services (Secret, Context).
//
// A reverse index (pluginTokens) maps plugin names to their set of tokens,
// allowing O(k) UnregisterPlugin where k is the number of tokens for that
// plugin, rather than a full-map scan.
type tokenStore struct {
	mu           sync.RWMutex
	tokens       map[string]string              // token -> plugin name
	pluginTokens map[string]map[string]struct{} // plugin name -> set of tokens
}

func newTokenStore() *tokenStore {
	return &tokenStore{
		tokens:       make(map[string]string),
		pluginTokens: make(map[string]map[string]struct{}),
	}
}

func (s *tokenStore) Register(token, name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens[token] = name
	if s.pluginTokens[name] == nil {
		s.pluginTokens[name] = make(map[string]struct{})
	}
	s.pluginTokens[name][token] = struct{}{}
}

func (s *tokenStore) Unregister(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if name, ok := s.tokens[token]; ok {
		delete(s.tokens, token)
		if toks, ok := s.pluginTokens[name]; ok {
			delete(toks, token)
			if len(toks) == 0 {
				delete(s.pluginTokens, name)
			}
		}
	}
}

func (s *tokenStore) UnregisterPlugin(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if toks, ok := s.pluginTokens[name]; ok {
		for token := range toks {
			delete(s.tokens, token)
		}
		delete(s.pluginTokens, name)
	}
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
		return "", "", ErrTokenNotFound
	}

	u, err := uuid.NewRandom()
	if err != nil {
		return "", "", errors.Wrap(err, "failed to generate replacement token")
	}
	newToken := u.String()

	// Swap tokens in the forward map.
	delete(s.tokens, currentToken)
	s.tokens[newToken] = name

	// Update the reverse index.
	toks, ok := s.pluginTokens[name]
	if !ok || toks == nil {
		toks = make(map[string]struct{})
		s.pluginTokens[name] = toks
	}
	delete(toks, currentToken)
	toks[newToken] = struct{}{}

	return name, newToken, nil
}
