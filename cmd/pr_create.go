package cmd

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"thoreinstein.com/rig/pkg/config"
	rigerrors "thoreinstein.com/rig/pkg/errors"
	"thoreinstein.com/rig/pkg/github"
)

type CreateOptions struct {
	Title      string
	Body       string
	Draft      bool
	Reviewers  []string
	BaseBranch string
	NoBrowser  bool
}

var prCreateOptions CreateOptions

// prCreateCmd creates a new pull request from the current branch.
var prCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a pull request",
	Long: `Create a new pull request from the current branch.

If no title is provided, the last commit message subject is used.
After creation, the PR URL is opened in the default browser.

Examples:
  rig pr create                           # Use last commit message as title
  rig pr create --title "Add feature X"   # Specify title
  rig pr create --draft                   # Create as draft PR
  rig pr create --reviewer user1,user2    # Request reviewers`,
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

		return runPRCreate(prCreateOptions, ghClient, cfg)
	},
}

func init() {
	prCmd.AddCommand(prCreateCmd)

	prCreateCmd.Flags().StringVarP(&prCreateOptions.Title, "title", "t", "", "PR title (defaults to last commit message)")
	prCreateCmd.Flags().StringVarP(&prCreateOptions.Body, "body", "b", "", "PR body/description")
	prCreateCmd.Flags().BoolVarP(&prCreateOptions.Draft, "draft", "d", false, "Create as draft PR")
	prCreateCmd.Flags().StringSliceVarP(&prCreateOptions.Reviewers, "reviewer", "r", nil, "Request reviewers (comma-separated)")
	prCreateCmd.Flags().StringVar(&prCreateOptions.BaseBranch, "base", "", "Base branch (defaults to repo default)")
	prCreateCmd.Flags().BoolVar(&prCreateOptions.NoBrowser, "no-browser", false, "Don't open PR URL in browser")
}

func runPRCreate(opts CreateOptions, ghClient github.Client, cfg *config.Config) error {
	ctx := context.Background()

	// Check authentication
	if !ghClient.IsAuthenticated() {
		return rigerrors.NewGitHubError("Auth", "not authenticated with GitHub. Run 'gh auth login' first")
	}

	// Get title from last commit if not provided
	title := opts.Title
	if title == "" {
		if verbose {
			fmt.Println("No title provided, using last commit message...")
		}
		commitTitle, err := getLastCommitMessage()
		if err != nil {
			return rigerrors.NewWorkflowErrorWithCause("PRCreate", "failed to get last commit message", err)
		}
		title = commitTitle
	}

	// Build GitHub create options
	ghOpts := github.CreatePROptions{
		Title:      title,
		Body:       opts.Body,
		BaseBranch: opts.BaseBranch,
		Draft:      opts.Draft,
		Reviewers:  opts.Reviewers,
	}

	// Add default reviewers from config if none specified
	if len(ghOpts.Reviewers) == 0 && len(cfg.GitHub.DefaultReviewers) > 0 {
		ghOpts.Reviewers = cfg.GitHub.DefaultReviewers
		if verbose {
			fmt.Printf("Using default reviewers: %s\n", strings.Join(ghOpts.Reviewers, ", "))
		}
	}

	if verbose {
		fmt.Printf("Creating PR with title: %s\n", title)
		if ghOpts.Draft {
			fmt.Println("  Draft: yes")
		}
		if len(ghOpts.Reviewers) > 0 {
			fmt.Printf("  Reviewers: %s\n", strings.Join(ghOpts.Reviewers, ", "))
		}
	}

	// Create the PR
	pr, err := ghClient.CreatePR(ctx, ghOpts)
	if err != nil {
		fmt.Println(rigerrors.FormatUserError(err))
		return err
	}

	// Print success message
	fmt.Printf("Created PR #%d: %s\n", pr.Number, pr.Title)
	fmt.Printf("URL: %s\n", pr.URL)

	if pr.Draft {
		fmt.Println("Status: Draft")
	}

	// Open in browser unless disabled
	if !opts.NoBrowser && pr.URL != "" {
		if verbose {
			fmt.Println("Opening PR in browser...")
		}
		if err := openURL(pr.URL); err != nil {
			// Don't fail if browser open fails
			if verbose {
				fmt.Printf("Warning: Could not open browser: %v\n", err)
			}
		}
	}

	return nil
}

// getLastCommitMessage returns the subject line of the last commit.
func getLastCommitMessage() (string, error) {
	cmd := exec.Command("git", "log", "-1", "--format=%s")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// openURL opens a URL in the default browser.
func openURL(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return rigerrors.NewWorkflowError("Browser", "unsupported platform: "+runtime.GOOS)
	}

	return cmd.Start()
}
