package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

// CollectProjectConfigs walks from gitRoot to cwd collecting .rig.toml paths (least-specific to most-specific)
func CollectProjectConfigs(gitRoot, cwd string) []string {
	if gitRoot == "" {
		if cwd == "" {
			cwd, _ = os.Getwd()
		}
		if cwd != "" {
			return []string{filepath.Join(cwd, ".rig.toml")}
		}
		return nil
	}

	// Ensure paths are absolute and cleaned
	gitRoot = filepath.Clean(gitRoot)
	cwd = filepath.Clean(cwd)

	// If cwd is not within gitRoot, just return gitRoot's config if it exists
	rel, err := filepath.Rel(gitRoot, cwd)
	if err != nil || strings.HasPrefix(rel, "..") {
		return []string{filepath.Join(gitRoot, ".rig.toml"), filepath.Join(cwd, ".rig.toml")}
	}

	var configs []string
	current := gitRoot
	parts := strings.Split(rel, string(filepath.Separator))

	// Add root config
	configs = append(configs, filepath.Join(current, ".rig.toml"))

	// Add intermediate and leaf configs
	for _, part := range parts {
		if part == "." || part == "" {
			continue
		}
		current = filepath.Join(current, part)
		configs = append(configs, filepath.Join(current, ".rig.toml"))
	}

	return configs
}

// DiffSettings returns flat dotted keys whose values changed between before and after snapshots.
func DiffSettings(before, after map[string]interface{}, prefix string) []string {
	var diffs []string

	for k, v := range after {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}

		oldVal, exists := before[k]
		if !exists {
			// Recurse into new maps to capture leaf keys only — skip
			// section headers (parent map keys) which are not config keys.
			if nextMap, ok := v.(map[string]interface{}); ok {
				diffs = append(diffs, DiffSettings(nil, nextMap, key)...)
			} else {
				diffs = append(diffs, key)
			}
			continue
		}

		// Recurse if both are maps
		beforeMap, ok1 := oldVal.(map[string]interface{})
		afterMap, ok2 := v.(map[string]interface{})
		if ok1 && ok2 {
			diffs = append(diffs, DiffSettings(beforeMap, afterMap, key)...)
			continue
		}

		// Otherwise check for equality
		if !reflect.DeepEqual(oldVal, v) {
			diffs = append(diffs, key)
		}
	}

	return diffs
}
