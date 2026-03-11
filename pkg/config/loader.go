package config

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/spf13/viper"

	"thoreinstein.com/rig/pkg/project"
)

// LayeredLoader orchestrates the 5-tier configuration cascade.
// Not safe for concurrent use if SkipGlobalSync is false (default).
type LayeredLoader struct {
	sources    SourceMap
	discovery  []DiscoveryEvent
	verbose    bool
	projectCtx *project.ProjectContext
	userFile   string
	cwd        string
	trustStore *TrustStore
	violations []TrustViolation

	// SkipGlobalSync prevents the loader from updating the global viper singleton.
	// Useful for tests to avoid side effects.
	SkipGlobalSync bool
}

// NewLayeredLoader creates a new loader and resolves necessary paths
func NewLayeredLoader(cfgFile string, verbose bool) (*LayeredLoader, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get current directory")
	}

	projectCtx, _ := project.CachedDiscover(cwd)

	userFile := cfgFile
	if userFile == "" {
		home, err := UserHomeDir()
		if err != nil {
			return nil, errors.Wrap(err, "failed to get home directory")
		}
		userFile = filepath.Join(home, ".config", "rig", "config.toml")
	} else if !strings.HasSuffix(userFile, ".toml") {
		return nil, errors.New("config file must be TOML format (.toml extension)")
	}

	trustStore, err := NewTrustStore()
	if err != nil {
		return nil, errors.Wrap(err, "failed to initialize trust store")
	}

	return &LayeredLoader{
		sources:    make(SourceMap),
		verbose:    verbose,
		projectCtx: projectCtx,
		userFile:   userFile,
		cwd:        cwd,
		trustStore: trustStore,
	}, nil
}

