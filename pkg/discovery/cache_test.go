package discovery

import (
	"path/filepath"
	"testing"
)

func TestCache(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "cache.json")

	cache := NewCache(cachePath)

	// Test Save
	projects := []Project{
		{Name: "proj-1", Path: "/path/to/1", Type: "standard"},
		{Name: "proj-2", Path: "/path/to/2", Type: "bare"},
	}
	cache.Update(projects)

	if err := cache.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Test Load
	loadedCache := NewCache(cachePath)
	if err := loadedCache.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(loadedCache.Projects) != 2 {
		t.Errorf("Expected 2 projects, got %d", len(loadedCache.Projects))
	}

	if loadedCache.Projects[0].Name != "proj-1" {
		t.Errorf("Expected proj-1, got %s", loadedCache.Projects[0].Name)
	}

	if loadedCache.LastScanned.IsZero() {
		t.Error("LastScanned should not be zero")
	}
}

func TestCache_LoadNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "missing.json")

	cache := NewCache(cachePath)
	if err := cache.Load(); err != nil {
		t.Fatalf("Load non-existent failed: %v", err)
	}

	if len(cache.Projects) != 0 {
		t.Error("Expected empty projects for non-existent cache")
	}
}
