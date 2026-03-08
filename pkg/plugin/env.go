package plugin

import (
	"os"
	"strings"
)

var defaultEnvAllowList = []string{
	"PATH",
	"HOME",
	"USER",
	"LANG",
	"LC_ALL",
	"LC_CTYPE",
	"TERM",
	"TMPDIR",
	"XDG_RUNTIME_DIR",
	"TZ",
	"SHELL",
}

// buildEnv constructs a sanitized environment for a plugin process.
// It filters os.Environ() against a combined set of allowed variables:
// 1. A hardcoded default "essential" list.
// 2. A global allow-list from Rig configuration.
// 3. A per-plugin allow-list from the plugin's own configuration.
//
// Supports prefix matching if a pattern ends with "*".
func buildEnv(globalAllow, pluginAllow []string) []string {
	allowedPatterns := make([]string, 0, len(defaultEnvAllowList)+len(globalAllow)+len(pluginAllow))
	allowedPatterns = append(allowedPatterns, defaultEnvAllowList...)
	allowedPatterns = append(allowedPatterns, globalAllow...)
	allowedPatterns = append(allowedPatterns, pluginAllow...)

	var result []string
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 0 {
			continue
		}
		key := parts[0]

		if isAllowed(key, allowedPatterns) {
			result = append(result, env)
		}
	}

	return result
}

func isAllowed(key string, patterns []string) bool {
	for _, pattern := range patterns {
		if strings.HasSuffix(pattern, "*") {
			prefix := strings.TrimSuffix(pattern, "*")
			if strings.HasPrefix(key, prefix) {
				return true
			}
		} else if key == pattern {
			return true
		}
	}
	return false
}
