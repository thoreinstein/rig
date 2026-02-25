package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/spf13/viper"
)

// LayeredLoader orchestrates the 5-tier configuration cascade
type LayeredLoader struct {
	sources  SourceMap
	verbose  bool
	gitRoot  string
	cwd      string
	userFile string
}

// NewLayeredLoader creates a new loader and resolves necessary paths
func NewLayeredLoader(cfgFile string, verbose bool) (*LayeredLoader, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get current directory")
	}

	gitRoot, _ := GetGitRoot(cwd)

	userFile := cfgFile
	if userFile == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, errors.Wrap(err, "failed to get home directory")
		}
		userFile = filepath.Join(home, ".config", "rig", "config.toml")
	} else if !strings.HasSuffix(userFile, ".toml") {
		return nil, errors.New("config file must be TOML format (.toml extension)")
	}

	return &LayeredLoader{
		sources:  make(SourceMap),
		verbose:  verbose,
		gitRoot:  gitRoot,
		cwd:      cwd,
		userFile: userFile,
	}, nil
}

// Load performs the configuration loading and merging
func (l *LayeredLoader) Load() (*Config, error) {
	l.sources = make(SourceMap)

	// Tier 1: Defaults
	setDefaults()
	defaultSettings := viper.AllSettings()
	for k := range flattenSettings(defaultSettings, "") {
		l.sources[k] = SourceEntry{Source: SourceDefault}
	}

	// Tier 2: User File
	viper.SetConfigFile(l.userFile)
	viper.SetConfigType("toml")
	if _, err := os.Stat(l.userFile); err == nil {
		if err := viper.ReadInConfig(); err != nil {
			return nil, errors.Wrapf(err, "failed to read user config file %q", l.userFile)
		}
		userSettings := viper.AllSettings()
		diffs := DiffSettings(defaultSettings, userSettings, "")
		for _, k := range diffs {
			l.sources[k] = SourceEntry{Source: SourceUser, File: l.userFile}
		}
		defaultSettings = userSettings
	}

	// Tier 3: Project Cascade (.rig.toml)
	projectConfigs := CollectProjectConfigs(l.gitRoot, l.cwd)
	for _, pc := range projectConfigs {
		if _, err := os.Stat(pc); err == nil {
			localViper := viper.New()
			localViper.SetConfigFile(pc)
			localViper.SetConfigType("toml")
			if err := localViper.ReadInConfig(); err != nil {
				return nil, errors.Wrapf(err, "failed to read project config file %q", pc)
			}

			projectSettings := localViper.AllSettings()
			// Merge into main viper
			if err := viper.MergeConfigMap(projectSettings); err != nil {
				return nil, errors.Wrapf(err, "failed to merge project config %q", pc)
			}

			// Track provenance
			currentSettings := viper.AllSettings()
			diffs := DiffSettings(defaultSettings, currentSettings, "")
			for _, k := range diffs {
				l.sources[k] = SourceEntry{Source: SourceProject, File: pc}
			}
			defaultSettings = currentSettings

			if l.verbose {
				fmt.Fprintf(os.Stderr, "Using project config: %s\n", pc)
			}
		}
	}

	// Tier 4: Environment Variables
	envSnapshot := viper.AllSettings()
	viper.SetEnvPrefix("RIG")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// Capture which keys are overridden by ENV
	// Note: AllSettings() only includes ENV vars if the key is already known.
	// This is fine since we have Defaults + User + Project tiers loaded.
	envSettings := viper.AllSettings()
	envDiffs := DiffSettings(envSnapshot, envSettings, "")
	for _, k := range envDiffs {
		l.sources[k] = SourceEntry{Source: SourceEnv}
	}

	// Tier 5: Flags (Attribution deferred to Phase 4 Command integration)
	// We'll handle Flag attribution in the actual command execution context if possible,
	// or here if we bind PFlags. For now, we'll mark it as deferred or handle common global flags.

	// Tier 6: Secret Hydration (Keychain)
	settings := viper.AllSettings()
	if err := ResolveKeychainValues(settings, l.sources, l.verbose); err != nil {
		return nil, errors.Wrap(err, "failed to resolve keychain secrets")
	}
	// Merge resolved secrets back into viper
	if err := viper.MergeConfigMap(settings); err != nil {
		return nil, errors.Wrap(err, "failed to merge resolved secrets")
	}

	// Finalize
	cfg := &Config{}
	if err := viper.Unmarshal(cfg); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal config")
	}

	if err := expandPaths(cfg); err != nil {
		return nil, errors.Wrap(err, "failed to expand paths")
	}

	if err := cfg.Validate(); err != nil {
		return nil, errors.Wrap(err, "config validation failed")
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
func GetGitRoot(cwd string) (string, error) {
	dir := cwd
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}

	for {
		gitPath := filepath.Join(dir, ".git")
		if info, err := os.Stat(gitPath); err == nil {
			if info.IsDir() || info.Mode().IsRegular() {
				return dir, nil
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", nil
		}
		dir = parent
	}
}
