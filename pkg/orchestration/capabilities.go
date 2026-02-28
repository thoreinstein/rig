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

// IOType defines the expected data type for a node input or output.
type IOType string

const (
	IOTypeString  IOType = "string"
	IOTypeNumber  IOType = "number"
	IOTypeBoolean IOType = "boolean"
	IOTypeObject  IOType = "object"
	IOTypeArray   IOType = "array"
)

// NodeIOSchema defines the static inputs and outputs for a node.
type NodeIOSchema struct {
	Inputs  map[string]IOType `json:"inputs"`
	Outputs map[string]IOType `json:"outputs"`
}

// IsValid reports whether t is a recognized IOType constant.
func (t IOType) IsValid() bool {
	switch t {
	case IOTypeString, IOTypeNumber, IOTypeBoolean, IOTypeObject, IOTypeArray:
		return true
	default:
		return false
	}
}

// ParseNodeConfig extracts the capabilities, IO schema, and the opaque plugin
// configuration from the raw JSON config of a Node. It supports both the
// explicit wrapper format and the legacy flat format.
func ParseNodeConfig(raw json.RawMessage) (*NodeCapabilities, *NodeIOSchema, json.RawMessage, error) {
	if len(raw) == 0 {
		return &NodeCapabilities{}, nil, json.RawMessage(`{}`), nil
	}

	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(raw, &rawMap); err != nil {
		return nil, nil, nil, err
	}

	caps := &NodeCapabilities{}
	var io *NodeIOSchema
	var pluginConfig json.RawMessage

	// We determine if this is the explicit wrapper format. It is a wrapper if:
	// 1. The "plugin" key is explicitly present.
	// 2. The "capabilities" key is present WITHOUT other top-level keys
	//    (except "io") that would imply a legacy flat plugin config.
	isWrapper := false
	if _, hasPlugin := rawMap["plugin"]; hasPlugin {
		isWrapper = true
	} else if _, hasCaps := rawMap["capabilities"]; hasCaps {
		hasOtherKeys := false
		for k := range rawMap {
			if k != "capabilities" && k != "io" {
				hasOtherKeys = true
				break
			}
		}
		if !hasOtherKeys {
			isWrapper = true
		}
	}

	if isWrapper {
		// Wrapper format detected
		if capRaw, ok := rawMap["capabilities"]; ok && len(capRaw) > 0 && string(capRaw) != "null" {
			if err := json.Unmarshal(capRaw, caps); err != nil {
				return nil, nil, nil, err
			}
		}

		if ioRaw, ok := rawMap["io"]; ok && len(ioRaw) > 0 && string(ioRaw) != "null" {
			io = &NodeIOSchema{}
			if err := json.Unmarshal(ioRaw, io); err != nil {
				return nil, nil, nil, err
			}
		}

		pluginRaw, ok := rawMap["plugin"]
		if ok && string(pluginRaw) != "null" && len(pluginRaw) > 0 {
			pluginConfig = pluginRaw
		} else {
			pluginConfig = json.RawMessage(`{}`)
		}
	} else {
		// Legacy format: treat top-level JSON entirely as plugin config.
		// Host capabilities and IO remain empty.
		if len(rawMap) == 0 {
			pluginConfig = json.RawMessage(`{}`)
		} else {
			repacked, err := json.Marshal(rawMap)
			if err != nil {
				return nil, nil, nil, err
			}
			pluginConfig = repacked
		}
	}

	return caps, io, pluginConfig, nil
}

// ParseNodeCapabilities extracts the capabilities and the opaque plugin configuration
// from the raw JSON config of a Node. If no capabilities are defined, it returns
// a deny-all default.
//
// This delegates to ParseNodeConfig and discards the IO schema.
func ParseNodeCapabilities(raw json.RawMessage) (*NodeCapabilities, json.RawMessage, error) {
	caps, _, pluginCfg, err := ParseNodeConfig(raw)
	return caps, pluginCfg, err
}

// resolveSymlinks safely evaluates symlinks for the longest existing prefix of the path.
// This is necessary because filepath.EvalSymlinks fails if the target file doesn't exist yet
// (e.g. during a WriteFile operation), which could allow a symlinked parent directory to escape.
func resolveSymlinks(p string) string {
	original := p
	var suffix string

	for {
		resolved, err := filepath.EvalSymlinks(p)
		if err == nil {
			if suffix != "" {
				return filepath.Join(resolved, suffix)
			}
			return resolved
		}

		parent := filepath.Dir(p)
		if parent == p {
			break
		}

		if suffix == "" {
			suffix = filepath.Base(p)
		} else {
			suffix = filepath.Join(filepath.Base(p), suffix)
		}
		p = parent
	}

	return original
}

// IsPathAllowed checks if the requested path is within the workspace or any of the allowed paths.
// It uses filepath.Clean and resolveSymlinks to prevent directory traversal attacks
// (e.g., ../../etc/passwd) and symlink escapes, even for files that do not exist yet.
func (c *NodeCapabilities) IsPathAllowed(requestedPath string) bool {
	// Must be an absolute path to prevent ambiguity
	if !filepath.IsAbs(requestedPath) {
		return false
	}

	cleanRequested := filepath.Clean(requestedPath)

	// Resolve symlinks so that a symlink inside the workspace pointing outside
	// cannot bypass the prefix check. If the path doesn't exist yet (e.g. a new
	// file being written), we evaluate symlinks of the longest existing parent directory.
	cleanRequested = resolveSymlinks(cleanRequested)

	// Check workspace
	if c.Workspace != "" {
		cleanWorkspace := filepath.Clean(c.Workspace)
		cleanWorkspace = resolveSymlinks(cleanWorkspace)
		if cleanRequested == cleanWorkspace || strings.HasPrefix(cleanRequested, cleanWorkspace+string(filepath.Separator)) {
			return true
		}
	}

	// Check allowed paths
	for _, allowed := range c.AllowedPaths {
		cleanAllowed := filepath.Clean(allowed)
		cleanAllowed = resolveSymlinks(cleanAllowed)
		if cleanRequested == cleanAllowed || strings.HasPrefix(cleanRequested, cleanAllowed+string(filepath.Separator)) {
			return true
		}
	}

	return false
}
