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

// DeepMergeMaps recursively merges override into base.
// Maps merge recursively; arrays and scalars are replaced.
func DeepMergeMaps(base, override map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	// Copy base
	for k, v := range base {
		result[k] = v
	}

	// Apply overrides
	for k, v := range override {
		if baseVal, ok := result[k]; ok {
			baseMap, ok1 := baseVal.(map[string]interface{})
			overrideMap, ok2 := v.(map[string]interface{})
			if ok1 && ok2 {
				result[k] = DeepMergeMaps(baseMap, overrideMap)
				continue
			}
		}
		result[k] = v
	}

	return result
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
			diffs = append(diffs, key)
			if nextMap, ok := v.(map[string]interface{}); ok {
				diffs = append(diffs, DiffSettings(nil, nextMap, key)...)
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
		if !reflectEqual(oldVal, v) {
			diffs = append(diffs, key)
		}
	}

	return diffs
}

func reflectEqual(a, b interface{}) bool {
	return reflect.DeepEqual(a, b)
}
