package plugin

import (
	"os"
	"strings"
	"testing"
)

func TestBuildEnv(t *testing.T) {
	// Set up some environment variables for testing
	t.Setenv("RIG_TEST_ALLOWED", "allowed")
	t.Setenv("RIG_TEST_BLOCKED", "blocked")
	t.Setenv("RIG_PREFIX_MATCH_1", "match1")
	t.Setenv("RIG_PREFIX_MATCH_2", "match2")

	// Ensure essential vars are set for default list test
	if os.Getenv("PATH") == "" {
		t.Setenv("PATH", "/usr/bin")
	}
	// Set a deny-listed var so we can test override behavior.
	t.Setenv("SSH_AUTH_SOCK", "/tmp/ssh-test-agent")

	tests := []struct {
		name         string
		globalAllow  []string
		pluginAllow  []string
		wantContains []string
		wantExcludes []string
	}{
		{
			name:         "Default allow-list only",
			globalAllow:  nil,
			pluginAllow:  nil,
			wantContains: []string{"PATH="},
			wantExcludes: []string{"RIG_TEST_ALLOWED=", "RIG_TEST_BLOCKED="},
		},
		{
			name:         "Global allow-list adds vars",
			globalAllow:  []string{"RIG_TEST_ALLOWED"},
			pluginAllow:  nil,
			wantContains: []string{"PATH=", "RIG_TEST_ALLOWED="},
			wantExcludes: []string{"RIG_TEST_BLOCKED="},
		},
		{
			name:         "Plugin allow-list adds vars",
			globalAllow:  nil,
			pluginAllow:  []string{"RIG_TEST_ALLOWED"},
			wantContains: []string{"PATH=", "RIG_TEST_ALLOWED="},
			wantExcludes: []string{"RIG_TEST_BLOCKED="},
		},
		{
			name:         "Prefix matching works",
			globalAllow:  []string{"RIG_PREFIX_*"},
			pluginAllow:  nil,
			wantContains: []string{"RIG_PREFIX_MATCH_1=", "RIG_PREFIX_MATCH_2="},
			wantExcludes: []string{"RIG_TEST_ALLOWED="},
		},
		{
			name:         "Combined lists work",
			globalAllow:  []string{"RIG_TEST_ALLOWED"},
			pluginAllow:  []string{"RIG_PREFIX_*"},
			wantContains: []string{"RIG_TEST_ALLOWED=", "RIG_PREFIX_MATCH_1=", "RIG_PREFIX_MATCH_2="},
			wantExcludes: []string{"RIG_TEST_BLOCKED="},
		},
		{
			name:         "Bare wildcard is ignored",
			globalAllow:  []string{"*"},
			pluginAllow:  nil,
			wantContains: []string{"PATH="},
			wantExcludes: []string{"RIG_TEST_ALLOWED=", "RIG_TEST_BLOCKED="},
		},
		{
			name:         "Deny-list blocks global allow-list",
			globalAllow:  []string{"SSH_AUTH_SOCK"},
			pluginAllow:  nil,
			wantContains: []string{"PATH="},
			wantExcludes: []string{"SSH_AUTH_SOCK="},
		},
		{
			name:         "Per-plugin allow-list overrides deny-list",
			globalAllow:  nil,
			pluginAllow:  []string{"SSH_AUTH_SOCK"},
			wantContains: []string{"PATH=", "SSH_AUTH_SOCK="},
			wantExcludes: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := buildEnv(tc.globalAllow, tc.pluginAllow)

			for _, want := range tc.wantContains {
				found := false
				for _, env := range got {
					if strings.HasPrefix(env, want) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected env to contain %q, but it didn't", want)
				}
			}

			for _, exclude := range tc.wantExcludes {
				for _, env := range got {
					if strings.HasPrefix(env, exclude) {
						t.Errorf("expected env to exclude %q, but it was found", exclude)
					}
				}
			}
		})
	}
}
