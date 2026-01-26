package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"thoreinstein.com/rig/pkg/ai"
	"thoreinstein.com/rig/pkg/config"
	"thoreinstein.com/rig/pkg/debrief"
	rigerrors "thoreinstein.com/rig/pkg/errors"
	"thoreinstein.com/rig/pkg/github"
	"thoreinstein.com/rig/pkg/jira"
	"thoreinstein.com/rig/pkg/workflow"
)

type PRMergeOptions struct {
	Number       int
	Yes          bool
	NoAI         bool
	AIOnly       bool
	KeepWorktree bool
	NoJira       bool
	MergeMethod  string
	SkipApproval bool
	DeleteBranch bool
}

var prMergeOptions PRMergeOptions

// prMergeCmd executes the full merge workflow with AI debrief.
var prMergeCmd = &cobra.Command{
	Use:   "merge [number]",
	Short: "Merge a pull request with AI debrief",
	Long: `Execute the full merge workflow for a pull request.

The workflow includes:
  1. Pre-flight checks (approval, CI, Jira status)
  2. Context gathering (PR details, commits, timeline)
  3. AI debrief session (interactive Q&A)
  4. Merge execution
  5. Close-out (Jira transition, cleanup)

If no PR number is provided, finds the PR for the current branch.

Examples:
  rig pr merge              # Merge PR for current branch
  rig pr merge 123          # Merge PR #123
  rig pr merge --yes        # Skip confirmations
  rig pr merge --no-ai      # Skip AI debrief
  rig pr merge --ai-only    # Only run AI debrief (no merge)`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			n, err := strconv.Atoi(args[0])
			if err != nil {
				return rigerrors.NewConfigError("number", "invalid PR number")
			}
			prMergeOptions.Number = n
		}

		// Load configuration
		cfg, err := loadConfig()
		if err != nil {
			return rigerrors.NewConfigErrorWithCause("", "failed to load configuration", err)
		}

		// Create GitHub client
		fmt.Println("Connecting to GitHub...")
		ghClient, err := github.NewClient(&cfg.GitHub, verbose)
		if err != nil {
			fmt.Println(rigerrors.FormatUserError(err))
			return err
		}

		// Create Jira client (optional)
		var jiraClient jira.JiraClient
		if cfg.Jira.Enabled && !prMergeOptions.NoJira {
			if verbose {
				fmt.Println("Initializing Jira client...")
			}
			jiraClient, err = jira.NewJiraClient(&cfg.Jira, verbose)
			if err != nil {
				// Warn but continue - Jira is optional
				fmt.Printf("Warning: Could not initialize Jira client: %v\n", err)
				fmt.Println("Continuing without Jira integration...")
			}
		}

		// Create AI provider (optional)
		var aiProvider ai.Provider
		if cfg.AI.Enabled && !prMergeOptions.NoAI {
			if verbose {
				fmt.Println("Initializing AI provider...")
			}
			aiProvider, err = ai.NewProvider(&cfg.AI, verbose)
			if err != nil {
				// Warn but continue - AI is optional
				fmt.Printf("Warning: Could not initialize AI provider: %v\n", err)
				fmt.Println("Continuing without AI debrief...")
				prMergeOptions.NoAI = true
			}
		}

		return runPRMerge(cmd, prMergeOptions, ghClient, jiraClient, aiProvider, cfg)
	},
}

func init() {
	prCmd.AddCommand(prMergeCmd)

	prMergeCmd.Flags().BoolVarP(&prMergeOptions.Yes, "yes", "y", false, "Skip confirmation prompts")
	prMergeCmd.Flags().BoolVar(&prMergeOptions.NoAI, "no-ai", false, "Skip AI debrief step")
	prMergeCmd.Flags().BoolVar(&prMergeOptions.AIOnly, "ai-only", false, "Only run AI debrief (don't merge)")
	prMergeCmd.Flags().BoolVar(&prMergeOptions.KeepWorktree, "keep-worktree", false, "Don't cleanup worktree after merge")
	prMergeCmd.Flags().BoolVar(&prMergeOptions.NoJira, "no-jira", false, "Skip Jira operations")
	prMergeCmd.Flags().StringVar(&prMergeOptions.MergeMethod, "merge-method", "", "Merge method: merge, squash, rebase")
	prMergeCmd.Flags().BoolVar(&prMergeOptions.SkipApproval, "skip-approval", false, "Skip approval check (for self-authored PRs)")
	prMergeCmd.Flags().BoolVarP(&prMergeOptions.DeleteBranch, "delete-branch", "d", false,
		"Delete remote branch after merge (usually not needed if repo has auto-delete enabled)")
}

