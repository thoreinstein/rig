package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/cockroachdb/errors"
	"github.com/spf13/cobra"

	"thoreinstein.com/rig/pkg/ai"
	"thoreinstein.com/rig/pkg/config"
	"thoreinstein.com/rig/pkg/debrief"
	rigerrors "thoreinstein.com/rig/pkg/errors"
	"thoreinstein.com/rig/pkg/github"
	"thoreinstein.com/rig/pkg/jira"
	"thoreinstein.com/rig/pkg/workflow"
)

var (
	prMergeYes          bool
	prMergeNoAI         bool
	prMergeAIOnly       bool
	prMergeKeepWorktree bool
	prMergeNoJira       bool
	prMergeMergeMethod  string
	prMergeSkipApproval bool
)

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
		var prNumber int
		if len(args) > 0 {
			n, err := strconv.Atoi(args[0])
			if err != nil {
				return errors.Wrap(err, "invalid PR number")
			}
			prNumber = n
		}
		return runPRMerge(prNumber)
	},
}

func init() {
	prCmd.AddCommand(prMergeCmd)

	prMergeCmd.Flags().BoolVarP(&prMergeYes, "yes", "y", false, "Skip confirmation prompts")
	prMergeCmd.Flags().BoolVar(&prMergeNoAI, "no-ai", false, "Skip AI debrief step")
	prMergeCmd.Flags().BoolVar(&prMergeAIOnly, "ai-only", false, "Only run AI debrief (don't merge)")
	prMergeCmd.Flags().BoolVar(&prMergeKeepWorktree, "keep-worktree", false, "Don't cleanup worktree after merge")
	prMergeCmd.Flags().BoolVar(&prMergeNoJira, "no-jira", false, "Skip Jira operations")
	prMergeCmd.Flags().StringVar(&prMergeMergeMethod, "merge-method", "", "Merge method: merge, squash, rebase")
	prMergeCmd.Flags().BoolVar(&prMergeSkipApproval, "skip-approval", false, "Skip approval check (for self-authored PRs)")
}

func runPRMerge(prNumber int) error {
	ctx := context.Background()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return errors.Wrap(err, "failed to load configuration")
	}

	// Create GitHub client
	fmt.Println("Connecting to GitHub...")
	ghClient, err := github.NewClient(&cfg.GitHub, verbose)
	if err != nil {
		fmt.Println(rigerrors.FormatUserError(err))
		return err
	}

	// Check authentication
	if !ghClient.IsAuthenticated() {
		return errors.New("not authenticated with GitHub. Run 'gh auth login' first")
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
		fmt.Printf("Found PR #%d for current branch\n", prNumber)
	}

	// Create Jira client (optional)
	var jiraClient jira.JiraClient
	if cfg.Jira.Enabled && !prMergeNoJira {
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
	if cfg.AI.Enabled && !prMergeNoAI {
		if verbose {
			fmt.Println("Initializing AI provider...")
		}
		aiProvider, err = ai.NewProvider(&cfg.AI, verbose)
		if err != nil {
			// Warn but continue - AI is optional
			fmt.Printf("Warning: Could not initialize AI provider: %v\n", err)
			fmt.Println("Continuing without AI debrief...")
			prMergeNoAI = true
		}
	}

	// Handle --ai-only mode
	if prMergeAIOnly {
		return runAIDebriefOnly(ctx, ghClient, aiProvider, prNumber)
	}

	// Build workflow options
	opts := workflow.MergeOptions{
		SkipAI:           prMergeNoAI || aiProvider == nil,
		SkipJira:         prMergeNoJira || jiraClient == nil,
		KeepWorktree:     prMergeKeepWorktree,
		MergeMethod:      resolveMergeMethod(cfg, prMergeMergeMethod),
		SkipConfirmation: prMergeYes,
		SkipApproval:     prMergeSkipApproval,
	}

	// Apply config defaults for optional flags
	if !prMergeKeepWorktree && cfg.Workflow.QueueWorktreeCleanup {
		// Default is to queue cleanup based on config
		opts.KeepWorktree = false
	}

	// Get current working directory for ticket routing
	cwd, err := os.Getwd()
	if err != nil {
		return errors.Wrap(err, "failed to get current directory")
	}

	// Create and run workflow engine
	fmt.Printf("Starting merge workflow for PR #%d...\n", prNumber)
	engine := workflow.NewEngine(ghClient, jiraClient, aiProvider, cfg, cwd, verbose)

	if err := engine.Run(ctx, prNumber, opts); err != nil {
		fmt.Println(rigerrors.FormatUserError(err))
		return err
	}

	fmt.Printf("\n%s PR #%d merged successfully!\n", checkMark(), prNumber)
	return nil
}
func runAIDebriefOnly(ctx context.Context, ghClient github.Client, aiProvider ai.Provider, prNumber int) error {
	if aiProvider == nil {
		return errors.New("AI provider not available. Configure AI in your config file or check ANTHROPIC_API_KEY/GROQ_API_KEY")
	}

	fmt.Printf("Running AI debrief for PR #%d (no merge)...\n", prNumber)

	// Get PR details
	pr, err := ghClient.GetPR(ctx, prNumber)
	if err != nil {
		return errors.Wrap(err, "failed to get PR details")
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
		return errors.Wrap(err, "debrief session failed")
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
func resolveMergeMethod(cfg *config.Config, flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if cfg.GitHub.DefaultMergeMethod != "" {
		return cfg.GitHub.DefaultMergeMethod
	}
	return "squash" // Default fallback
}
