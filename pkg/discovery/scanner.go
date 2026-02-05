package discovery

import (
	"os"
	"path/filepath"
	"time"

	"thoreinstein.com/rig/pkg/git"
)

// Scanner scans directories for git repositories
type Scanner struct {
	MaxDepth    int
	SearchPaths []string
	Exclusions  map[string]bool
}

// NewScanner creates a new scanner with default exclusions
func NewScanner(paths []string, depth int) *Scanner {
	return &Scanner{
		MaxDepth:    depth,
		SearchPaths: paths,
		Exclusions: map[string]bool{
			"node_modules": true,
			"vendor":       true,
			".terraform":   true,
			".git":         true,
			".idea":        true,
			".vscode":      true,
		},
	}
}

// Scan performs the scan and returns the result
func (s *Scanner) Scan() (*Result, error) {
	start := time.Now()
	var projects []Project
	scanned := 0
	visited := make(map[string]bool)

	for _, root := range s.SearchPaths {
		// Resolve symlinks for root
		realRoot, err := filepath.EvalSymlinks(root)
		if err != nil {
			continue // Skip invalid roots
		}

		_ = filepath.WalkDir(realRoot, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil // Ignore permission errors
			}

			scanned++

			// Check exclusions
			if d.IsDir() && s.Exclusions[d.Name()] {
				return filepath.SkipDir
			}

			// Check depth
			rel, err := filepath.Rel(realRoot, path)
			if err != nil {
				return nil
			}
			depth := 0
			if rel != "." {
				depth = 1
				for _, c := range rel {
					if c == os.PathSeparator {
						depth++
					}
				}
			}
			
			if depth > s.MaxDepth {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			// Resolve symlinks
			realPath, err := filepath.EvalSymlinks(path)
			if err != nil {
				return nil
			}

			if visited[realPath] {
				return nil // Avoid cycles/duplicates
			}
			visited[realPath] = true

			// Check if it's a git repo
			if git.IsGitRepo(path) {
				projects = append(projects, Project{
					Name: filepath.Base(path),
					Path: path,
					Type: "standard",
				})
			}

			return nil
		})
	}

	return &Result{
		Projects: projects,
		Scanned:  scanned,
		Duration: time.Since(start),
	}, nil
}
