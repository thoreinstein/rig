package debrief

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"thoreinstein.com/rig/pkg/config"
	rigerrors "thoreinstein.com/rig/pkg/errors"
	"thoreinstein.com/rig/pkg/github"
	"thoreinstein.com/rig/pkg/history"
	"thoreinstein.com/rig/pkg/jira"
)

// Aggregator collects context from various sources for the debrief.
type Aggregator struct {
	ghClient   github.Client
	jiraClient jira.JiraClient
	cfg        *config.Config
	verbose    bool
}

// NewAggregator creates a context aggregator.
func NewAggregator(gh github.Client, jiraClient jira.JiraClient, cfg *config.Config, verbose bool) *Aggregator {
	return &Aggregator{
		ghClient:   gh,
		jiraClient: jiraClient,
		cfg:        cfg,
		verbose:    verbose,
	}
}

// Gather collects all available context for the debrief.
// It aggregates data from GitHub, Jira, git, and shell history.
// Missing sources are handled gracefully - the debrief can proceed with partial context.
func (a *Aggregator) Gather(ctx context.Context, prNumber int, ticket string) (*Context, error) {
	debriefCtx := &Context{
		StartedAt: time.Now(),
	}

	// Gather git context first (always available)
	if err := a.gatherGitContext(ctx, debriefCtx); err != nil {
		if a.verbose {
			fmt.Fprintf(os.Stderr, "Warning: failed to gather git context: %v\n", err)
		}
	}

	// Gather GitHub PR context
	if prNumber > 0 && a.ghClient != nil {
		if err := a.gatherPRContext(ctx, debriefCtx, prNumber); err != nil {
			if a.verbose {
				fmt.Fprintf(os.Stderr, "Warning: failed to gather PR context: %v\n", err)
			}
		}
	}

	// Gather Jira context
	if ticket != "" && a.jiraClient != nil && a.jiraClient.IsAvailable() {
		if err := a.gatherJiraContext(ctx, debriefCtx, ticket); err != nil {
			if a.verbose {
				fmt.Fprintf(os.Stderr, "Warning: failed to gather Jira context: %v\n", err)
			}
		}
	}

	// Gather shell history context
	if err := a.gatherHistoryContext(ctx, debriefCtx, ticket); err != nil {
		if a.verbose {
			fmt.Fprintf(os.Stderr, "Warning: failed to gather history context: %v\n", err)
		}
	}

	// Calculate duration if we have start time info from commits
	if len(debriefCtx.Commits) > 0 {
		firstCommit := debriefCtx.Commits[0]
		debriefCtx.Duration = time.Since(firstCommit.Date)
	}

	return debriefCtx, nil
}

// gatherGitContext collects local git information.
func (a *Aggregator) gatherGitContext(ctx context.Context, debriefCtx *Context) error {
	// Get current branch
	branch, err := a.runGit(ctx, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return rigerrors.Wrap(err, "failed to get current branch")
	}
	debriefCtx.BranchName = strings.TrimSpace(branch)

	// Get base branch (try to detect from tracking or use main/master)
	baseBranch, err := a.detectBaseBranch(ctx)
	if err != nil {
		if a.verbose {
			fmt.Fprintf(os.Stderr, "Warning: could not detect base branch: %v\n", err)
		}
		baseBranch = "main" // fallback
	}
	debriefCtx.BaseBranch = baseBranch

	// Get diff stats against base
	diffStats, err := a.getDiffStats(ctx, baseBranch)
	if err != nil {
		if a.verbose {
			fmt.Fprintf(os.Stderr, "Warning: could not get diff stats: %v\n", err)
		}
	} else {
		debriefCtx.DiffStats = diffStats
	}

	// Get commit history for this branch
	commits, err := a.getCommits(ctx, baseBranch)
	if err != nil {
		if a.verbose {
			fmt.Fprintf(os.Stderr, "Warning: could not get commits: %v\n", err)
		}
	} else {
		debriefCtx.Commits = commits
	}

	// Get changed files
	files, err := a.getChangedFiles(ctx, baseBranch)
	if err != nil {
		if a.verbose {
			fmt.Fprintf(os.Stderr, "Warning: could not get changed files: %v\n", err)
		}
	} else {
		debriefCtx.FilesChanged = files
	}

	return nil
}

// detectBaseBranch attempts to determine the base branch for the current branch.
func (a *Aggregator) detectBaseBranch(ctx context.Context) (string, error) {
	// Check config override first
	if a.cfg != nil && a.cfg.Git.BaseBranch != "" {
		return a.cfg.Git.BaseBranch, nil
	}

	// Try to get from GitHub default branch
	if a.ghClient != nil {
		if defaultBranch, err := a.ghClient.GetDefaultBranch(ctx); err == nil {
			return defaultBranch, nil
		}
	}

	// Try common defaults
	for _, branch := range []string{"main", "master"} {
		if _, err := a.runGit(ctx, "rev-parse", "--verify", "origin/"+branch); err == nil {
			return branch, nil
		}
	}

	return "", rigerrors.New("could not determine base branch")
}

// getDiffStats returns diff statistics comparing current branch to base.
func (a *Aggregator) getDiffStats(ctx context.Context, baseBranch string) (DiffStats, error) {
	// Use --shortstat to get summary
	output, err := a.runGit(ctx, "diff", "--shortstat", baseBranch+"...")
	if err != nil {
		return DiffStats{}, err
	}

	return parseDiffStats(output), nil
}

