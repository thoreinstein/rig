package project

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/cockroachdb/errors"
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
	// Canonicalize the start path early to improve cache hits for equivalent paths
	// (relative, absolute, or symlink spellings).
	var key string
	if startDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, errors.Wrap(err, "failed to get current working directory")
		}
		key = cwd
	} else {
		abs, err := filepath.Abs(startDir)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to resolve absolute path for %q", startDir)
		}
		key = abs
	}

	// Resolve physical path (handle symlinks) to ensure the key is truly unique
	key, err := filepath.EvalSymlinks(key)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to resolve physical path for %q", startDir)
	}
	key = filepath.Clean(key)

	// Check cache first.
	cacheMu.RLock()
	entry, ok := cache[key]
	cacheMu.RUnlock()

	if ok {
		return entry.ctx, entry.err
	}

	// Cache miss — call Discover.
	// Since we already resolved the physical path, we can pass it directly.
	ctx, err := Discover(key)
	if err != nil {
		return nil, err
	}

	// Cache the successful result under the resolved key.
	cacheMu.Lock()
	cache[key] = &cacheEntry{ctx: ctx}
	cacheMu.Unlock()

	return ctx, nil
}

// ResetCache clears all cached project discovery results.
func ResetCache() {
	cacheMu.Lock()
	cache = make(map[string]*cacheEntry)
	cacheMu.Unlock()
}
