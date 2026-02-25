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
	// If empty, get CWD now to use as cache key
	if startDir == "" {
		ctx, err := Discover("")
		if err != nil {
			return nil, err
		}
		// Discover("") already resolved the path and potentially cached it?
		// No, Discover is the raw engine.
		// We use the Origin from the first raw discovery as the true start path.
		startDir = ctx.Origin
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
