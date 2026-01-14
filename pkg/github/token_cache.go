package github

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/zalando/go-keyring"
	"golang.org/x/oauth2"

	rigerrors "thoreinstein.com/rig/pkg/errors"
)

const (
	// KeyringService is the keychain service name for rig.
	KeyringService = "rig-github"
	// KeyringAccount is the keychain account name for OAuth tokens.
	KeyringAccount = "oauth-token"

	// TokenCacheDir is the directory for token cache files.
	TokenCacheDir = ".config/rig" //nolint:gosec // Not a credential, just a directory name
	// TokenCacheFile is the filename for cached tokens.
	TokenCacheFile = "github-token.json" //nolint:gosec // Not a credential, just a filename
)

// TokenCache manages OAuth token storage.
type TokenCache interface {
	Get() (*oauth2.Token, error)
	Set(token *oauth2.Token) error
	Clear() error
}

// cachedToken wraps oauth2.Token with JSON serialization.
type cachedToken struct {
	AccessToken  string    `json:"access_token"`
	TokenType    string    `json:"token_type"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	Expiry       time.Time `json:"expiry,omitempty"`
}

func (c *cachedToken) toOAuth2Token() *oauth2.Token {
	return &oauth2.Token{
		AccessToken:  c.AccessToken,
		TokenType:    c.TokenType,
		RefreshToken: c.RefreshToken,
		Expiry:       c.Expiry,
	}
}

func fromOAuth2Token(t *oauth2.Token) *cachedToken {
	return &cachedToken{
		AccessToken:  t.AccessToken,
		TokenType:    t.TokenType,
		RefreshToken: t.RefreshToken,
		Expiry:       t.Expiry,
	}
}

// NewTokenCache creates a token cache, preferring keychain when available.
func NewTokenCache() TokenCache {
	// Try keychain first - check if keyring is accessible
	// We do a test operation to see if keyring works on this system
	testService := KeyringService + "-test"
	if err := keyring.Set(testService, "test", "test"); err == nil {
		// Clean up test entry
		_ = keyring.Delete(testService, "test")
		return &KeychainTokenCache{
			service: KeyringService,
			account: KeyringAccount,
		}
	}

	// Fall back to file cache
	return &FileTokenCache{
		path: tokenCachePath(),
	}
}

// KeychainTokenCache uses macOS keychain / Linux secret service / Windows credential manager.
type KeychainTokenCache struct {
	service string
	account string
}

// Get retrieves the cached token from keychain.
func (k *KeychainTokenCache) Get() (*oauth2.Token, error) {
	data, err := keyring.Get(k.service, k.account)
	if err != nil {
		if err == keyring.ErrNotFound {
			return nil, nil // No cached token
		}
		return nil, rigerrors.NewGitHubErrorWithCause("TokenCache.Get", "failed to read from keychain", err)
	}

	var cached cachedToken
	if err := json.Unmarshal([]byte(data), &cached); err != nil {
		return nil, rigerrors.NewGitHubErrorWithCause("TokenCache.Get", "failed to parse cached token", err)
	}

	return cached.toOAuth2Token(), nil
}

// Set stores the token in keychain.
func (k *KeychainTokenCache) Set(token *oauth2.Token) error {
	cached := fromOAuth2Token(token)
	data, err := json.Marshal(cached)
	if err != nil {
		return rigerrors.NewGitHubErrorWithCause("TokenCache.Set", "failed to serialize token", err)
	}

	if err := keyring.Set(k.service, k.account, string(data)); err != nil {
		return rigerrors.NewGitHubErrorWithCause("TokenCache.Set", "failed to save to keychain", err)
	}

	return nil
}

// Clear removes the token from keychain.
func (k *KeychainTokenCache) Clear() error {
	err := keyring.Delete(k.service, k.account)
	if err != nil && err != keyring.ErrNotFound {
		return rigerrors.NewGitHubErrorWithCause("TokenCache.Clear", "failed to clear keychain", err)
	}
	return nil
}

// FileTokenCache stores token in a file (fallback for headless systems).
type FileTokenCache struct {
	path string
}

// Get retrieves the cached token from file.
func (f *FileTokenCache) Get() (*oauth2.Token, error) {
	data, err := os.ReadFile(f.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No cached token
		}
		return nil, rigerrors.NewGitHubErrorWithCause("TokenCache.Get", "failed to read token file", err)
	}

	var cached cachedToken
	if err := json.Unmarshal(data, &cached); err != nil {
		return nil, rigerrors.NewGitHubErrorWithCause("TokenCache.Get", "failed to parse cached token", err)
	}

	return cached.toOAuth2Token(), nil
}

// Set stores the token in a file with restrictive permissions.
func (f *FileTokenCache) Set(token *oauth2.Token) error {
	// Ensure directory exists
	dir := filepath.Dir(f.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return rigerrors.NewGitHubErrorWithCause("TokenCache.Set", "failed to create config directory", err)
	}

	cached := fromOAuth2Token(token)
	data, err := json.Marshal(cached)
	if err != nil {
		return rigerrors.NewGitHubErrorWithCause("TokenCache.Set", "failed to serialize token", err)
	}

	// Write with restrictive permissions (owner read/write only)
	if err := os.WriteFile(f.path, data, 0600); err != nil {
		return rigerrors.NewGitHubErrorWithCause("TokenCache.Set", "failed to write token file", err)
	}

	return nil
}

// Clear removes the token file.
func (f *FileTokenCache) Clear() error {
	err := os.Remove(f.path)
	if err != nil && !os.IsNotExist(err) {
		return rigerrors.NewGitHubErrorWithCause("TokenCache.Clear", "failed to remove token file", err)
	}
	return nil
}

func tokenCachePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, TokenCacheDir, TokenCacheFile)
}
