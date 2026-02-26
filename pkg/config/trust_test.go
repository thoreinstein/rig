package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsImmutable(t *testing.T) {
	tests := []struct {
		key  string
		want bool
	}{
		{"github.token", true},
		{"ai.api_key", true},
		{"daemon.enabled", true},
		{"git.base_branch", false},
		{"notes.path", false},
	}

	for _, tt := range tests {
		if got := IsImmutable(tt.key); got != tt.want {
			t.Errorf("IsImmutable(%q) = %v, want %v", tt.key, got, tt.want)
		}
	}
}

func TestTrustStore(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "trusted-projects.json")

	store := &TrustStore{
		path:    path,
		trusted: make(map[string]TrustEntry),
	}

	project := "/tmp/test-project"

	// Test IsTrusted on empty store
	if store.IsTrusted(project) {
		t.Error("expected project to be untrusted")
	}

	// Test Add
	if err := store.Add(project); err != nil {
		t.Fatalf("failed to add project: %v", err)
	}

	if !store.IsTrusted(project) {
		t.Error("expected project to be trusted after Add")
	}

	// Test Persistence
	store2 := &TrustStore{
		path:    path,
		trusted: make(map[string]TrustEntry),
	}
	if err := store2.Load(); err != nil {
		t.Fatalf("failed to load store: %v", err)
	}

	if !store2.IsTrusted(project) {
		t.Error("expected project to be trusted after reload")
	}

	// Test List
	list := store2.List()
	if len(list) != 1 {
		t.Errorf("expected 1 trusted project, got %d", len(list))
	}

	// Test Remove
	if err := store.Remove(project); err != nil {
		t.Fatalf("failed to remove project: %v", err)
	}

	if store.IsTrusted(project) {
		t.Error("expected project to be untrusted after Remove")
	}

	// Verify persistence of removal
	if err := store2.Load(); err != nil {
		t.Fatalf("failed to reload store: %v", err)
	}
	if store2.IsTrusted(project) {
		t.Error("expected project to be untrusted after reload of removal")
	}
}

func TestTrustStore_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "empty.json")
	if err := os.WriteFile(path, []byte(""), 0600); err != nil {
		t.Fatal(err)
	}

	store := &TrustStore{
		path:    path,
		trusted: make(map[string]TrustEntry),
	}

	if err := store.Load(); err != nil {
		t.Errorf("expected no error loading empty file, got %v", err)
	}
}
