package bootstrap

import (
	"testing"
)

func TestPreParseGlobalFlags(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantConfig  string
		wantVerbose bool
	}{
		{
			name:        "flags before subcommand",
			args:        []string{"rig", "--config", "myconfig.toml", "-v", "work", "RIG-123"},
			wantConfig:  "myconfig.toml",
			wantVerbose: true,
		},
		{
			name:        "flags after subcommand",
			args:        []string{"rig", "work", "RIG-123", "--config", "other.toml", "-v"},
			wantConfig:  "other.toml",
			wantVerbose: true,
		},
		{
			name:        "shorthand and equals",
			args:        []string{"rig", "hack", "-C=test.toml", "--verbose"},
			wantConfig:  "test.toml",
			wantVerbose: true,
		},
		{
			name:        "respect double dash",
			args:        []string{"rig", "work", "--", "--config", "ignored.toml"},
			wantConfig:  "",
			wantVerbose: false,
		},
		{
			name:        "shorthand attached",
			args:        []string{"rig", "-Cmy.toml"},
			wantConfig:  "my.toml",
			wantVerbose: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotConfig, gotVerbose := PreParseGlobalFlags(tt.args)
			if gotConfig != tt.wantConfig {
				t.Errorf("PreParseGlobalFlags() gotConfig = %v, want %v", gotConfig, tt.wantConfig)
			}
			if gotVerbose != tt.wantVerbose {
				t.Errorf("PreParseGlobalFlags() gotVerbose = %v, want %v", gotVerbose, tt.wantVerbose)
			}
		})
	}
}
