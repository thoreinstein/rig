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

	// If cwd is not within gitRoot, just return gitRoot and cwd configs.
	// (Handling the rare case where they are disjoint but both have configs).
	rel, err := filepath.Rel(gitRoot, cwd)
	if err != nil || strings.HasPrefix(rel, "..") {
		rootCfg := filepath.Join(gitRoot, ".rig.toml")
		cwdCfg := filepath.Join(cwd, ".rig.toml")
		if rootCfg == cwdCfg {
			return []string{rootCfg}
		}
		return []string{rootCfg, cwdCfg}
	}

	parts := strings.Split(rel, string(filepath.Separator))
	// Pre-allocate: 1 for root + number of segments in rel
	configs := make([]string, 0, len(parts)+1)
	current := gitRoot

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

// DiffSettings returns flat dotted keys whose values were added or changed in 'after'
// compared to 'before'. This is a one-directional diff used for tracking which
// keys were overridden by a higher-priority configuration tier.
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
