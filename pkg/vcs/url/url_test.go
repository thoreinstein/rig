package url

import (
	"strings"
	"testing"
)

func TestParseGitHubURL_SSH(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *RepoURL
		wantErr bool
	}{
		{
			name:  "SSH with .git suffix",
			input: "git@github.com:thoreinstein/rig.git",
			want: &RepoURL{
				Original:  "git@github.com:thoreinstein/rig.git",
				Canonical: "git@github.com:thoreinstein/rig.git",
				Protocol:  "ssh",
				Owner:     "thoreinstein",
				Repo:      "rig",
			},
		},
		{
			name:  "SSH without .git suffix",
			input: "git@github.com:thoreinstein/rig",
			want: &RepoURL{
				Original:  "git@github.com:thoreinstein/rig",
				Canonical: "git@github.com:thoreinstein/rig.git",
				Protocol:  "ssh",
				Owner:     "thoreinstein",
				Repo:      "rig",
			},
		},
		{
			name:  "SSH with dashes and underscores",
			input: "git@github.com:my-org/my_repo.git",
			want: &RepoURL{
				Original:  "git@github.com:my-org/my_repo.git",
				Canonical: "git@github.com:my-org/my_repo.git",
				Protocol:  "ssh",
				Owner:     "my-org",
				Repo:      "my_repo",
			},
		},
		{
			name:  "SSH with dots in repo name",
			input: "git@github.com:owner/repo.name.git",
			want: &RepoURL{
				Original:  "git@github.com:owner/repo.name.git",
				Canonical: "git@github.com:owner/repo.name.git",
				Protocol:  "ssh",
				Owner:     "owner",
				Repo:      "repo.name",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseGitHubURL(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseGitHubURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				assertRepoURLEqual(t, got, tt.want)
			}
		})
	}
}

func TestParseGitHubURL_HTTPS(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *RepoURL
		wantErr bool
	}{
		{
			name:  "HTTPS with .git suffix",
			input: "https://github.com/thoreinstein/rig.git",
			want: &RepoURL{
				Original:  "https://github.com/thoreinstein/rig.git",
				Canonical: "https://github.com/thoreinstein/rig.git",
				Protocol:  "https",
				Owner:     "thoreinstein",
				Repo:      "rig",
			},
		},
		{
			name:  "HTTPS without .git suffix",
			input: "https://github.com/thoreinstein/rig",
			want: &RepoURL{
				Original:  "https://github.com/thoreinstein/rig",
				Canonical: "https://github.com/thoreinstein/rig.git",
				Protocol:  "https",
				Owner:     "thoreinstein",
				Repo:      "rig",
			},
		},
		{
			name:  "HTTPS with dashes",
			input: "https://github.com/my-org/my-repo",
			want: &RepoURL{
				Original:  "https://github.com/my-org/my-repo",
				Canonical: "https://github.com/my-org/my-repo.git",
				Protocol:  "https",
				Owner:     "my-org",
				Repo:      "my-repo",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseGitHubURL(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseGitHubURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				assertRepoURLEqual(t, got, tt.want)
			}
		})
	}
}

func TestParseGitHubURL_Shorthand(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *RepoURL
		wantErr bool
	}{
		{
			name:  "shorthand basic",
			input: "github.com/thoreinstein/rig",
			want: &RepoURL{
				Original:  "github.com/thoreinstein/rig",
				Canonical: "git@github.com:thoreinstein/rig.git",
				Protocol:  "ssh",
				Owner:     "thoreinstein",
				Repo:      "rig",
			},
		},
		{
			name:  "shorthand with .git",
			input: "github.com/owner/repo.git",
			want: &RepoURL{
				Original:  "github.com/owner/repo.git",
				Canonical: "git@github.com:owner/repo.git",
				Protocol:  "ssh",
				Owner:     "owner",
				Repo:      "repo",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseGitHubURL(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseGitHubURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				assertRepoURLEqual(t, got, tt.want)
			}
		})
	}
}

func TestParseGitHubURL_OwnerRepoShorthand(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *RepoURL
		wantErr bool
	}{
		{
			name:  "owner/repo shorthand",
			input: "thoreinstein/rig",
			want: &RepoURL{
				Original:  "thoreinstein/rig",
				Canonical: "git@github.com:thoreinstein/rig.git",
				Protocol:  "ssh",
				Owner:     "thoreinstein",
				Repo:      "rig",
			},
		},
		{
			name:  "owner/repo shorthand with .git",
			input: "owner/repo.git",
			want: &RepoURL{
				Original:  "owner/repo.git",
				Canonical: "git@github.com:owner/repo.git",
				Protocol:  "ssh",
				Owner:     "owner",
				Repo:      "repo",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseGitHubURL(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseGitHubURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				assertRepoURLEqual(t, got, tt.want)
			}
		})
	}
}

func TestParseGitHubURL_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		errMsgs []string
	}{
		{
			name:    "empty string",
			input:   "",
			errMsgs: []string{"empty URL"},
		},
		{
			name:    "whitespace only",
			input:   "   ",
			errMsgs: []string{"empty URL"},
		},
		{
			name:    "random text",
			input:   "not a valid url",
			errMsgs: []string{"invalid GitHub URL"},
		},
		{
			name:    "gitlab URL",
			input:   "git@gitlab.com:owner/repo.git",
			errMsgs: []string{"invalid GitHub URL"},
		},
		{
			name:    "bitbucket URL",
			input:   "https://bitbucket.org/owner/repo",
			errMsgs: []string{"invalid GitHub URL"},
		},
		{
			name:    "missing repo",
			input:   "git@github.com:owner",
			errMsgs: []string{"invalid GitHub URL"},
		},
		{
			name:    "missing owner",
			input:   "https://github.com/repo",
			errMsgs: []string{"invalid GitHub URL"},
		},
		{
			name:    "invalid characters in owner",
			input:   "git@github.com:ow ner/repo.git",
			errMsgs: []string{"invalid GitHub URL"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseGitHubURL(tt.input)
			if err == nil {
				t.Errorf("ParseGitHubURL(%q) expected error, got %+v", tt.input, got)
				return
			}
			for _, msg := range tt.errMsgs {
				if !strings.Contains(err.Error(), msg) {
					t.Errorf("ParseGitHubURL(%q) error = %q, should contain %q", tt.input, err.Error(), msg)
				}
			}
		})
	}
}

// Helper function to compare RepoURL structs
func assertRepoURLEqual(t *testing.T, got, want *RepoURL) {
	t.Helper()
	if got.Original != want.Original {
		t.Errorf("Original = %q, want %q", got.Original, want.Original)
	}
	if got.Canonical != want.Canonical {
		t.Errorf("Canonical = %q, want %q", got.Canonical, want.Canonical)
	}
	if got.Protocol != want.Protocol {
		t.Errorf("Protocol = %q, want %q", got.Protocol, want.Protocol)
	}
	if got.Owner != want.Owner {
		t.Errorf("Owner = %q, want %q", got.Owner, want.Owner)
	}
	if got.Repo != want.Repo {
		t.Errorf("Repo = %q, want %q", got.Repo, want.Repo)
	}
}
