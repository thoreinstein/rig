package project

import (
	"sync"
)

var (
	cacheMu sync.RWMutex
	cache   = make(map[string]*cacheEntry)
)

type cacheEntry struct {
	ctx *ProjectContext
	err error
}

// CachedDiscover is a mutex-guarded version of Discover that caches results
// by the resolved absolute path of the start directory.
// Only successful results are cached to avoid persisting transient errors.
func CachedDiscover(startDir string) (*ProjectContext, error) {
	key := startDir

	// Check cache first (even for empty startDir, using "" as lookup key).
	cacheMu.RLock()
	entry, ok := cache[key]
	cacheMu.RUnlock()

	if ok {
		return entry.ctx, entry.err
	}

	// Cache miss — call Discover.
	ctx, err := Discover(startDir)
	if err != nil {
		return nil, err
	}

	// Cache the successful result under the requested key.
	// When startDir was "", also store under ctx.Origin so that
	// subsequent calls with the resolved path hit the cache.
	cacheMu.Lock()
	cache[key] = &cacheEntry{ctx: ctx}
	if key == "" && ctx.Origin != "" {
		cache[ctx.Origin] = &cacheEntry{ctx: ctx}
	}
	cacheMu.Unlock()

	return ctx, nil
}

// ResetCache clears all cached project discovery results.
func ResetCache() {
	cacheMu.Lock()
	cache = make(map[string]*cacheEntry)
	cacheMu.Unlock()
}
