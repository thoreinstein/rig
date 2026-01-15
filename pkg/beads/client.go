package beads

// BeadsClient defines the interface for beads issue tracking integration.
// Current implementation wraps the bd CLI tool.
type BeadsClient interface {
	// IsAvailable checks if the bd CLI tool is available in PATH.
	IsAvailable() bool

	// IsBeadsProject checks if the given path contains a beads project
	// (i.e., has a .beads/beads.jsonl file).
	IsBeadsProject(path string) bool

	// Show retrieves issue details for the given issue ID.
	// Returns an error if the issue doesn't exist or the CLI fails.
	Show(id string) (*IssueInfo, error)

	// UpdateStatus updates the status of an issue.
	// Valid statuses are: open, in_progress, closed.
	UpdateStatus(id string, status string) error
}

// Compile-time check that CLIClient implements BeadsClient.
var _ BeadsClient = (*CLIClient)(nil)
