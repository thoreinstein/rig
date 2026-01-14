package github

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func TestFileTokenCache_GetSet(t *testing.T) {
	// Create a temp directory for the test
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "test-token.json")

	cache := &FileTokenCache{path: cachePath}

	// Test Get on non-existent file
	token, err := cache.Get()
	if err != nil {
		t.Fatalf("Get on non-existent file should not error: %v", err)
	}
	if token != nil {
		t.Error("Get on non-existent file should return nil token")
	}

	// Test Set
	testToken := &oauth2.Token{
		AccessToken:  "test-access-token",
		TokenType:    "Bearer",
		RefreshToken: "test-refresh-token",
		Expiry:       time.Now().Add(time.Hour),
	}

	err = cache.Set(testToken)
	if err != nil {
		t.Fatalf("Set should not error: %v", err)
	}

	// Verify file was created with correct permissions
	info, err := os.Stat(cachePath)
	if err != nil {
		t.Fatalf("Token file should exist: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("Token file permissions = %o, want 0600", info.Mode().Perm())
	}

	// Test Get after Set
	retrieved, err := cache.Get()
	if err != nil {
		t.Fatalf("Get after Set should not error: %v", err)
	}
	if retrieved == nil {
		t.Fatal("Get after Set should return non-nil token")
	}
	if retrieved.AccessToken != testToken.AccessToken {
		t.Errorf("AccessToken = %s, want %s", retrieved.AccessToken, testToken.AccessToken)
	}
	if retrieved.TokenType != testToken.TokenType {
		t.Errorf("TokenType = %s, want %s", retrieved.TokenType, testToken.TokenType)
	}
	if retrieved.RefreshToken != testToken.RefreshToken {
		t.Errorf("RefreshToken = %s, want %s", retrieved.RefreshToken, testToken.RefreshToken)
	}
}

func TestFileTokenCache_Clear(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "test-token.json")

	cache := &FileTokenCache{path: cachePath}

	// Set a token first
	testToken := &oauth2.Token{
		AccessToken: "test-token",
		TokenType:   "Bearer",
	}
	err := cache.Set(testToken)
	if err != nil {
		t.Fatalf("Set should not error: %v", err)
	}

	// Clear the cache
	err = cache.Clear()
	if err != nil {
		t.Fatalf("Clear should not error: %v", err)
	}

	// Verify file is gone
	_, err = os.Stat(cachePath)
	if !os.IsNotExist(err) {
		t.Error("Token file should not exist after Clear")
	}

	// Clear again should not error (idempotent)
	err = cache.Clear()
	if err != nil {
		t.Errorf("Clear on non-existent file should not error: %v", err)
	}
}

func TestCachedToken_Serialization(t *testing.T) {
	expiry := time.Now().Add(time.Hour).Truncate(time.Second)
	original := &cachedToken{
		AccessToken:  "access",
		TokenType:    "Bearer",
		RefreshToken: "refresh",
		Expiry:       expiry,
	}

	// Convert to oauth2.Token and back
	oauth2Token := original.toOAuth2Token()
	roundTrip := fromOAuth2Token(oauth2Token)

	if roundTrip.AccessToken != original.AccessToken {
		t.Errorf("AccessToken = %s, want %s", roundTrip.AccessToken, original.AccessToken)
	}
	if roundTrip.TokenType != original.TokenType {
		t.Errorf("TokenType = %s, want %s", roundTrip.TokenType, original.TokenType)
	}
	if roundTrip.RefreshToken != original.RefreshToken {
		t.Errorf("RefreshToken = %s, want %s", roundTrip.RefreshToken, original.RefreshToken)
	}
	if !roundTrip.Expiry.Equal(original.Expiry) {
		t.Errorf("Expiry = %v, want %v", roundTrip.Expiry, original.Expiry)
	}
}

func TestTokenCachePath(t *testing.T) {
	path := tokenCachePath()
	if path == "" {
		t.Error("tokenCachePath should return non-empty path")
	}

	// Should contain the expected components
	if !filepath.IsAbs(path) {
		// It might be relative if home dir can't be determined
		// but should still be valid
		if path == "" {
			t.Error("path should not be empty")
		}
	}
}
