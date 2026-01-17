package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"thoreinstein.com/rig/pkg/config"
	rigerrors "thoreinstein.com/rig/pkg/errors"
	"thoreinstein.com/rig/pkg/github"
)

type ListOptions struct {
	State string
	Mine  bool
}

var prListOptions ListOptions

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
		// Load configuration
		cfg, err := config.Load()
		if err != nil {
			return rigerrors.NewConfigErrorWithCause("", "failed to load configuration", err)
		}

		// Create GitHub client
		ghClient, err := github.NewClient(&cfg.GitHub, verbose)
		if err != nil {
			fmt.Println(rigerrors.FormatUserError(err))
			return err
		}

		return runPRList(prListOptions, ghClient, cfg)
	},
}

func init() {
	prCmd.AddCommand(prListCmd)

	prListCmd.Flags().StringVarP(&prListOptions.State, "state", "s", "open", "Filter by state (open, closed, merged, all)")
	prListCmd.Flags().BoolVarP(&prListOptions.Mine, "mine", "m", false, "Show only PRs authored by you")
}

func runPRList(opts ListOptions, ghClient github.Client, cfg *config.Config) error {
	ctx := context.Background()

	if verbose {
		fmt.Printf("Listing PRs with state: %s\n", opts.State)
	}

	// Determine author filter
	author := ""
	if opts.Mine {
		author = "@me"
		if verbose {
			fmt.Printf("Filtering by author: %s\n", author)
		}
	}

	// List PRs with optional author filter
	prs, err := ghClient.ListPRs(ctx, opts.State, author)
	if err != nil {
		fmt.Println(rigerrors.FormatUserError(err))
		return err
	}

	// Display results
	if len(prs) == 0 {
		fmt.Println("No pull requests found.")
		return nil
	}

	displayPRList(prs)
	return nil
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
