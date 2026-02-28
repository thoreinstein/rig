package orchestration

import (
	"encoding/json"
	"path/filepath"
	"strings"
)

// NodeCapabilities represents the explicitly granted permissions for a workflow node.
type NodeCapabilities struct {
	Workspace      string            `json:"workspace"`
	AllowedPaths   []string          `json:"allowed_paths"`
	NetworkAccess  bool              `json:"network_access"`
	SecretsMapping map[string]string `json:"secrets_mapping"`
}

// ConfigWrapper is used to unmarshal the top-level structure of a Node's config.
type ConfigWrapper struct {
	Capabilities *NodeCapabilities `json:"capabilities"`
	PluginConfig json.RawMessage   `json:"plugin"`
}

// ParseNodeCapabilities extracts the capabilities and the opaque plugin configuration
// from the raw JSON config of a Node. If no capabilities are defined, it returns
// a deny-all default.
func ParseNodeCapabilities(raw json.RawMessage) (*NodeCapabilities, json.RawMessage, error) {
	if len(raw) == 0 {
		return &NodeCapabilities{}, json.RawMessage(`{}`), nil
	}

	var wrapper ConfigWrapper
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		return nil, nil, err
	}

	caps := wrapper.Capabilities
	if caps == nil {
		// Deny-all by default
		caps = &NodeCapabilities{}
	}

	pluginConfig := wrapper.PluginConfig
	if len(pluginConfig) == 0 {
		pluginConfig = json.RawMessage(`{}`)
	}

	return caps, pluginConfig, nil
}

// IsPathAllowed checks if the requested path is within the workspace or any of the allowed paths.
// It uses filepath.Clean and filepath.EvalSymlinks to prevent directory traversal attacks
// (e.g., ../../etc/passwd) and symlink escapes.
func (c *NodeCapabilities) IsPathAllowed(requestedPath string) bool {
	// Must be an absolute path to prevent ambiguity
	if !filepath.IsAbs(requestedPath) {
		return false
	}

	cleanRequested := filepath.Clean(requestedPath)

	// Resolve symlinks so that a symlink inside the workspace pointing outside
	// cannot bypass the prefix check. If the path doesn't exist yet (e.g. a new
	// file being written), we fall through with the cleaned path.
	if resolved, err := filepath.EvalSymlinks(cleanRequested); err == nil {
		cleanRequested = resolved
	}

	// Check workspace
	if c.Workspace != "" {
		cleanWorkspace := filepath.Clean(c.Workspace)
		if resolved, err := filepath.EvalSymlinks(cleanWorkspace); err == nil {
			cleanWorkspace = resolved
		}
		if cleanRequested == cleanWorkspace || strings.HasPrefix(cleanRequested, cleanWorkspace+string(filepath.Separator)) {
			return true
		}
	}

	// Check allowed paths
	for _, allowed := range c.AllowedPaths {
		cleanAllowed := filepath.Clean(allowed)
		if resolved, err := filepath.EvalSymlinks(cleanAllowed); err == nil {
			cleanAllowed = resolved
		}
		if cleanRequested == cleanAllowed || strings.HasPrefix(cleanRequested, cleanAllowed+string(filepath.Separator)) {
			return true
		}
	}

	return false
}