// Load performs the configuration loading and merging
func (l *LayeredLoader) Load() (*Config, error) {
	l.sources = make(SourceMap)
	l.discovery = nil
	l.violations = nil

	// We use a fresh viper instance for the entire resolution process
	// to avoid pollution from previous loads or singleton state.
	v := viper.New()

	// Tier 1: Defaults
	SetDefaults(v)
	defaultSettings := v.AllSettings()
	flatDefaults := flattenSettings(defaultSettings, "")
	l.logDiscovery("default", fmt.Sprintf("Populated %d default keys", len(flatDefaults)))
	for k := range flatDefaults {
		val := v.Get(k)
		l.sources[k] = SourceEntry{Source: SourceDefault, Value: val, RawValue: val}
	}

	// Tier 2: User File
	l.logDiscovery("user", "Looking for user config: "+l.userFile)
	if _, err := os.Stat(l.userFile); err == nil {
		v.SetConfigFile(l.userFile)
		v.SetConfigType("toml")
		if err := v.ReadInConfig(); err != nil {
			return nil, errors.Wrapf(err, "failed to read user config file %q", l.userFile)
		}
		userSettings := v.AllSettings()
		diffs := DiffSettings(defaultSettings, userSettings, "")
		l.logDiscovery("user", fmt.Sprintf("Loaded user config: %s (%d keys overridden)", l.userFile, len(diffs)), l.userFile)
		for _, k := range diffs {
			val := v.Get(k)
			l.sources[k] = SourceEntry{Source: SourceUser, File: l.userFile, Value: val, RawValue: val}
		}
		defaultSettings = userSettings
	} else {
		l.logDiscovery("user", "User config not found: "+l.userFile)
	}

	// Tier 3: Project Cascade (.rig.toml)
	var projectConfigs []string
	// Determine trust status once before iterating project configs.
	// If trustStore is nil (failed to init), treat as untrusted — fail-closed.
	var untrustedProject bool
	if l.projectCtx != nil {
		projectConfigs = CollectProjectConfigs(l.projectCtx.RootPath, l.projectCtx.Origin)
		untrustedProject = l.trustStore == nil || !l.trustStore.IsTrusted(l.projectCtx.RootPath)
		trustStatus := "trusted"
		if untrustedProject {
			trustStatus = "untrusted"
		}
		l.logDiscovery("project", fmt.Sprintf("Project trust: %s (root: %s)", trustStatus, l.projectCtx.RootPath))
	} else {
		projectConfigs = CollectProjectConfigs("", l.cwd)
		untrustedProject = len(projectConfigs) > 0
		l.logDiscovery("project", "No project context discovered")
	}

	for _, pc := range projectConfigs {
		l.logDiscovery("project", "Searching: "+pc)
		if _, err := os.Stat(pc); err == nil {
			localViper := viper.New()
			localViper.SetConfigFile(pc)
			localViper.SetConfigType("toml")
			if err := localViper.ReadInConfig(); err != nil {
				return nil, errors.Wrapf(err, "failed to read project config file %q", pc)
			}

			projectSettings := localViper.AllSettings()
			diffs := DiffSettings(defaultSettings, projectSettings, "")

			// Catch immutable keys even when their value matches the current default
			for immKey := range immutableKeys {
				if localViper.IsSet(immKey) && !slices.Contains(diffs, immKey) {
					diffs = append(diffs, immKey)
				}
			}

			var immutableBlocked int
			for _, key := range diffs {
				// 1. Check Immutability (Always Blocked)
				if IsImmutable(key) {
					immutableBlocked++
					l.violations = append(l.violations, TrustViolation{
						Key:            key,
						File:           pc,
						Reason:         ViolationImmutable,
						AttemptedValue: localViper.Get(key),
					})
					deleteFlatKey(projectSettings, key)
					continue
				}

				// 2. Check Project Trust (Applied with warning)
				if untrustedProject {
					l.violations = append(l.violations, TrustViolation{
						Key:            key,
						File:           pc,
						Reason:         ViolationUntrustedProject,
						AttemptedValue: localViper.Get(key),
					})
				}
			}

			// Merge into our local viper
			if err := v.MergeConfigMap(projectSettings); err != nil {
				return nil, errors.Wrapf(err, "failed to merge project config %q", pc)
			}

			// Track provenance
			currentSettings := v.AllSettings()
			newDiffs := DiffSettings(defaultSettings, currentSettings, "")
			l.logDiscovery("project", fmt.Sprintf("Using project config: %s (%d keys overridden, %d immutable blocked)", pc, len(newDiffs), immutableBlocked), pc)

			for _, k := range newDiffs {
				val := v.Get(k)
				l.sources[k] = SourceEntry{Source: SourceProject, File: pc, Value: val, RawValue: val}
			}
			defaultSettings = currentSettings
		} else {
			l.logDiscovery("project", "Not found: "+pc)
		}
	}

	// Tier 4: Environment Variables
	// AutomaticEnv() is lazy — it sets a flag so future Get() calls check env,
	// but AllSettings() won't reflect env values. We must explicitly check
	// os.Getenv for each known config key to detect env overrides.
	v.SetEnvPrefix("RIG")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Check each known key for an env override.
	// We use os.LookupEnv (not os.Getenv) so that explicitly-set-but-empty
	// env vars (e.g. RIG_GITHUB_TOKEN="") are correctly attributed to Env.
	knownKeys := flattenSettings(v.AllSettings(), "")
	for key := range knownKeys {
		envKey := "RIG_" + strings.ToUpper(strings.ReplaceAll(key, ".", "_"))
		if _, ok := os.LookupEnv(envKey); ok {
			l.logDiscovery("env", fmt.Sprintf("Env override: %s → %s", key, envKey))
			val := v.Get(key)
			l.sources[key] = SourceEntry{Source: SourceEnv, Value: val, RawValue: val}
		}
	}

	// Tier 5: Flags (Attribution deferred to Command integration)

	// Tier 6: Secret Hydration (Keychain)
	l.logDiscovery("keychain", "Resolving keychain values")
	settings := v.AllSettings()

	// Defer hydration of the 'plugins' section.
	// This ensures that secrets stored in the keychain are not automatically
	// included in the configuration JSON sent to plugins during Handshake.
	// Instead, plugins must use the host's SecretService to resolve them.
	pluginsRaw, hasPlugins := settings["plugins"]
	delete(settings, "plugins")

	if err := ResolveKeychainValues(settings, l.sources, l.verbose); err != nil {
		return nil, errors.Wrap(err, "failed to resolve keychain secrets")
	}

	// Restore plugins (un-hydrated)
	if hasPlugins {
		settings["plugins"] = pluginsRaw
	}

	// Merge resolved secrets back into our local viper
	if err := v.MergeConfigMap(settings); err != nil {
		return nil, errors.Wrap(err, "failed to merge resolved secrets")
	}

	// Finalize
	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal config")
	}

	if err := expandPaths(cfg); err != nil {
		return nil, errors.Wrap(err, "failed to expand paths")
	}

	if err := cfg.Validate(); err != nil {
		return nil, errors.Wrap(err, "config validation failed")
	}

	// Sync local viper back to global viper for the rest of the application.
	// This ensures that existing calls to viper.Get() work as expected.
	//
	// NOTE: The global viper state intentionally contains unresolved
	// `keychain://` URIs under `plugins.*` keys. Plugin configurations are
	// NOT hydrated here — plugins must use the host's SecretService (via
	// gRPC) to resolve secrets at runtime. This prevents accidental leakage
	// of plaintext secrets through the handshake config JSON.
	if !l.SkipGlobalSync {
		if err := viper.MergeConfigMap(v.AllSettings()); err != nil {
			return nil, errors.Wrap(err, "failed to sync to global viper")
		}
	}

	return cfg, nil
}