// parseDiffStats parses git diff --shortstat output.
func parseDiffStats(output string) DiffStats {
	stats := DiffStats{}
	output = strings.TrimSpace(output)
	if output == "" {
		return stats
	}

	// Format: "X files changed, Y insertions(+), Z deletions(-)"
	parts := strings.Split(output, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		fields := strings.Fields(part)
		if len(fields) < 2 {
			continue
		}
		num, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}

		if strings.Contains(part, "file") {
			stats.FilesChanged = num
		} else if strings.Contains(part, "insertion") {
			stats.Insertions = num
		} else if strings.Contains(part, "deletion") {
			stats.Deletions = num
		}
	}

	return stats
}

// getCommits returns commits on the current branch since diverging from base.
func (a *Aggregator) getCommits(ctx context.Context, baseBranch string) ([]CommitSummary, error) {
	// Format: hash|subject|author|date
	format := "%H|%s|%an|%aI"
	output, err := a.runGit(ctx, "log", "--format="+format, baseBranch+"..")
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	commits := make([]CommitSummary, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 4)
		if len(parts) < 4 {
			continue
		}

		date, _ := time.Parse(time.RFC3339, parts[3])
		commits = append(commits, CommitSummary{
			SHA:     parts[0][:8], // Short SHA
			Message: parts[1],
			Author:  parts[2],
			Date:    date,
		})
	}

	return commits, nil
}

// getChangedFiles returns list of files changed compared to base branch.
func (a *Aggregator) getChangedFiles(ctx context.Context, baseBranch string) ([]string, error) {
	output, err := a.runGit(ctx, "diff", "--name-only", baseBranch+"...")
	if err != nil {
		return nil, err
	}

	var files []string
	for _, f := range strings.Split(strings.TrimSpace(output), "\n") {
		if f != "" {
			files = append(files, f)
		}
	}

	return files, nil
}

// gatherPRContext fetches PR details from GitHub.
func (a *Aggregator) gatherPRContext(ctx context.Context, debriefCtx *Context, prNumber int) error {
	pr, err := a.ghClient.GetPR(ctx, prNumber)
	if err != nil {
		return err
	}

	debriefCtx.PRTitle = pr.Title
	debriefCtx.PRBody = pr.Body

	// PR comments would require additional API call - placeholder for future
	// For now, we rely on git commit messages

	return nil
}

// gatherJiraContext fetches ticket details from Jira.
func (a *Aggregator) gatherJiraContext(_ context.Context, debriefCtx *Context, ticket string) error {
	ticketInfo, err := a.jiraClient.FetchTicketDetails(ticket)
	if err != nil {
		return err
	}

	debriefCtx.TicketID = ticket
	debriefCtx.TicketSummary = ticketInfo.Summary
	debriefCtx.TicketType = ticketInfo.Type
	debriefCtx.TicketDescription = ticketInfo.Description

	return nil
}

// gatherHistoryContext fetches relevant shell history.
func (a *Aggregator) gatherHistoryContext(_ context.Context, debriefCtx *Context, ticket string) error {
	if a.cfg == nil {
		return nil
	}

	dbManager := history.NewDatabaseManager(a.cfg.History.DatabasePath, a.verbose)
	if !dbManager.IsAvailable() {
		return nil
	}

	// Query for commands related to this work
	// Look for commands in the last 7 days that might be related
	since := time.Now().Add(-7 * 24 * time.Hour)
	opts := history.QueryOptions{
		Since: &since,
		Limit: 100,
	}

	// If we have a ticket, filter by it
	if ticket != "" {
		opts.Ticket = ticket
	}

	// If we have a working directory from git, filter by it
	if cwd, err := os.Getwd(); err == nil {
		opts.Directory = cwd
	}

	commands, err := dbManager.QueryCommands(opts)
	if err != nil {
		return err
	}

	// Filter to interesting commands (not just ls, cd, etc.)
	for _, cmd := range commands {
		if isInterestingCommand(cmd.Command, a.cfg.History.IgnorePatterns) {
			debriefCtx.RelevantCommands = append(debriefCtx.RelevantCommands, cmd.Command)
		}
	}

	// Limit to most recent 20 interesting commands
	if len(debriefCtx.RelevantCommands) > 20 {
		debriefCtx.RelevantCommands = debriefCtx.RelevantCommands[len(debriefCtx.RelevantCommands)-20:]
	}

	return nil
}

// isInterestingCommand returns true if the command is worth including in the debrief.
func isInterestingCommand(cmd string, ignorePatterns []string) bool {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return false
	}

	// Check against ignore patterns
	for _, pattern := range ignorePatterns {
		if strings.HasPrefix(cmd, pattern+" ") || cmd == pattern {
			return false
		}
	}

	// Additional common commands to ignore
	boringPrefixes := []string{
		"cd ", "ls", "pwd", "clear", "exit", "echo ",
		"cat ", "less ", "more ", "head ", "tail ",
	}
	for _, prefix := range boringPrefixes {
		if strings.HasPrefix(cmd, prefix) {
			return false
		}
	}

	return true
}

// runGit executes a git command and returns its output.
func (a *Aggregator) runGit(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = err.Error()
		}
		return "", rigerrors.New(errMsg)
	}

	return stdout.String(), nil
}
