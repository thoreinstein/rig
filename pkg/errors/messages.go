package errors

import (
	"fmt"
	"strings"
)

// FormatUserError returns a user-friendly error message with actionable guidance.
// It examines the error chain and provides context-appropriate help text.
func FormatUserError(err error) string {
	if err == nil {
		return ""
	}

	// Check for ConfigError
	var configErr *ConfigError
	if As(err, &configErr) {
		return formatConfigError(configErr)
	}

	// Check for GitHubError
	var ghErr *GitHubError
	if As(err, &ghErr) {
		return formatGitHubError(ghErr)
	}

	// Check for AIError
	var aiErr *AIError
	if As(err, &aiErr) {
		return formatAIError(aiErr)
	}

	// Check for JiraError
	var jiraErr *JiraError
	if As(err, &jiraErr) {
		return formatJiraError(jiraErr)
	}

	// Check for WorkflowError
	var wfErr *WorkflowError
	if As(err, &wfErr) {
		return formatWorkflowError(wfErr)
	}

	// Default: return the error message as-is
	return err.Error()
}

// formatConfigError formats a ConfigError with actionable guidance.
func formatConfigError(err *ConfigError) string {
	var b strings.Builder

	if err.Field != "" {
		fmt.Fprintf(&b, "Configuration error in '%s': %s\n", err.Field, err.Message)
	} else {
		fmt.Fprintf(&b, "Configuration error: %s\n", err.Message)
	}

	b.WriteString("\nTo fix this:\n")
	b.WriteString("  • Check your config file: ~/.config/rig/config.toml\n")
	b.WriteString("  • Run 'rig config setup' to reconfigure\n")

	if err.Cause != nil {
		fmt.Fprintf(&b, "\nUnderlying error: %v", err.Cause)
	}

	return b.String()
}

// formatGitHubError formats a GitHubError with actionable guidance based on status code.
func formatGitHubError(err *GitHubError) string {
	var b strings.Builder

	fmt.Fprintf(&b, "GitHub error during %s: %s\n", err.Operation, err.Message)

	switch err.StatusCode {
	case 401:
		b.WriteString("\nAuthentication failed. To fix this:\n")
		b.WriteString("  • Run 'rig config setup' to configure GitHub authentication\n")
		b.WriteString("  • Or set the RIG_GITHUB_TOKEN environment variable\n")
		b.WriteString("  • Ensure your token has the required scopes (repo, read:org)\n")

	case 403:
		b.WriteString("\nPermission denied. To fix this:\n")
		b.WriteString("  • Ensure you have write access to this repository\n")
		b.WriteString("  • Check that your token has the 'repo' scope\n")
		b.WriteString("  • If using SSO, ensure the token is authorized for your organization\n")

	case 404:
		b.WriteString("\nResource not found. To fix this:\n")
		b.WriteString("  • Verify the repository name and owner are correct\n")
		b.WriteString("  • Ensure the branch or PR exists\n")
		b.WriteString("  • Check that you have access to the repository\n")

	case 422:
		b.WriteString("\nValidation failed. To fix this:\n")
		b.WriteString("  • Check that all required fields are provided\n")
		b.WriteString("  • Ensure branch names don't conflict with existing branches\n")
		b.WriteString("  • Review the error message for specific field issues\n")

	case 429:
		b.WriteString("\nRate limit exceeded. To fix this:\n")
		b.WriteString("  • Wait a few minutes before retrying\n")
		b.WriteString("  • Consider using a GitHub App for higher rate limits\n")

	case 500, 502, 503, 504:
		b.WriteString("\nGitHub server error. To fix this:\n")
		b.WriteString("  • Wait a few moments and try again\n")
		b.WriteString("  • Check GitHub Status: https://www.githubstatus.com\n")
	}

	if err.Retryable {
		b.WriteString("\nThis error may be temporary. The operation will be retried automatically.\n")
	}

	if err.Cause != nil {
		fmt.Fprintf(&b, "\nUnderlying error: %v", err.Cause)
	}

	return b.String()
}

// formatAIError formats an AIError with actionable guidance based on status code.
func formatAIError(err *AIError) string {
	var b strings.Builder

	fmt.Fprintf(&b, "AI provider error (%s) during %s: %s\n", err.Provider, err.Operation, err.Message)

	switch err.StatusCode {
	case 401:
		fmt.Fprintf(&b, "\nAuthentication failed with %s. To fix this:\n", err.Provider)
		b.WriteString("  • Run 'rig config setup' to configure AI provider\n")
		fmt.Fprintf(&b, "  • Or set the appropriate API key environment variable\n")
		b.WriteString("  • Verify your API key is valid and not expired\n")

	case 403:
		fmt.Fprintf(&b, "\nAccess denied by %s. To fix this:\n", err.Provider)
		b.WriteString("  • Check your API key permissions\n")
		b.WriteString("  • Verify your account is in good standing\n")
		b.WriteString("  • Ensure the model you're using is available to your account tier\n")

	case 429:
		fmt.Fprintf(&b, "\n%s rate limit exceeded. To fix this:\n", err.Provider)
		b.WriteString("  • Wait a few minutes before retrying\n")
		b.WriteString("  • Consider upgrading your API tier for higher limits\n")
		b.WriteString("  • Reduce request frequency\n")

	case 500, 502, 503, 504:
		fmt.Fprintf(&b, "\n%s server error. To fix this:\n", err.Provider)
		b.WriteString("  • Wait a few moments and try again\n")
		b.WriteString("  • Check the provider's status page\n")
	}

	if err.Retryable {
		b.WriteString("\nThis error may be temporary. The operation will be retried automatically.\n")
	}

	if err.Cause != nil {
		fmt.Fprintf(&b, "\nUnderlying error: %v", err.Cause)
	}

	return b.String()
}

