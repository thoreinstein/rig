package config

import (
	"testing"
)

func TestGetVaultSubdir(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		ticketType  string
		expected    string
	}{
		{
			name: "default jira ticket type",
			config: &Config{},
			ticketType: "fraas",
			expected:   "Jira",
		},
		{
			name: "incident ticket type",
			config: &Config{},
			ticketType: "incident",
			expected:   "Incidents",
		},
		{
			name: "hack ticket type",
			config: &Config{},
			ticketType: "hack",
			expected:   "Hacks",
		},
		{
			name: "configured ticket type overrides default",
			config: &Config{
				TicketTypes: map[string]TicketTypeConfig{
					"fraas": {Repo: "main", VaultSubdir: "CustomJira"},
				},
			},
			ticketType: "fraas",
			expected:   "CustomJira",
		},
		{
			name: "configured ticket type with empty subdir uses default",
			config: &Config{
				TicketTypes: map[string]TicketTypeConfig{
					"fraas": {Repo: "main", VaultSubdir: ""},
				},
			},
			ticketType: "fraas",
			expected:   "Jira",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.GetVaultSubdir(tt.ticketType)
			if result != tt.expected {
				t.Errorf("GetVaultSubdir(%q) = %q, want %q", tt.ticketType, result, tt.expected)
			}
		})
	}
}

