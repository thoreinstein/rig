package beads

import (
	"os"
	"path/filepath"
)

const (
	// BeadsDirName is the name of the beads configuration directory.
	BeadsDirName = ".beads"

	// BeadsFileName is the name of the issue database file.
	BeadsFileName = "issues.jsonl"
)

// IsBeadsProject checks if the given directory contains a beads project.
// A beads project is identified by the presence of .beads/issues.jsonl.
func IsBeadsProject(dir string) bool {
	beadsFile := filepath.Join(dir, BeadsDirName, BeadsFileName)
	info, err := os.Stat(beadsFile)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// FindBeadsRoot walks up the directory tree from startDir looking for a
// beads project root (a directory containing .beads/issues.jsonl).
// Returns the root directory path and true if found, or empty string and
// false if no beads project is found.
func FindBeadsRoot(startDir string) (string, bool) {
	// Clean and convert to absolute path for consistent traversal
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", false
	}

	for {
		if IsBeadsProject(dir) {
			return dir, true
		}

		parent := filepath.Dir(dir)
		// Reached filesystem root
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}
