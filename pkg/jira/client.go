package jira

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/cockroachdb/errors"

	"thoreinstein.com/rig/pkg/config"
)

// validCliCommandPattern validates CLI command names to prevent injection.
// Allows alphanumeric characters, hyphens, underscores, and forward slashes (for paths).
var validCliCommandPattern = regexp.MustCompile(`^[a-zA-Z0-9_\-/]+$`)

// TicketInfo holds JIRA ticket information
type TicketInfo struct {
	Type         string
	Summary      string
	Status       string
	Priority     string
	Description  string
	CustomFields map[string]string // Maps friendly field names to their values
}

// Transition represents an available workflow transition for a ticket.
type Transition struct {
	ID   string
	Name string
	To   TransitionStatus
}

// TransitionStatus represents the target status of a transition.
type TransitionStatus struct {
	ID   string
	Name string
}

// JiraClient defines the interface for JIRA integrations.
// Current implementations include CLIClient (wrapping the ACLI CLI tool) and APIClient.
type JiraClient interface {
	// IsAvailable checks if the JIRA client is ready to use
	IsAvailable() bool

	// FetchTicketDetails retrieves ticket information from JIRA
	FetchTicketDetails(ticket string) (*TicketInfo, error)

	// GetTransitions returns the available workflow transitions for a ticket.
	GetTransitions(ticket string) ([]Transition, error)

	// TransitionTicket executes a workflow transition by its ID.
	TransitionTicket(ticket string, transitionID string) error

	// TransitionTicketByName finds a transition by status name and executes it.
	// This is a convenience method that calls GetTransitions then TransitionTicket.
	TransitionTicketByName(ticket string, statusName string) error
}

// Compile-time check that CLIClient implements JiraClient.
var _ JiraClient = (*CLIClient)(nil)

// NewJiraClient creates a JiraClient based on the provided configuration.
// Returns an APIClient when mode is "api", or a CLIClient when mode is "acli" or empty.
func NewJiraClient(cfg *config.JiraConfig, verbose bool) (JiraClient, error) {
	if cfg == nil {
		return nil, errors.New("jira config is required")
	}

	switch cfg.Mode {
	case "api":
		return NewAPIClient(cfg, verbose)
	case "acli", "":
		return NewCLIClient(cfg.CliCommand, verbose)
	default:
		return nil, errors.Newf("unknown jira mode: %s", cfg.Mode)
	}
}

// CLIClient handles JIRA integration via CLI tool (e.g., ACLI)
type CLIClient struct {
	CliCommand string
	Verbose    bool
}

// NewCLIClient creates a new CLI-based JIRA client.
// Returns an error if the CLI command contains invalid characters.
func NewCLIClient(cliCommand string, verbose bool) (*CLIClient, error) {
	if !validCliCommandPattern.MatchString(cliCommand) {
		return nil, errors.Newf("invalid CLI command %q: must contain only alphanumeric characters, hyphens, underscores, or forward slashes", cliCommand)
	}
	return &CLIClient{
		CliCommand: cliCommand,
		Verbose:    verbose,
	}, nil
}

// NewClient creates a new JIRA client.
//
// Deprecated: Use NewCLIClient or NewJiraClient instead.
func NewClient(cliCommand string, verbose bool) (*CLIClient, error) {
	return NewCLIClient(cliCommand, verbose)
}

// IsAvailable checks if the JIRA CLI command is available
func (c *CLIClient) IsAvailable() bool {
	_, err := exec.LookPath(c.CliCommand)
	return err == nil
}

// FetchTicketDetails fetches JIRA ticket details using the CLI
func (c *CLIClient) FetchTicketDetails(ticket string) (*TicketInfo, error) {
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
func (c *CLIClient) parseJiraOutput(output string) *TicketInfo {
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
		} else if strings.HasPrefix(line, "Priority:") {
			jiraInfo.Priority = strings.TrimSpace(strings.TrimPrefix(line, "Priority:"))
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
func (c *CLIClient) isNewField(line string) bool {
	// Match pattern: "FieldName:" at the start of line
	matched, _ := regexp.MatchString(`^[A-Z][a-zA-Z\s]*:`, line)
	return matched
}

// GetTransitions returns an error as CLI-based transitions are not implemented.
// The ACLI tool does not provide a straightforward way to query available transitions.
func (c *CLIClient) GetTransitions(ticket string) ([]Transition, error) {
	return nil, errors.New("GetTransitions not implemented for CLI client")
}

// TransitionTicket returns an error as CLI-based transitions are not implemented.
// The ACLI tool does not provide a straightforward way to execute transitions.
func (c *CLIClient) TransitionTicket(ticket string, transitionID string) error {
	return errors.New("TransitionTicket not implemented for CLI client")
}

// TransitionTicketByName returns an error as CLI-based transitions are not implemented.
// The ACLI tool does not provide a straightforward way to execute transitions.
func (c *CLIClient) TransitionTicketByName(ticket string, statusName string) error {
	return errors.New("TransitionTicketByName not implemented for CLI client")
}