func TestIsMultiRepo(t *testing.T) {
	tests := []struct {
		name     string
		config   *Config
		expected bool
	}{
		{
			name:     "empty repositories map",
			config:   &Config{},
			expected: false,
		},
		{
			name: "nil repositories map",
			config: &Config{
				Repositories: nil,
			},
			expected: false,
		},
		{
			name: "single repository in map",
			config: &Config{
				Repositories: map[string]RepositoryConfig{
					"main": {Owner: "test", Name: "repo"},
				},
			},
			expected: true,
		},
		{
			name: "multiple repositories in map",
			config: &Config{
				Repositories: map[string]RepositoryConfig{
					"main":  {Owner: "test", Name: "repo1"},
					"infra": {Owner: "test", Name: "repo2"},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.IsMultiRepo()
			if result != tt.expected {
				t.Errorf("IsMultiRepo() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGetRepoForTicketType(t *testing.T) {
	tests := []struct {
		name         string
		config       *Config
		ticketType   string
		expectedName string
	}{
		{
			name: "legacy single repo config",
			config: &Config{
				Repository: RepositoryConfig{Owner: "legacy", Name: "repo"},
			},
			ticketType:   "fraas",
			expectedName: "repo",
		},
		{
			name: "multi-repo with ticket type mapping",
			config: &Config{
				Repositories: map[string]RepositoryConfig{
					"main":  {Owner: "test", Name: "main-repo"},
					"infra": {Owner: "test", Name: "infra-repo"},
				},
				TicketTypes: map[string]TicketTypeConfig{
					"fraas": {Repo: "main"},
					"ops":   {Repo: "infra"},
				},
			},
			ticketType:   "ops",
			expectedName: "infra-repo",
		},
		{
			name: "multi-repo falls back to default repo",
			config: &Config{
				Repositories: map[string]RepositoryConfig{
					"main":  {Owner: "test", Name: "main-repo"},
					"infra": {Owner: "test", Name: "infra-repo"},
				},
				DefaultRepo: "main",
			},
			ticketType:   "unknown",
			expectedName: "main-repo",
		},
		{
			name: "multi-repo falls back to first repo when no default",
			config: &Config{
				Repositories: map[string]RepositoryConfig{
					"only": {Owner: "test", Name: "only-repo"},
				},
			},
			ticketType:   "unknown",
			expectedName: "only-repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.GetRepoForTicketType(tt.ticketType)
			if result == nil {
				t.Fatal("GetRepoForTicketType() returned nil")
			}
			if result.Name != tt.expectedName {
				t.Errorf("GetRepoForTicketType(%q).Name = %q, want %q", tt.ticketType, result.Name, tt.expectedName)
			}
		})
	}
}

func TestGetRepoByName(t *testing.T) {
	tests := []struct {
		name         string
		config       *Config
		repoName     string
		expectedName string
		expectError  bool
	}{
		{
			name: "multi-repo finds existing repo",
			config: &Config{
				Repositories: map[string]RepositoryConfig{
					"main":  {Owner: "test", Name: "main-repo"},
					"infra": {Owner: "test", Name: "infra-repo"},
				},
			},
			repoName:     "infra",
			expectedName: "infra-repo",
			expectError:  false,
		},
		{
			name: "multi-repo returns error for missing repo",
			config: &Config{
				Repositories: map[string]RepositoryConfig{
					"main": {Owner: "test", Name: "main-repo"},
				},
			},
			repoName:    "nonexistent",
			expectError: true,
		},
		{
			name: "legacy config returns repo for matching name",
			config: &Config{
				Repository: RepositoryConfig{Owner: "test", Name: "legacy-repo"},
			},
			repoName:     "legacy-repo",
			expectedName: "legacy-repo",
			expectError:  false,
		},
		{
			name: "legacy config returns repo for empty name",
			config: &Config{
				Repository: RepositoryConfig{Owner: "test", Name: "legacy-repo"},
			},
			repoName:     "",
			expectedName: "legacy-repo",
			expectError:  false,
		},
		{
			name: "legacy config returns error for non-matching name",
			config: &Config{
				Repository: RepositoryConfig{Owner: "test", Name: "legacy-repo"},
			},
			repoName:    "other-repo",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.config.GetRepoByName(tt.repoName)
			if tt.expectError {
				if err == nil {
					t.Error("GetRepoByName() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("GetRepoByName() unexpected error: %v", err)
			}
			if result.Name != tt.expectedName {
				t.Errorf("GetRepoByName(%q).Name = %q, want %q", tt.repoName, result.Name, tt.expectedName)
			}
		})
	}
}

func TestGetDefaultRepo(t *testing.T) {
	tests := []struct {
		name         string
		config       *Config
		expectedName string
	}{
		{
			name: "legacy config returns Repository",
			config: &Config{
				Repository: RepositoryConfig{Owner: "test", Name: "legacy-repo"},
			},
			expectedName: "legacy-repo",
		},
		{
			name: "multi-repo returns named default",
			config: &Config{
				Repositories: map[string]RepositoryConfig{
					"main":  {Owner: "test", Name: "main-repo"},
					"infra": {Owner: "test", Name: "infra-repo"},
				},
				DefaultRepo: "infra",
			},
			expectedName: "infra-repo",
		},
		{
			name: "multi-repo returns first repo when no default set",
			config: &Config{
				Repositories: map[string]RepositoryConfig{
					"only": {Owner: "test", Name: "only-repo"},
				},
			},
			expectedName: "only-repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.GetDefaultRepo()
			if result == nil {
				t.Fatal("GetDefaultRepo() returned nil")
			}
			if result.Name != tt.expectedName {
				t.Errorf("GetDefaultRepo().Name = %q, want %q", result.Name, tt.expectedName)
			}
		})
	}
}

func TestGetAllRepos(t *testing.T) {
	tests := []struct {
		name          string
		config        *Config
		expectedCount int
		expectedKeys  []string
	}{
		{
			name: "legacy config returns single repo",
			config: &Config{
				Repository: RepositoryConfig{Owner: "test", Name: "legacy-repo"},
			},
			expectedCount: 1,
			expectedKeys:  []string{"legacy-repo"},
		},
		{
			name: "legacy config with empty name uses default key",
			config: &Config{
				Repository: RepositoryConfig{Owner: "test", Name: ""},
			},
			expectedCount: 1,
			expectedKeys:  []string{"default"},
		},
		{
			name: "multi-repo returns all repos",
			config: &Config{
				Repositories: map[string]RepositoryConfig{
					"main":  {Owner: "test", Name: "main-repo"},
					"infra": {Owner: "test", Name: "infra-repo"},
				},
			},
			expectedCount: 2,
			expectedKeys:  []string{"main", "infra"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.GetAllRepos()
			if len(result) != tt.expectedCount {
				t.Errorf("GetAllRepos() returned %d repos, want %d", len(result), tt.expectedCount)
			}
			for _, key := range tt.expectedKeys {
				if _, ok := result[key]; !ok {
					t.Errorf("GetAllRepos() missing expected key %q", key)
				}
			}
		})
	}
}

func TestGetRepositoryPathForRepo(t *testing.T) {
	config := &Config{}
	repo := &RepositoryConfig{
		BasePath: "/home/user/src",
		Owner:    "myorg",
		Name:     "myrepo",
	}

	result := config.GetRepositoryPathForRepo(repo)
	expected := "/home/user/src/myorg/myrepo"

	if result != expected {
		t.Errorf("GetRepositoryPathForRepo() = %q, want %q", result, expected)
	}
}

func TestExpandPath(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectTilde bool
	}{
		{
			name:        "empty path",
			input:       "",
			expectTilde: false,
		},
		{
			name:        "absolute path without tilde",
			input:       "/usr/local/bin",
			expectTilde: false,
		},
		{
			name:        "path with tilde",
			input:       "~/Documents",
			expectTilde: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := expandPath(tt.input)
			if err != nil {
				t.Fatalf("expandPath() error: %v", err)
			}

			if tt.expectTilde {
				if result[0] == '~' {
					t.Errorf("expandPath(%q) still contains ~: %q", tt.input, result)
				}
			} else if tt.input != "" {
				if result != tt.input {
					t.Errorf("expandPath(%q) = %q, want %q", tt.input, result, tt.input)
				}
			}
		})
	}
}
