package discovery

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Cache handles persistence of discovered projects
type Cache struct {
	Path        string    `json:"-"`
	Projects    []Project `json:"projects"`
	LastScanned time.Time `json:"last_scanned"`
}

// NewCache creates a new cache instance
func NewCache(path string) *Cache {
	return &Cache{
		Path: path,
	}
}

// Load reads the cache from disk
func (c *Cache) Load() error {
	data, err := os.ReadFile(c.Path)
	if os.IsNotExist(err) {
		return nil // Not an error, just empty
	}
	if err != nil {
		return err
	}

	return json.Unmarshal(data, c)
}

// Save writes the cache to disk
func (c *Cache) Save() error {
	if err := os.MkdirAll(filepath.Dir(c.Path), 0755); err != nil {
		return err
	}

	c.LastScanned = time.Now()
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(c.Path, data, 0644)
}

// Update updates the cache with new projects
func (c *Cache) Update(projects []Project) {
	c.Projects = projects
}
