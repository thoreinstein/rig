package discovery

import (
	"os"
	"path/filepath"
	"testing"

	"thoreinstein.com/rig/pkg/config"
)

func TestEngine(t *testing.T) {
	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "src")
	cachePath := filepath.Join(tmpDir, "cache.json")
	
	mustMkdir(t, srcDir)
	repoPath := filepath.Join(srcDir, "repo1")
	mustMkdir(t, repoPath)
	mustMkdir(t, filepath.Join(repoPath, ".git"))

	cfg := &config.DiscoveryConfig{
		SearchPaths: []string{srcDir},
		MaxDepth:    2,
		CachePath:   cachePath,
	}

	engine := NewEngine(cfg)

	// First call - should scan
	projects, err := engine.GetProjects(false)
	if err != nil {
		t.Fatalf("GetProjects failed: %v", err)
	}
	if len(projects) != 1 {
		t.Errorf("Expected 1 project, got %d", len(projects))
	}

	// Verify cache created
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		t.Error("Cache file was not created")
	}

	// Modify cache to fake a stale entry or different entry
	cache := NewCache(cachePath)
	_ = cache.Load()
	cache.Projects = []Project{{Name: "fake", Path: "/fake"}}
	_ = cache.Save()

	// Second call - should load from cache
	projects, err = engine.GetProjects(false)
	if err != nil {
		t.Fatalf("GetProjects failed: %v", err)
	}
	if len(projects) != 1 || projects[0].Name != "fake" {
		t.Error("Expected to load from cache")
	}

	// Force refresh
	projects, err = engine.GetProjects(true)
	if err != nil {
		t.Fatalf("GetProjects failed: %v", err)
	}
	if len(projects) != 1 || projects[0].Name != "repo1" {
		t.Error("Expected to rescan on force refresh")
	}
}

func TestEngine_ExpiredCache(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "cache.json")
	
	cfg := &config.DiscoveryConfig{
		SearchPaths: []string{tmpDir}, // Empty scan
		CachePath:   cachePath,
	}

	// Create an expired cache manually to avoid Save() updating the timestamp
	cacheData := `{"projects": [{"Name": "old", "Path": "/old", "Type": ""}], "last_scanned": "2020-01-01T00:00:00Z"}`
	if err := os.WriteFile(cachePath, []byte(cacheData), 0644); err != nil {
		t.Fatalf("Failed to write cache file: %v", err)
	}

	engine := NewEngine(cfg)
	projects, err := engine.GetProjects(false)
	if err != nil {
		t.Fatalf("GetProjects failed: %v", err)
	}

	// Should have re-scanned (and found nothing)
	if len(projects) != 0 {
		t.Errorf("Expected 0 projects (rescan), got %d (likely from cache)", len(projects))
	}
}
