package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func TestGetVaultSubdir(t *testing.T) {
	// Helper to create config with default vault subdirs
	makeConfig := func(ticketTypes map[string]TicketTypeConfig) *Config {
		return &Config{
			Vault: VaultConfig{
				DefaultSubdir:  "Tickets",
				IncidentSubdir: "Incidents",
				HackSubdir:     "Hacks",
			},
			TicketTypes: ticketTypes,
		}
	}

	tests := []struct {
		name       string
		config     *Config
		ticketType string
		expected   string
	}{
		{
			name:       "default ticket type",
			config:     makeConfig(nil),
			ticketType: "proj",
			expected:   "Tickets",
		},
		{
			name:       "incident ticket type",
			config:     makeConfig(nil),
			ticketType: "incident",
			expected:   "Incidents",
		},
		{
			name:       "hack ticket type",
			config:     makeConfig(nil),
			ticketType: "hack",
			expected:   "Hacks",
		},
		{
			name: "configured ticket type overrides default",
			config: makeConfig(map[string]TicketTypeConfig{
				"proj": {Repo: "main", VaultSubdir: "CustomTickets"},
			}),
			ticketType: "proj",
			expected:   "CustomTickets",
		},
		{
			name: "configured ticket type with empty subdir uses default",
			config: makeConfig(map[string]TicketTypeConfig{
				"proj": {Repo: "main", VaultSubdir: ""},
			}),
			ticketType: "proj",
			expected:   "Tickets",
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

func TestGetRepositoryPath(t *testing.T) {
	config := &Config{
		Repository: RepositoryConfig{
			BasePath: "/home/user/src",
			Owner:    "myorg",
			Name:     "myrepo",
		},
	}

	result := config.GetRepositoryPath()
	expected := "/home/user/src/myorg/myrepo"

	if result != expected {
		t.Errorf("GetRepositoryPath() = %q, want %q", result, expected)
	}
}

func TestGetWorktreePath(t *testing.T) {
	config := &Config{
		Repository: RepositoryConfig{
			BasePath: "/home/user/src",
			Owner:    "myorg",
			Name:     "myrepo",
		},
	}

	result := config.GetWorktreePath("fraas", "FRAAS-123")
	expected := "/home/user/src/myorg/myrepo/fraas/FRAAS-123"

	if result != expected {
		t.Errorf("GetWorktreePath() = %q, want %q", result, expected)
	}
}

func TestGetNotePath(t *testing.T) {
	config := &Config{
		Vault: VaultConfig{
			Path:          "/home/user/vault",
			AreasDir:      "Areas/Work",
			DefaultSubdir: "Tickets",
		},
	}

	result := config.GetNotePath("proj", "PROJ-123")
	// GetNotePath uses GetVaultSubdir which returns DefaultSubdir
	expected := "/home/user/vault/Areas/Work/Tickets/proj/PROJ-123.md"

	if result != expected {
		t.Errorf("GetNotePath() = %q, want %q", result, expected)
	}
}

func TestGetNotePath_WithConfiguredSubdir(t *testing.T) {
	config := &Config{
		Vault: VaultConfig{
			Path:          "/home/user/vault",
			AreasDir:      "Areas/Work",
			DefaultSubdir: "Tickets",
		},
		TicketTypes: map[string]TicketTypeConfig{
			"proj": {VaultSubdir: "CustomTickets"},
		},
	}

	result := config.GetNotePath("proj", "PROJ-123")
	expected := "/home/user/vault/Areas/Work/CustomTickets/proj/PROJ-123.md"

	if result != expected {
		t.Errorf("GetNotePath() = %q, want %q", result, expected)
	}
}

func TestLoad_WithDefaults(t *testing.T) {
	// Reset viper to clean state
	viper.Reset()

	// Don't read any config file - just use defaults
	config, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// Verify defaults are set
	if config.Jira.Enabled != true {
		t.Error("Expected jira.enabled to default to true")
	}
	if config.Jira.CliCommand != "acli" {
		t.Errorf("Expected jira.cli_command to default to 'acli', got %q", config.Jira.CliCommand)
	}
	if config.Repository.BaseBranch != "main" {
		t.Errorf("Expected repository.base_branch to default to 'main', got %q", config.Repository.BaseBranch)
	}
	if config.Vault.TemplatesDir != "templates" {
		t.Errorf("Expected vault.templates_dir to default to 'templates', got %q", config.Vault.TemplatesDir)
	}
}

func TestLoad_WithConfigFile(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()

	configContent := `
vault:
  path: "/test/vault"
  templates_dir: "custom-templates"
  areas_dir: "CustomAreas"
  daily_dir: "CustomDaily"

repository:
  owner: "testowner"
  name: "testrepo"
  base_path: "/test/src"
  base_branch: "develop"

jira:
  enabled: false
  cli_command: "custom-jira"

tmux:
  session_prefix: "test-"
`

	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Reset viper and configure it to read our test file
	viper.Reset()
	viper.SetConfigFile(configPath)
	if err := viper.ReadInConfig(); err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	config, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// Verify custom values
	if config.Vault.Path != "/test/vault" {
		t.Errorf("Vault.Path = %q, want %q", config.Vault.Path, "/test/vault")
	}
	if config.Vault.TemplatesDir != "custom-templates" {
		t.Errorf("Vault.TemplatesDir = %q, want %q", config.Vault.TemplatesDir, "custom-templates")
	}
	if config.Repository.Owner != "testowner" {
		t.Errorf("Repository.Owner = %q, want %q", config.Repository.Owner, "testowner")
	}
	if config.Repository.BaseBranch != "develop" {
		t.Errorf("Repository.BaseBranch = %q, want %q", config.Repository.BaseBranch, "develop")
	}
	if config.Jira.Enabled != false {
		t.Error("Jira.Enabled should be false")
	}
	if config.Jira.CliCommand != "custom-jira" {
		t.Errorf("Jira.CliCommand = %q, want %q", config.Jira.CliCommand, "custom-jira")
	}
	if config.Tmux.SessionPrefix != "test-" {
		t.Errorf("Tmux.SessionPrefix = %q, want %q", config.Tmux.SessionPrefix, "test-")
	}
}

func TestExpandPaths(t *testing.T) {
	homeDir, _ := os.UserHomeDir()

	config := &Config{
		Vault: VaultConfig{
			Path: "~/vault",
		},
		Repository: RepositoryConfig{
			BasePath: "~/src",
		},
		Repositories: map[string]RepositoryConfig{
			"main": {BasePath: "~/repos/main"},
		},
		History: HistoryConfig{
			DatabasePath: "~/.histdb/history.db",
		},
	}

	err := expandPaths(config)
	if err != nil {
		t.Fatalf("expandPaths() error: %v", err)
	}

	expectedVaultPath := filepath.Join(homeDir, "vault")
	if config.Vault.Path != expectedVaultPath {
		t.Errorf("Vault.Path = %q, want %q", config.Vault.Path, expectedVaultPath)
	}

	expectedRepoPath := filepath.Join(homeDir, "src")
	if config.Repository.BasePath != expectedRepoPath {
		t.Errorf("Repository.BasePath = %q, want %q", config.Repository.BasePath, expectedRepoPath)
	}

	expectedMultiRepoPath := filepath.Join(homeDir, "repos/main")
	if config.Repositories["main"].BasePath != expectedMultiRepoPath {
		t.Errorf("Repositories[main].BasePath = %q, want %q", config.Repositories["main"].BasePath, expectedMultiRepoPath)
	}

	expectedHistoryPath := filepath.Join(homeDir, ".histdb/history.db")
	if config.History.DatabasePath != expectedHistoryPath {
		t.Errorf("History.DatabasePath = %q, want %q", config.History.DatabasePath, expectedHistoryPath)
	}
}

func TestExpandPaths_AbsolutePaths(t *testing.T) {
	config := &Config{
		Vault: VaultConfig{
			Path: "/absolute/vault",
		},
		Repository: RepositoryConfig{
			BasePath: "/absolute/src",
		},
		History: HistoryConfig{
			DatabasePath: "/absolute/history.db",
		},
	}

	err := expandPaths(config)
	if err != nil {
		t.Fatalf("expandPaths() error: %v", err)
	}

	// Absolute paths should remain unchanged
	if config.Vault.Path != "/absolute/vault" {
		t.Errorf("Vault.Path = %q, want %q", config.Vault.Path, "/absolute/vault")
	}
	if config.Repository.BasePath != "/absolute/src" {
		t.Errorf("Repository.BasePath = %q, want %q", config.Repository.BasePath, "/absolute/src")
	}
}