// formatJiraError formats a JiraError with actionable guidance based on status code.
func formatJiraError(err *JiraError) string {
	var b strings.Builder

	if err.Ticket != "" {
		fmt.Fprintf(&b, "Jira error during %s for ticket %s: %s\n", err.Operation, err.Ticket, err.Message)
	} else {
		fmt.Fprintf(&b, "Jira error during %s: %s\n", err.Operation, err.Message)
	}

	switch err.StatusCode {
	case 401:
		b.WriteString("\nAuthentication failed. To fix this:\n")
		b.WriteString("  • Run 'rig config setup' to configure Jira authentication\n")
		b.WriteString("  • Or set the JIRA_TOKEN environment variable\n")
		b.WriteString("  • Verify your email and API token are correct\n")
		b.WriteString("  • Generate a new API token at: https://id.atlassian.com/manage-profile/security/api-tokens\n")

	case 403:
		b.WriteString("\nAccess denied. To fix this:\n")
		b.WriteString("  • Ensure you have permission to access this ticket\n")
		b.WriteString("  • Check that your Jira account has the required project permissions\n")

	case 404:
		if err.Ticket != "" {
			fmt.Fprintf(&b, "\nTicket %s not found. To fix this:\n", err.Ticket)
		} else {
			b.WriteString("\nResource not found. To fix this:\n")
		}
		b.WriteString("  • Verify the ticket ID is correct\n")
		b.WriteString("  • Check that you have access to the project\n")

	case 429:
		b.WriteString("\nJira rate limit exceeded. To fix this:\n")
		b.WriteString("  • Wait before making more requests\n")
		b.WriteString("  • The request will be retried automatically\n")

	case 500, 502, 503, 504:
		b.WriteString("\nJira server error. To fix this:\n")
		b.WriteString("  • Wait a few moments and try again\n")
		b.WriteString("  • Check Atlassian Status: https://status.atlassian.com\n")
	}

	if err.Retryable {
		b.WriteString("\nThis error may be temporary. The operation will be retried automatically.\n")
	}

	if err.Cause != nil {
		fmt.Fprintf(&b, "\nUnderlying error: %v", err.Cause)
	}

	return b.String()
}

// formatWorkflowError formats a WorkflowError with actionable guidance.
func formatWorkflowError(err *WorkflowError) string {
	var b strings.Builder

	if err.Step != "" {
		fmt.Fprintf(&b, "Workflow error in '%s' step: %s\n", err.Step, err.Message)
	} else {
		fmt.Fprintf(&b, "Workflow error: %s\n", err.Message)
	}

	// Provide step-specific guidance
	switch err.Step {
	case "preflight":
		b.WriteString("\nPreflight checks failed. To fix this:\n")
		b.WriteString("  • Ensure you have uncommitted changes staged\n")
		b.WriteString("  • Verify your branch is up to date with the base branch\n")
		b.WriteString("  • Check that all required tools are available\n")

	case "gather":
		b.WriteString("\nFailed to gather context. To fix this:\n")
		b.WriteString("  • Check your git repository is in a clean state\n")
		b.WriteString("  • Verify network connectivity for external services\n")

	case "debrief":
		b.WriteString("\nDebrief step failed. To fix this:\n")
		b.WriteString("  • Review the AI provider configuration\n")
		b.WriteString("  • Check network connectivity\n")
		b.WriteString("  • Try running with --verbose for more details\n")

	case "merge":
		b.WriteString("\nMerge failed. To fix this:\n")
		b.WriteString("  • Ensure the PR has been approved\n")
		b.WriteString("  • Check for merge conflicts\n")
		b.WriteString("  • Verify all required status checks have passed\n")

	case "closeout":
		b.WriteString("\nCloseout failed. To fix this:\n")
		b.WriteString("  • The PR may have been merged successfully\n")
		b.WriteString("  • Check Jira/tmux/worktree cleanup manually if needed\n")

	default:
		b.WriteString("\nTo troubleshoot:\n")
		b.WriteString("  • Run with --verbose for more details\n")
		b.WriteString("  • Check the error message for specific issues\n")
	}

	if err.Retryable {
		b.WriteString("\nThis error may be temporary. You can try running the command again.\n")
	}

	if err.Cause != nil {
		fmt.Fprintf(&b, "\nUnderlying error: %v", err.Cause)
	}

	return b.String()
}
