package cmd

import (
	"github.com/spf13/cobra"
)

// prCmd is the parent command for PR operations.
var prCmd = &cobra.Command{
	Use:   "pr",
	Short: "Manage pull requests",
	Long: `Manage GitHub pull requests with AI-assisted merge workflow.

The pr command provides subcommands for creating, viewing, listing,
and merging pull requests with optional AI-powered debrief sessions.

Examples:
  rig pr create              # Create PR from current branch
  rig pr view                # View PR for current branch
  rig pr view 123            # View PR #123
  rig pr list                # List open PRs
  rig pr merge 123           # Full merge workflow with AI debrief`,
}

func init() {
	rootCmd.AddCommand(prCmd)
}
