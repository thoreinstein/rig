package git

import (
	"os"
	"path/filepath"
)

// IsGitRepo checks if a path is a git repository
func IsGitRepo(path string) bool {
	// Check for .git directory or file (for worktrees)
	gitPath := filepath.Join(path, ".git")
	if info, err := os.Stat(gitPath); err == nil {
		return info.IsDir() || info.Mode().IsRegular()
	}

	// Also check if it's a bare repo (contains HEAD, config, objects)
	headPath := filepath.Join(path, "HEAD")
	configPath := filepath.Join(path, "config")
	objectsPath := filepath.Join(path, "objects")
	if _, err := os.Stat(headPath); err == nil {
		if _, err := os.Stat(configPath); err == nil {
			if info, err := os.Stat(objectsPath); err == nil && info.IsDir() {
				return true
			}
		}
	}

	return false
}