func runPRMerge(cmd *cobra.Command, opts PRMergeOptions, ghClient github.Client, jiraClient jira.JiraClient, aiProvider ai.Provider, cfg *config.Config) error {
	ctx := context.Background()

	// Check authentication
	if !ghClient.IsAuthenticated() {
		return rigerrors.NewGitHubError("Auth", "not authenticated with GitHub. Run 'gh auth login' first")
	}

	prNumber := opts.Number

	// If no PR number provided, find PR for current branch
	if prNumber == 0 {
		if verbose {
			fmt.Println("No PR number provided, looking for PR for current branch...")
		}
		num, err := findPRForCurrentBranch(ctx, ghClient)
		if err != nil {
			return err
		}
		prNumber = num
		fmt.Printf("Found PR #%d for current branch\n", prNumber)
	}

	// Handle --ai-only mode
	if opts.AIOnly {
		return runAIDebriefOnly(ctx, ghClient, aiProvider, prNumber)
	}

	// Build workflow options
	// Determine if we should delete branch (flag takes precedence over config)
	var deleteBranch *bool
	if cmd.Flags().Changed("delete-branch") {
		// Flag explicitly set (true or false) – honor user choice
		val := opts.DeleteBranch
		deleteBranch = &val
	} else if cfg.GitHub.DeleteBranchOnMerge {
		// Flag not set – fall back to config
		val := true
		deleteBranch = &val
	}

	// Resolve and validate merge method
	mergeMethod, err := resolveMergeMethod(cfg, opts.MergeMethod)
	if err != nil {
		return err
	}

	wfOpts := workflow.MergeOptions{
		SkipAI:           opts.NoAI || aiProvider == nil,
		SkipJira:         opts.NoJira || jiraClient == nil,
		KeepWorktree:     opts.KeepWorktree,
		MergeMethod:      mergeMethod,
		SkipConfirmation: opts.Yes,
		SkipApproval:     opts.SkipApproval,
		DeleteBranch:     deleteBranch,
	}

	// Apply config defaults for optional flags
	if !opts.KeepWorktree && cfg.Workflow.QueueWorktreeCleanup {
		// Default is to queue cleanup based on config
		wfOpts.KeepWorktree = false
	}

	// Get current working directory for ticket routing
	cwd, err := os.Getwd()
	if err != nil {
		return rigerrors.NewWorkflowErrorWithCause("PRMerge", "failed to get current directory", err)
	}

	// Create and run workflow engine
	fmt.Printf("Starting merge workflow for PR #%d...\n", prNumber)
	engine := workflow.NewEngine(ghClient, jiraClient, aiProvider, cfg, cwd, verbose)

	if err := engine.Run(ctx, prNumber, wfOpts); err != nil {
		fmt.Println(rigerrors.FormatUserError(err))
		return err
	}

	fmt.Printf("\n%s PR #%d merged successfully!\n", checkMark(), prNumber)
	return nil
}
func runAIDebriefOnly(ctx context.Context, ghClient github.Client, aiProvider ai.Provider, prNumber int) error {
	if aiProvider == nil {
		return rigerrors.NewAIError("general", "Debrief", "AI provider not available. Configure AI in your config file or check ANTHROPIC_API_KEY/GROQ_API_KEY")
	}

	fmt.Printf("Running AI debrief for PR #%d (no merge)...\n", prNumber)

	// Get PR details
	pr, err := ghClient.GetPR(ctx, prNumber)
	if err != nil {
		return rigerrors.NewGitHubErrorWithCause("GetPR", "failed to get PR details", err)
	}

	// Build debrief context
	debriefCtx := &debrief.Context{
		PRTitle:    pr.Title,
		PRBody:     pr.Body,
		BranchName: pr.HeadBranch,
		BaseBranch: pr.BaseBranch,
	}

	// Run debrief session
	session := debrief.NewDebriefSession(aiProvider, debriefCtx, verbose)
	output, err := session.Run(ctx)
	if err != nil {
		return rigerrors.NewAIErrorWithCause("general", "Debrief", "debrief session failed", err)
	}

	// Display summary
	fmt.Println("\n=== Debrief Summary ===")
	fmt.Println(output.Summary)

	if len(output.KeyDecisions) > 0 {
		fmt.Println("\nKey Decisions:")
		for _, d := range output.KeyDecisions {
			fmt.Printf("  - %s\n", d)
		}
	}

	if len(output.LessonsLearned) > 0 {
		fmt.Println("\nLessons Learned:")
		for _, l := range output.LessonsLearned {
			fmt.Printf("  - %s\n", l)
		}
	}

	if len(output.FollowUps) > 0 {
		fmt.Println("\nFollow-ups:")
		for _, f := range output.FollowUps {
			fmt.Printf("  - %s\n", f)
		}
	}

	return nil
}

// resolveMergeMethod determines the merge method to use.
// Returns an error if the method is invalid.
func resolveMergeMethod(cfg *config.Config, flagValue string) (string, error) {
	method := flagValue
	if method == "" {
		method = cfg.GitHub.DefaultMergeMethod
	}
	if method == "" {
		method = "squash" // Default fallback
	}

	// Validate the merge method
	if err := config.ValidateMergeMethod(method); err != nil {
		return "", err
	}

	return method, nil
}
