package cmd

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/spf13/cobra"

	"thoreinstein.com/rig/pkg/config"
	rigerrors "thoreinstein.com/rig/pkg/errors"
	"thoreinstein.com/rig/pkg/github"
)

// prViewCmd displays pull request details.
var prViewCmd = &cobra.Command{
	Use:   "view [number]",
	Short: "View pull request details",
	Long: `View details for a pull request.

If no PR number is provided, finds the PR for the current branch.

Displays:
  - Title and state
  - Reviewers and approval status
  - CI checks status
  - Mergeable state

Examples:
  rig view view         # View PR for current branch
  rig pr view 123       # View PR #123`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var prNumber int
		if len(args) > 0 {
			n, err := strconv.Atoi(args[0])
			if err != nil {
				return errors.Wrap(err, "invalid PR number")
			}
			prNumber = n
		}
		return runPRView(prNumber)
	},
}

func init() {
	prCmd.AddCommand(prViewCmd)
}

func runPRView(prNumber int) error {
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

	// If no PR number provided, find PR for current branch
	if prNumber == 0 {
		if verbose {
			fmt.Println("No PR number provided, looking for PR for current branch...")
		}
		prNumber, err = findPRForCurrentBranch(ctx, ghClient)
		if err != nil {
			return err
		}
	}

	// Get PR details
	pr, err := ghClient.GetPR(ctx, prNumber)
	if err != nil {
		fmt.Println(rigerrors.FormatUserError(err))
		return err
	}

	// Display PR information
	displayPRInfo(pr)

	return nil
}

// findPRForCurrentBranch finds the PR associated with the current git branch.
func findPRForCurrentBranch(ctx context.Context, ghClient github.Client) (int, error) {
	// Get current branch name
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return 0, errors.Wrap(err, "failed to get current branch")
	}
	branch := strings.TrimSpace(string(output))

	if branch == "HEAD" || branch == "" {
		return 0, errors.New("not on a branch (detached HEAD state)")
	}

	if verbose {
		fmt.Printf("Looking for PR with head branch: %s\n", branch)
	}

	// List open PRs and find one matching the branch
	prs, err := ghClient.ListPRs(ctx, "open")
	if err != nil {
		return 0, errors.Wrap(err, "failed to list PRs")
	}

	for _, pr := range prs {
		if pr.HeadBranch == branch {
			return pr.Number, nil
		}
	}

	return 0, errors.Newf("no open PR found for branch '%s'", branch)
}

// displayPRInfo formats and prints PR information.
func displayPRInfo(pr *github.PRInfo) {
	// Title and number
	fmt.Printf("\n#%d: %s\n", pr.Number, pr.Title)
	fmt.Printf("URL: %s\n", pr.URL)
	fmt.Println(strings.Repeat("-", 60))

	// State
	stateIcon := stateToIcon(pr.State)
	fmt.Printf("State:     %s %s", stateIcon, pr.State)
	if pr.Draft {
		fmt.Print(" (draft)")
	}
	fmt.Println()

	// Branches
	fmt.Printf("Branches:  %s -> %s\n", pr.HeadBranch, pr.BaseBranch)

	// Review status
	reviewIcon := "?"
	reviewStatus := "Pending"
	if pr.Approved {
		reviewIcon = checkMark()
		reviewStatus = "Approved"
	} else if len(pr.Reviewers) > 0 {
		reviewIcon = "..."
		reviewStatus = fmt.Sprintf("Waiting (%s)", strings.Join(pr.Reviewers, ", "))
	}
	fmt.Printf("Reviews:   %s %s\n", reviewIcon, reviewStatus)

	// CI checks
	var checksIcon, checksStatus string
	if pr.ChecksPassing {
		checksIcon = checkMark()
		checksStatus = "Passing"
	} else {
		checksIcon = crossMark()
		checksStatus = "Failing"
	}
	fmt.Printf("Checks:    %s %s\n", checksIcon, checksStatus)

	// Mergeable state
	mergeIcon := "?"
	mergeStatus := pr.Mergeable
	if pr.IsMergeable() {
		mergeIcon = checkMark()
		mergeStatus = "No conflicts"
	} else if pr.Mergeable == "CONFLICTING" {
		mergeIcon = crossMark()
		mergeStatus = "Has conflicts"
	}
	fmt.Printf("Mergeable: %s %s\n", mergeIcon, mergeStatus)

	// Overall merge state
	if pr.IsClean() {
		fmt.Printf("\n%s Ready to merge\n", checkMark())
	} else if pr.State == "MERGED" || pr.State == "merged" {
		fmt.Printf("\n%s Already merged\n", checkMark())
	} else if pr.State == "CLOSED" || pr.State == "closed" {
		fmt.Println("\nClosed without merging")
	} else {
		fmt.Printf("\nMerge state: %s\n", pr.MergeableState)
	}

	// Body preview
	if pr.Body != "" {
		fmt.Println(strings.Repeat("-", 60))
		body := pr.Body
		if len(body) > 500 {
			body = body[:500] + "..."
		}
		fmt.Println(body)
	}

	fmt.Println()
}

// stateToIcon returns an icon for the PR state.
func stateToIcon(state string) string {
	switch strings.ToUpper(state) {
	case "OPEN":
		return "O"
	case "CLOSED":
		return crossMark()
	case "MERGED":
		return checkMark()
	default:
		return "?"
	}
}

// checkMark returns a check mark symbol.
func checkMark() string {
	return "\u2713" // ✓
}

// crossMark returns a cross mark symbol.
func crossMark() string {
	return "\u2717" // ✗
}
