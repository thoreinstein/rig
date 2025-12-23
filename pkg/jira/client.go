package jira

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/cockroachdb/errors"
)

// validCliCommandPattern validates CLI command names to prevent injection.
// Allows alphanumeric characters, hyphens, underscores, and forward slashes (for paths).
var validCliCommandPattern = regexp.MustCompile(`^[a-zA-Z0-9_\-/]+$`)

// TicketInfo holds JIRA ticket information
type TicketInfo struct {
	Type        string
	Summary     string
	Status      string
	Description string
}

// Client handles JIRA integration via CLI tool
type Client struct {
	CliCommand string
	Verbose    bool
}

// NewClient creates a new JIRA client.
// Returns an error if the CLI command contains invalid characters.
func NewClient(cliCommand string, verbose bool) (*Client, error) {
	if !validCliCommandPattern.MatchString(cliCommand) {
		return nil, errors.Newf("invalid CLI command %q: must contain only alphanumeric characters, hyphens, underscores, or forward slashes", cliCommand)
	}
	return &Client{
		CliCommand: cliCommand,
		Verbose:    verbose,
	}, nil
}

// IsAvailable checks if the JIRA CLI command is available
func (c *Client) IsAvailable() bool {
	_, err := exec.LookPath(c.CliCommand)
	return err == nil
}

// FetchTicketDetails fetches JIRA ticket details using the CLI
func (c *Client) FetchTicketDetails(ticket string) (*TicketInfo, error) {
	if !c.IsAvailable() {
		if c.Verbose {
			fmt.Printf("JIRA CLI command '%s' not found, skipping JIRA details fetch\n", c.CliCommand)
		}
		return nil, errors.New("JIRA CLI command not available")
	}

	// Execute the CLI command
	//nolint:gosec // G204: ticket validated by parseTicket regex, CliCommand from config
	cmd := exec.Command(c.CliCommand, "jira", "workitem", "view", ticket)
	output, err := cmd.Output()
	if err != nil {
		if c.Verbose {
			fmt.Printf("Failed to fetch JIRA details for %s: %v\n", ticket, err)
		}
		return nil, errors.Wrap(err, "failed to fetch JIRA details")
	}

	// Parse the output
	jiraInfo := c.parseJiraOutput(string(output))

	if c.Verbose {
		fmt.Printf("Fetched JIRA details for %s: %s\n", ticket, jiraInfo.Summary)
	}

	return jiraInfo, nil
}

// parseJiraOutput parses the output from the JIRA CLI command
func (c *Client) parseJiraOutput(output string) *TicketInfo {
	jiraInfo := &TicketInfo{}

	lines := strings.Split(output, "\n")
	descriptionStarted := false
	var descriptionLines []string

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "Type:") {
			jiraInfo.Type = strings.TrimSpace(strings.TrimPrefix(line, "Type:"))
		} else if strings.HasPrefix(line, "Summary:") {
			jiraInfo.Summary = strings.TrimSpace(strings.TrimPrefix(line, "Summary:"))
		} else if strings.HasPrefix(line, "Status:") {
			jiraInfo.Status = strings.TrimSpace(strings.TrimPrefix(line, "Status:"))
		} else if strings.HasPrefix(line, "Description:") {
			descriptionStarted = true
			// Don't include the "Description:" line itself
			continue
		} else if descriptionStarted {
			// Stop collecting description if we hit a new field (starts with capital letter and colon)
			if c.isNewField(line) {
				descriptionStarted = false
			} else if line != "" || len(descriptionLines) > 0 {
				// Include empty lines only if we already have description content
				descriptionLines = append(descriptionLines, line)
			}
		}
	}

	if len(descriptionLines) > 0 {
		// Trim trailing empty lines
		for len(descriptionLines) > 0 && descriptionLines[len(descriptionLines)-1] == "" {
			descriptionLines = descriptionLines[:len(descriptionLines)-1]
		}
		jiraInfo.Description = strings.Join(descriptionLines, "\n")
	}

	return jiraInfo
}

// isNewField checks if a line represents a new field in JIRA output
func (c *Client) isNewField(line string) bool {
	// Match pattern: "FieldName:" at the start of line
	matched, _ := regexp.MatchString(`^[A-Z][a-zA-Z\s]*:`, line)
	return matched
}
