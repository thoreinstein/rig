package bootstrap

import (
	"reflect"
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

func TestParsePluginFlags(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want map[string]string
	}{
		{
			name: "mixed flags and positionals",
			args: []string{"--foo=bar", "pos1", "-b", "--baz", "qux"},
			want: map[string]string{
				"foo": "bar",
				"b":   "true",
				"baz": "qux",
			},
		},
		{
			name: "boolean flags",
			args: []string{"--bool", "-v"},
			want: map[string]string{
				"bool": "true",
				"v":    "true",
			},
		},
		{
			name: "no flags",
			args: []string{"pos1", "pos2"},
			want: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParsePluginFlags(tt.args)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParsePluginFlags() = %v, want %v", got, tt.want)
			}
		})
	}
}
