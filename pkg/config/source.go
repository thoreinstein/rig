package config

import "fmt"

// ConfigSource defines the origin tier of a configuration value
type ConfigSource int

const (
	SourceDefault ConfigSource = iota
	SourceUser
	SourceProject
	SourceEnv
	SourceFlag
	SourceKeychain
)

func (s ConfigSource) String() string {
	switch s {
	case SourceDefault:
		return "Default"
	case SourceUser:
		return "User"
	case SourceProject:
		return "Project"
	case SourceEnv:
		return "Env"
	case SourceFlag:
		return "Flag"
	case SourceKeychain:
		return "Keychain"
	default:
		return "Unknown"
	}
}

// SourceEntry records a value and its origin
type SourceEntry struct {
	Value    interface{} // The final resolved/hydrated value
	Source   ConfigSource
	File     string      // Optional: specific file path for User/Project sources
	RawValue interface{} // The raw value before any resolution or hydration (e.g. keychain:// URI)
}

// SourceMap maps dotted config keys to their provenance
type SourceMap map[string]SourceEntry

// Get returns the source string for a key, including file info if available
func (m SourceMap) Get(key string) string {
	entry, ok := m[key]
	if !ok {
		return "Unknown"
	}
	if entry.File != "" {
		return fmt.Sprintf("%s: %s", entry.Source, entry.File)
	}
	return entry.Source.String()
}

// DiscoveryEvent records a significant event in the configuration resolution process
type DiscoveryEvent struct {
	Tier    string `json:"tier"` // "default", "user", "project", "env", "keychain"
	Message string `json:"message"`
	File    string `json:"file,omitempty"`
}