// Sources returns the provenance map
func (l *LayeredLoader) Sources() SourceMap {
	return l.sources
}

// UserFile returns the resolved user config file path
func (l *LayeredLoader) UserFile() string {
	return l.userFile
}

// Violations returns the trust violations discovered during loading.
func (l *LayeredLoader) Violations() []TrustViolation {
	return l.violations
}

// DiscoveryLog returns the trace of events recorded during configuration loading.
func (l *LayeredLoader) DiscoveryLog() []DiscoveryEvent {
	return l.discovery
}

func (l *LayeredLoader) logDiscovery(tier, msg string, file ...string) {
	event := DiscoveryEvent{
		Tier:    tier,
		Message: msg,
	}
	if len(file) > 0 {
		event.File = file[0]
	}
	l.discovery = append(l.discovery, event)

	if l.verbose {
		// "Using project config: ..." is printed without a tier prefix for
		// backward-compatibility with the pre-discovery verbose output.
		if tier == "project" && strings.Contains(msg, "Using project config") {
			fmt.Fprintln(os.Stderr, msg)
		} else {
			fmt.Fprintf(os.Stderr, "[%s] %s\n", tier, msg)
		}
	}
}

// deleteFlatKey removes a dotted key from a nested map.
func deleteFlatKey(m map[string]interface{}, key string) {
	parts := strings.Split(key, ".")
	current := m
	for i := range len(parts) - 1 {
		next, ok := current[parts[i]].(map[string]interface{})
		if !ok {
			return
		}
		current = next
	}
	delete(current, parts[len(parts)-1])
}

// flattenSettings is a helper for initial default population
func flattenSettings(m map[string]interface{}, prefix string) map[string]struct{} {
	res := make(map[string]struct{})
	for k, v := range m {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		if sub, ok := v.(map[string]interface{}); ok {
			for sk := range flattenSettings(sub, key) {
				res[sk] = struct{}{}
			}
		} else {
			res[key] = struct{}{}
		}
	}
	return res
}

// GetGitRoot finds the root of the current git repository.
//
// Deprecated: Use project.Discover or project.CachedDiscover instead.
// This is maintained for legacy support and returns ("", nil) on no-context to match old behavior.
func GetGitRoot(cwd string) (string, error) {
	ctx, err := project.Discover(cwd)
	if err != nil {
		if project.IsNoProjectContext(err) {
			return "", nil
		}
		return "", err
	}
	return ctx.Markers[project.MarkerGit], nil
}
