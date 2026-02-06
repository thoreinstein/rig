package discovery

import (
	"time"

	"thoreinstein.com/rig/pkg/config"
)

// Engine orchestrates project discovery and caching
type Engine struct {
	Config  *config.DiscoveryConfig
	Cache   *Cache
	Verbose bool
}

// NewEngine creates a new discovery engine
func NewEngine(cfg *config.DiscoveryConfig, verbose bool) *Engine {
	return &Engine{
		Config:  cfg,
		Cache:   NewCache(cfg.CachePath),
		Verbose: verbose,
	}
}

// GetProjects returns a list of projects, using cache if available and fresh
func (e *Engine) GetProjects(forceRefresh bool) ([]Project, error) {
	if !forceRefresh {
		if err := e.Cache.Load(); err == nil {
			// Check if cache is fresh (e.g., < 24 hours)
			// TODO: Make cache TTL configurable?
			if len(e.Cache.Projects) > 0 && time.Since(e.Cache.LastScanned) < 24*time.Hour {
				return e.Cache.Projects, nil
			}
		}
	}

	return e.Scan()
}

// Scan performs a fresh scan and updates the cache
func (e *Engine) Scan() ([]Project, error) {
	scanner := NewScanner(e.Config.SearchPaths, e.Config.MaxDepth)
	scanner.Verbose = e.Verbose
	result, err := scanner.Scan()
	if err != nil {
		return nil, err
	}

	e.Cache.Update(result.Projects)
	_ = e.Cache.Save() // Best effort save

	return result.Projects, nil
}
