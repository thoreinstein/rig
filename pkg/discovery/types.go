package discovery

import "time"

// Project represents a discovered git repository
type Project struct {
	Name string // Basename of the directory
	Path string // Absolute path to the repository
	Type string // "standard" or "bare"
}

// Result represents the result of a discovery scan
type Result struct {
	Projects []Project
	Scanned  int           // Number of directories scanned
	Duration time.Duration // Time taken to scan
}
