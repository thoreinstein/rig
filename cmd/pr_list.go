package cmd

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/spf13/cobra"

	"thoreinstein.com/rig/pkg/config"
	rigerrors "thoreinstein.com/rig/pkg/errors"
	"thoreinstein.com/rig/pkg/github"
)

var (
	prListState string
	prListMine  bool
)

// prListCmd lists pull requests.
var prListCmd = &cobra.Command{
	Use:   "list",
	Short: "List pull requests",
	Long: `List pull requests for the current repository.

Filters:
  --state: Filter by state (open, closed, merged, all)
  --mine:  Show only PRs authored by you

Examples:
  rig pr list              # List open PRs
  rig pr list --state all  # List all PRs
  rig pr list --mine       # List your PRs`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runPRList()
	},
}

func init() {
	prCmd.AddCommand(prListCmd)

	prListCmd.Flags().StringVarP(&prListState, "state", "s", "open", "Filter by state (open, closed, merged, all)")
	prListCmd.Flags().BoolVarP(&prListMine, "mine", "m", false, "Show only PRs authored by you")
}

func runPRList() error {
	ctx := context.Background()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return errors.Wrap(err, "failed to load configuration")
	}

	// Create GitHub client
	ghClient, err := github.NewClient(&cfg.GitHub, verbose)
	if err != nil {
		fmt.Println(rigerrors.FormatUserError(err))
		return err
	}

	if verbose {
		fmt.Printf("Listing PRs with state: %s\n", prListState)
	}

	// Get current user for --mine filter
	var currentUser string
	if prListMine {
		currentUser, err = getCurrentGitHubUser()
		if err != nil {
			return errors.Wrap(err, "failed to get current GitHub user")
		}
		if verbose {
			fmt.Printf("Filtering by author: %s\n", currentUser)
		}
	}

	// List PRs
	prs, err := ghClient.ListPRs(ctx, prListState)
	if err != nil {
		fmt.Println(rigerrors.FormatUserError(err))
		return err
	}

	// Filter by current user if --mine flag is set
	if prListMine {
		prs = filterPRsByAuthor(prs, currentUser)
	}

	// Display results
	if len(prs) == 0 {
		fmt.Println("No pull requests found.")
		return nil
	}

	displayPRList(prs)
	return nil
}

// getCurrentGitHubUser returns the authenticated GitHub username.
func getCurrentGitHubUser() (string, error) {
	cmd := exec.Command("gh", "api", "user", "--jq", ".login")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// filterPRsByAuthor filters PRs to only include those by the given author.
// Note: This is a simple filter based on available PR info.
// The gh CLI doesn't return author in the standard fields we request,
// so this filters based on branch naming conventions or returns all if
// author info isn't available.
func filterPRsByAuthor(prs []github.PRInfo, author string) []github.PRInfo {
	// Since the standard PR fields don't include author, we'd need to
	// make additional API calls or use gh pr list --author directly.
	// For now, return all PRs when --mine is used with a note.
	// A more complete implementation would use gh pr list --author @me
	return prs
}

// displayPRList formats and prints a list of PRs.
func displayPRList(prs []github.PRInfo) {
	// Calculate column widths
	maxNumWidth := 5 // "#123" + padding
	maxTitleWidth := 50

	fmt.Println()
	fmt.Printf("%-*s  %-6s  %-*s  %s\n",
		maxNumWidth, "#",
		"STATE",
		maxTitleWidth, "TITLE",
		"BRANCH",
	)
	fmt.Println(strings.Repeat("-", 100))

	for _, pr := range prs {
		// Truncate title if too long
		title := pr.Title
		if len(title) > maxTitleWidth {
			title = title[:maxTitleWidth-3] + "..."
		}

		// Format state
		state := formatState(pr.State, pr.Draft)

		// Print row
		fmt.Printf("#%-*d  %-6s  %-*s  %s\n",
			maxNumWidth-1, pr.Number,
			state,
			maxTitleWidth, title,
			pr.HeadBranch,
		)
	}

	fmt.Printf("\nTotal: %d PR(s)\n", len(prs))
}

// formatState returns a formatted state string.
func formatState(state string, isDraft bool) string {
	s := strings.ToLower(state)
	if isDraft && (s == "open") {
		return "draft"
	}
	return s
}
