package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/cockroachdb/errors"
)

// immutableKeys defines the set of configuration keys that cannot be overridden
// by project-level configs. This map is package-level and must not be modified
// after initialization. Use IsImmutable() as the public accessor.
var immutableKeys = map[string]bool{
	"github.token":               true,
	"github.client_id":           true,
	"jira.token":                 true,
	"ai.api_key":                 true,
	"ai.gemini_api_key":          true,
	"daemon.enabled":             true,
	"daemon.plugin_idle_timeout": true,
	"daemon.daemon_idle_timeout": true,
	"discovery.search_paths":     true,
}

// IsImmutable returns true if the given dotted key is protected from project-level overrides.
func IsImmutable(key string) bool {
	return immutableKeys[key]
}

// ViolationReason is the type for trust violation reason constants.
type ViolationReason string

const (
	ViolationImmutable        ViolationReason = "immutable"
	ViolationUntrustedProject ViolationReason = "untrusted_project"
)

// TrustViolation represents an attempt by a project config to override a protected key or an untrusted project.
type TrustViolation struct {
	Key            string
	File           string
	Reason         ViolationReason
	AttemptedValue interface{}
}

// TrustEntry represents metadata about a trusted project.
type TrustEntry struct {
	TrustedAt time.Time `json:"trusted_at"`
}

// TrustStore manages the persistent list of trusted project root paths.
type TrustStore struct {
	path    string
	trusted map[string]TrustEntry
}

// NewTrustStore initializes a new trust store at ~/.config/rig/trusted-projects.json.
func NewTrustStore() (*TrustStore, error) {
	home, err := UserHomeDir()
	if err != nil {
		return nil, errors.Wrap(err, "failed to resolve trust store home directory")
	}

	path := filepath.Join(home, ".config", "rig", "trusted-projects.json")
	store := &TrustStore{
		path:    path,
		trusted: make(map[string]TrustEntry),
	}

	if err := store.Load(); err != nil && !os.IsNotExist(err) {
		return nil, errors.Wrap(err, "failed to load trust store")
	}

	return store, nil
}

// Load reads the trust store from disk.
// It refuses to follow symlinks to prevent trust-store hijacking.
func (s *TrustStore) Load() error {
	info, err := os.Lstat(s.path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return errors.Newf("trust store %q is a symlink; refusing to load", s.path)
	}

	data, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}

	if len(data) == 0 {
		return nil
	}

	// Always clear map before loading to ensure a clean state
	s.trusted = make(map[string]TrustEntry)
	return json.Unmarshal(data, &s.trusted)
}

// Save writes the trust store to disk atomically via temp-file + rename.
func (s *TrustStore) Save() error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return errors.Wrap(err, "failed to create trust store directory")
	}

	data, err := json.MarshalIndent(s.trusted, "", "  ")
	if err != nil {
		return errors.Wrap(err, "failed to marshal trust data")
	}

	tmp, err := os.CreateTemp(dir, "trusted-projects-*.json.tmp")
	if err != nil {
		return errors.Wrap(err, "failed to create temp file for trust store")
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return errors.Wrap(err, "failed to write trust data to temp file")
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return errors.Wrap(err, "failed to close temp file")
	}
	if err := os.Chmod(tmpPath, 0600); err != nil {
		os.Remove(tmpPath)
		return errors.Wrap(err, "failed to set permissions on temp file")
	}
	if err := os.Rename(tmpPath, s.path); err != nil {
		os.Remove(tmpPath)
		return errors.Wrap(err, "failed to atomically replace trust store")
	}

	return nil
}

// IsTrusted returns true if the given project root path is trusted.
// The path is canonicalized via filepath.Clean before lookup.
func (s *TrustStore) IsTrusted(projectRoot string) bool {
	if projectRoot == "" {
		return false
	}
	// Normalize path for lookup
	path := filepath.Clean(projectRoot)
	_, ok := s.trusted[path]
	return ok
}

// Add adds a project root path to the trust store.
// The path is canonicalized via filepath.Clean before storage.
func (s *TrustStore) Add(projectRoot string) error {
	if projectRoot == "" {
		return errors.New("project root path cannot be empty")
	}
	path := filepath.Clean(projectRoot)
	s.trusted[path] = TrustEntry{
		TrustedAt: time.Now(),
	}
	return s.Save()
}

// Remove removes a project root path from the trust store.
// The path is canonicalized via filepath.Clean before deletion.
func (s *TrustStore) Remove(projectRoot string) error {
	if projectRoot == "" {
		return errors.New("project root path cannot be empty")
	}
	path := filepath.Clean(projectRoot)
	delete(s.trusted, path)
	return s.Save()
}

// List returns all trusted projects.
func (s *TrustStore) List() map[string]TrustEntry {
	// Return a copy to avoid external modification of internal state
	res := make(map[string]TrustEntry, len(s.trusted))
	for k, v := range s.trusted {
		res[k] = v
	}
	return res
}
