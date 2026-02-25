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
func CachedDiscover(startDir string) (*ProjectContext, error) {
	// Resolve CWD once so we have a stable cache key.
	if startDir == "" {
		ctx, err := Discover("")
		if err != nil {
			return nil, err
		}
		cacheMu.Lock()
		cache[ctx.Origin] = &cacheEntry{ctx: ctx, err: nil}
		cacheMu.Unlock()
		return ctx, nil
	}

	cacheMu.RLock()
	entry, ok := cache[startDir]
	cacheMu.RUnlock()

	if ok {
		return entry.ctx, entry.err
	}

	// Cache miss
	ctx, err := Discover(startDir)

	cacheMu.Lock()
	cache[startDir] = &cacheEntry{ctx: ctx, err: err}
	cacheMu.Unlock()

	return ctx, err
}

// ResetCache clears all cached project discovery results.
func ResetCache() {
	cacheMu.Lock()
	cache = make(map[string]*cacheEntry)
	cacheMu.Unlock()
}
