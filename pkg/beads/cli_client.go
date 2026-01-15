package beads

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"

	"github.com/cockroachdb/errors"

	rigerrors "thoreinstein.com/rig/pkg/errors"
)

// validCliCommandPattern validates CLI command names to prevent injection.
// Allows alphanumeric characters, hyphens, underscores, and forward slashes (for paths).
var validCliCommandPattern = regexp.MustCompile(`^[a-zA-Z0-9_\-/]+$`)

// validIssueIDPattern validates beads issue IDs.
// Beads IDs typically follow a prefix-number pattern (e.g., beads-123).
var validIssueIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_\-]+$`)

// CLIClient handles beads integration via the bd CLI tool.
type CLIClient struct {
	CliCommand string
	Verbose    bool
}

// NewCLIClient creates a new CLI-based beads client.
// Returns an error if the CLI command contains invalid characters.
func NewCLIClient(cliCommand string, verbose bool) (*CLIClient, error) {
	if cliCommand == "" {
		cliCommand = "bd"
	}

	if !validCliCommandPattern.MatchString(cliCommand) {
		return nil, rigerrors.NewBeadsError("NewCLIClient",
			fmt.Sprintf("invalid CLI command %q: must contain only alphanumeric characters, hyphens, underscores, or forward slashes", cliCommand))
	}

	return &CLIClient{
		CliCommand: cliCommand,
		Verbose:    verbose,
	}, nil
}

// IsAvailable checks if the bd CLI command is available in PATH.
func (c *CLIClient) IsAvailable() bool {
	_, err := exec.LookPath(c.CliCommand)
	return err == nil
}

// IsBeadsProject checks if the given path contains a beads project.
func (c *CLIClient) IsBeadsProject(path string) bool {
	return IsBeadsProject(path)
}

// Show retrieves issue details for the given issue ID.
func (c *CLIClient) Show(id string) (*IssueInfo, error) {
	if !validIssueIDPattern.MatchString(id) {
		return nil, rigerrors.NewBeadsErrorWithIssue("Show", id, "invalid issue ID format")
	}

	if !c.IsAvailable() {
		if c.Verbose {
			fmt.Printf("beads CLI command '%s' not found, skipping issue fetch\n", c.CliCommand)
		}
		return nil, rigerrors.NewBeadsError("Show", "beads CLI command not available")
	}

	if c.Verbose {
		fmt.Printf("fetching beads issue %s\n", id)
	}

	// Execute: bd show <id> --json
	// G204: id validated by validIssueIDPattern regex, CliCommand validated by validCliCommandPattern
	cmd := exec.Command(c.CliCommand, "show", id, "--json") //nolint:gosec
	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, rigerrors.NewBeadsErrorWithCause("Show", id,
				"bd show failed: "+string(exitErr.Stderr), err)
		}
		return nil, rigerrors.NewBeadsErrorWithCause("Show", id, "failed to execute bd show", err)
	}

	var info IssueInfo
	if err := json.Unmarshal(output, &info); err != nil {
		return nil, rigerrors.NewBeadsErrorWithCause("Show", id, "failed to parse JSON output", err)
	}

	if c.Verbose {
		fmt.Printf("fetched beads issue %s: %s\n", id, info.Title)
	}

	return &info, nil
}

// UpdateStatus updates the status of an issue.
func (c *CLIClient) UpdateStatus(id string, status string) error {
	if !validIssueIDPattern.MatchString(id) {
		return rigerrors.NewBeadsErrorWithIssue("UpdateStatus", id, "invalid issue ID format")
	}

	if !IsValidStatus(status) {
		return rigerrors.NewBeadsErrorWithIssue("UpdateStatus", id,
			fmt.Sprintf("invalid status %q: must be one of %v", status, ValidStatuses))
	}

	if !c.IsAvailable() {
		if c.Verbose {
			fmt.Printf("beads CLI command '%s' not found, skipping status update\n", c.CliCommand)
		}
		return rigerrors.NewBeadsError("UpdateStatus", "beads CLI command not available")
	}

	if c.Verbose {
		fmt.Printf("updating beads issue %s status to %s\n", id, status)
	}

	// Execute: bd update <id> --status <status> --json
	// G204: id validated by validIssueIDPattern, status validated by IsValidStatus, CliCommand by validCliCommandPattern
	cmd := exec.Command(c.CliCommand, "update", id, "--status", status, "--json") //nolint:gosec
	output, err := cmd.CombinedOutput()
	if err != nil {
		return rigerrors.NewBeadsErrorWithCause("UpdateStatus", id,
			"bd update failed: "+string(output), err)
	}

	if c.Verbose {
		fmt.Printf("updated beads issue %s status to %s\n", id, status)
	}

	return nil
}
