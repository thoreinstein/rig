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

// envDenyList contains environment variables that must never be passed to plugins,
// regardless of allow-list configuration. Prevents accidental credential leakage.
var envDenyList = map[string]struct{}{
	"AWS_SECRET_ACCESS_KEY":     {},
	"AWS_SESSION_TOKEN":         {},
	"GITHUB_TOKEN":              {},
	"GH_TOKEN":                  {},
	"GITLAB_TOKEN":              {},
	"SSH_AUTH_SOCK":             {},
	"GPG_AGENT_INFO":            {},
	"NPM_TOKEN":                 {},
	"HOMEBREW_GITHUB_API_TOKEN": {},
}

// buildEnv constructs a sanitized environment for a plugin process.
// It filters os.Environ() against a combined set of allowed variables:
// 1. A hardcoded default "essential" list.
// 2. A global allow-list from Rig configuration.
// 3. A per-plugin allow-list from the plugin's own configuration.
//
// Supports prefix matching if a pattern ends with "*".
//
// Complexity: O(n*p) where n = len(os.Environ()) and p = number of prefix
// patterns. Exact-match keys use a map for O(1) lookups. In practice both n
// and p are small (tens of items), so this is effectively linear.
func buildEnv(globalAllow, pluginAllow []string) []string {
	allPatterns := make([]string, 0, len(defaultEnvAllowList)+len(globalAllow)+len(pluginAllow))
	allPatterns = append(allPatterns, defaultEnvAllowList...)
	allPatterns = append(allPatterns, globalAllow...)
	allPatterns = append(allPatterns, pluginAllow...)

	// Split into exact-match set and prefix slice for O(1) exact lookups.
	exact := make(map[string]struct{}, len(allPatterns))
	var prefixes []string
	for _, p := range allPatterns {
		if prefix, ok := strings.CutSuffix(p, "*"); ok {
			if prefix == "" {
				continue // bare "*" would expose entire environment
			}
			prefixes = append(prefixes, prefix)
		} else {
			exact[p] = struct{}{}
		}
	}

	environ := os.Environ()
	result := make([]string, 0, len(environ))
	for _, env := range environ {
		key, _, ok := strings.Cut(env, "=")
		if !ok {
			continue
		}

		// Deny-list takes precedence over all allow-lists.
		if _, denied := envDenyList[key]; denied {
			continue
		}

		if _, ok := exact[key]; ok {
			result = append(result, env)
			continue
		}

		for _, prefix := range prefixes {
			if strings.HasPrefix(key, prefix) {
				result = append(result, env)
				break
			}
		}
	}

	return result
}
