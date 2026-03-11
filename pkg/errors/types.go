// Package errors provides typed errors for the rig project.
//
// This package defines domain-specific error types that provide structured
// error information for different subsystems (config, GitHub, AI, Jira, workflow).
// All error types implement the standard error interface and support
// errors.Is() and errors.As() from the standard library and cockroachdb/errors.
package errors

import (
	"fmt"
	"strings"

	"github.com/cockroachdb/errors"
)

// ConfigError represents configuration-related errors.
type ConfigError struct {
	Field   string // Which config field has the issue
	Message string
	Cause   error
}

// Error implements the error interface.
func (e *ConfigError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("config error in %s: %s", e.Field, e.Message)
	}
	return "config error: " + e.Message
}

// Unwrap returns the underlying cause for error chain traversal.
func (e *ConfigError) Unwrap() error {
	return e.Cause
}

// NewConfigError creates a new ConfigError.
func NewConfigError(field, message string) *ConfigError {
	return &ConfigError{Field: field, Message: message}
}

// NewConfigErrorWithCause creates a new ConfigError with an underlying cause.
func NewConfigErrorWithCause(field, message string, cause error) *ConfigError {
	return &ConfigError{Field: field, Message: message, Cause: cause}
}

// GitHubError represents GitHub API/CLI errors.
type GitHubError struct {
	Operation  string // e.g., "CreatePR", "MergePR"
	StatusCode int    // HTTP status code if applicable
	Message    string
	Retryable  bool
	Cause      error
}

// Error implements the error interface.
func (e *GitHubError) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("github %s failed (HTTP %d): %s", e.Operation, e.StatusCode, e.Message)
	}
	return fmt.Sprintf("github %s failed: %s", e.Operation, e.Message)
}

// Unwrap returns the underlying cause for error chain traversal.
func (e *GitHubError) Unwrap() error {
	return e.Cause
}

// NewGitHubError creates a new GitHubError.
func NewGitHubError(operation, message string) *GitHubError {
	return &GitHubError{Operation: operation, Message: message}
}

// NewGitHubErrorWithStatus creates a new GitHubError with HTTP status code.
func NewGitHubErrorWithStatus(operation string, statusCode int, message string) *GitHubError {
	retryable := isRetryableHTTPStatus(statusCode)
	return &GitHubError{
		Operation:  operation,
		StatusCode: statusCode,
		Message:    message,
		Retryable:  retryable,
	}
}

// NewGitHubErrorWithCause creates a new GitHubError with an underlying cause.
func NewGitHubErrorWithCause(operation, message string, cause error) *GitHubError {
	return &GitHubError{
		Operation: operation,
		Message:   message,
		Retryable: IsRetryable(cause),
		Cause:     cause,
	}
}

// AIError represents AI provider errors.
type AIError struct {
	Provider   string // e.g., "anthropic", "groq"
	Operation  string // e.g., "Chat", "StreamChat"
	StatusCode int
	Message    string
	Retryable  bool
	Cause      error
}

// Error implements the error interface.
func (e *AIError) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("ai %s %s failed (HTTP %d): %s", e.Provider, e.Operation, e.StatusCode, e.Message)
	}
	return fmt.Sprintf("ai %s %s failed: %s", e.Provider, e.Operation, e.Message)
}

// Unwrap returns the underlying cause for error chain traversal.
func (e *AIError) Unwrap() error {
	return e.Cause
}

// NewAIError creates a new AIError.
func NewAIError(provider, operation, message string) *AIError {
	return &AIError{Provider: provider, Operation: operation, Message: message}
}

// NewAIErrorWithStatus creates a new AIError with HTTP status code.
func NewAIErrorWithStatus(provider, operation string, statusCode int, message string) *AIError {
	retryable := isRetryableHTTPStatus(statusCode)
	return &AIError{
		Provider:   provider,
		Operation:  operation,
		StatusCode: statusCode,
		Message:    message,
		Retryable:  retryable,
	}
}

// NewAIErrorWithCause creates a new AIError with an underlying cause.
func NewAIErrorWithCause(provider, operation, message string, cause error) *AIError {
	return &AIError{
		Provider:  provider,
		Operation: operation,
		Message:   message,
		Retryable: IsRetryable(cause),
		Cause:     cause,
	}
}

// JiraError represents Jira API errors.
type JiraError struct {
	Operation  string
	Ticket     string
	StatusCode int
	Message    string
	Retryable  bool
	Cause      error
}

// Error implements the error interface.
func (e *JiraError) Error() string {
	if e.Ticket != "" && e.StatusCode > 0 {
		return fmt.Sprintf("jira %s for %s failed (HTTP %d): %s", e.Operation, e.Ticket, e.StatusCode, e.Message)
	}
	if e.Ticket != "" {
		return fmt.Sprintf("jira %s for %s failed: %s", e.Operation, e.Ticket, e.Message)
	}
	if e.StatusCode > 0 {
		return fmt.Sprintf("jira %s failed (HTTP %d): %s", e.Operation, e.StatusCode, e.Message)
	}
	return fmt.Sprintf("jira %s failed: %s", e.Operation, e.Message)
}

// Unwrap returns the underlying cause for error chain traversal.
func (e *JiraError) Unwrap() error {
	return e.Cause
}

// NewJiraError creates a new JiraError.
func NewJiraError(operation, message string) *JiraError {
	return &JiraError{Operation: operation, Message: message}
}

// NewJiraErrorWithTicket creates a new JiraError for a specific ticket.
func NewJiraErrorWithTicket(operation, ticket, message string) *JiraError {
	return &JiraError{Operation: operation, Ticket: ticket, Message: message}
}

// NewJiraErrorWithStatus creates a new JiraError with HTTP status code.
func NewJiraErrorWithStatus(operation, ticket string, statusCode int, message string) *JiraError {
	retryable := isRetryableHTTPStatus(statusCode)
	return &JiraError{
		Operation:  operation,
		Ticket:     ticket,
		StatusCode: statusCode,
		Message:    message,
		Retryable:  retryable,
	}
}

// NewJiraErrorWithCause creates a new JiraError with an underlying cause.
func NewJiraErrorWithCause(operation, ticket, message string, cause error) *JiraError {
	return &JiraError{
		Operation: operation,
		Ticket:    ticket,
		Message:   message,
		Retryable: IsRetryable(cause),
		Cause:     cause,
	}
}

// BeadsError represents beads issue tracking errors.
type BeadsError struct {
	Operation string
	IssueID   string
	Message   string
	Retryable bool
	Cause     error
}

// Error implements the error interface.
func (e *BeadsError) Error() string {
	if e.IssueID != "" {
		return fmt.Sprintf("beads %s for %s failed: %s", e.Operation, e.IssueID, e.Message)
	}
	return fmt.Sprintf("beads %s failed: %s", e.Operation, e.Message)
}

// Unwrap returns the underlying cause for error chain traversal.
func (e *BeadsError) Unwrap() error {
	return e.Cause
}

// NewBeadsError creates a new BeadsError.
func NewBeadsError(operation, message string) *BeadsError {
	return &BeadsError{Operation: operation, Message: message}
}

// NewBeadsErrorWithIssue creates a new BeadsError for a specific issue.
func NewBeadsErrorWithIssue(operation, issueID, message string) *BeadsError {
	return &BeadsError{Operation: operation, IssueID: issueID, Message: message}
}

// NewBeadsErrorWithCause creates a new BeadsError with an underlying cause.
func NewBeadsErrorWithCause(operation, issueID, message string, cause error) *BeadsError {
	return &BeadsError{
		Operation: operation,
		IssueID:   issueID,
		Message:   message,
		Retryable: IsRetryable(cause),
		Cause:     cause,
	}
}

// WorkflowError represents workflow orchestration errors.
type WorkflowError struct {
	Step      string // e.g., "preflight", "gather", "debrief", "merge", "closeout"
	Message   string
	Retryable bool
	Cause     error
}

// Error implements the error interface.
func (e *WorkflowError) Error() string {
	if e.Step != "" {
		return fmt.Sprintf("workflow step %s failed: %s", e.Step, e.Message)
	}
	return "workflow error: " + e.Message
}

// Unwrap returns the underlying cause for error chain traversal.
func (e *WorkflowError) Unwrap() error {
	return e.Cause
}

// NewWorkflowError creates a new WorkflowError.
func NewWorkflowError(step, message string) *WorkflowError {
	return &WorkflowError{Step: step, Message: message}
}

// NewWorkflowErrorWithCause creates a new WorkflowError with an underlying cause.
func NewWorkflowErrorWithCause(step, message string, cause error) *WorkflowError {
	return &WorkflowError{
		Step:      step,
		Message:   message,
		Retryable: IsRetryable(cause),
		Cause:     cause,
	}
}

// CapabilityError represents capability/permission denial errors.
type CapabilityError struct {
	NodeID     string
	Capability string // e.g., "network", "filesystem", "secret"
	Message    string
	Cause      error
}

// Error implements the error interface.
func (e *CapabilityError) Error() string {
	if e.NodeID != "" {
		return fmt.Sprintf("capability %s denied for node %s: %s", e.Capability, e.NodeID, e.Message)
	}
	return fmt.Sprintf("capability %s denied: %s", e.Capability, e.Message)
}

// Unwrap returns the underlying cause for error chain traversal.
func (e *CapabilityError) Unwrap() error {
	return e.Cause
}

// NewCapabilityError creates a new CapabilityError.
func NewCapabilityError(nodeID, capability, message string) *CapabilityError {
	return &CapabilityError{NodeID: nodeID, Capability: capability, Message: message}
}

// WithCause adds an underlying cause to the CapabilityError.
func (e *CapabilityError) WithCause(cause error) *CapabilityError {
	e.Cause = cause
	return e
}

// PluginError represents errors related to plugin execution and communication.
type PluginError struct {
	Plugin    string
	Operation string // e.g., "Start", "Handshake", "Dial"
	Message   string
	Cause     error
}

// Error implements the error interface.
func (e *PluginError) Error() string {
	if e.Plugin != "" {
		return fmt.Sprintf("plugin %s %s failed: %s", e.Plugin, e.Operation, e.Message)
	}
	return fmt.Sprintf("plugin %s failed: %s", e.Operation, e.Message)
}

// Unwrap returns the underlying cause for error chain traversal.
func (e *PluginError) Unwrap() error {
	return e.Cause
}

// NewPluginError creates a new PluginError.
func NewPluginError(plugin, operation, message string) *PluginError {
	return &PluginError{Plugin: plugin, Operation: operation, Message: message}
}

// WithCause adds an underlying cause to the PluginError.
func (e *PluginError) WithCause(cause error) *PluginError {
	e.Cause = cause
	return e
}

// DaemonError represents errors related to the Rig background daemon.
type DaemonError struct {
	Operation string // e.g., "Connect", "Execute", "Status"
	Message   string
	Fallback  bool // If true, caller should fall back to direct execution
	Cause     error
}

// Error implements the error interface.
func (e *DaemonError) Error() string {
	if e.Operation != "" {
		return fmt.Sprintf("daemon %s failed: %s", e.Operation, e.Message)
	}
	return "daemon error: " + e.Message
}

// Unwrap returns the underlying cause for error chain traversal.
func (e *DaemonError) Unwrap() error {
	return e.Cause
}

// NewDaemonError creates a new DaemonError.
func NewDaemonError(operation, message string) *DaemonError {
	return &DaemonError{Operation: operation, Message: message}
}

// WithCause adds an underlying cause to the DaemonError.
func (e *DaemonError) WithCause(cause error) *DaemonError {
	e.Cause = cause
	return e
}

// DatabaseError represents errors related to database operations,
// specifically targeting Dolt concurrency and locking issues.
type DatabaseError struct {
	Operation string // e.g., "CreateWorkflow", "UpdateStatus"
	Message   string
	Code      int  // MySQL/Dolt error code if applicable (e.g., 1213)
	Retryable bool // Whether the operation should be retried
	Cause     error
}

// Error implements the error interface.
func (e *DatabaseError) Error() string {
	if e.Code > 0 {
		return fmt.Sprintf("database %s failed (code %d): %s", e.Operation, e.Code, e.Message)
	}
	return fmt.Sprintf("database %s failed: %s", e.Operation, e.Message)
}

// Unwrap returns the underlying cause for error chain traversal.
func (e *DatabaseError) Unwrap() error {
	return e.Cause
}

// NewDatabaseError creates a new DatabaseError.
func NewDatabaseError(operation, message string, code int) *DatabaseError {
	// Detect if this is a known retryable Dolt error
	retryable := code == 1213 || code == 1205 // 1213: Deadlock, 1205: Lock wait timeout
	return &DatabaseError{
		Operation: operation,
		Message:   message,
		Code:      code,
		Retryable: retryable,
	}
}

// WithCause adds an underlying cause to the DatabaseError.
func (e *DatabaseError) WithCause(cause error) *DatabaseError {
	e.Cause = cause
	return e
}

// IsDoltSerializationError returns true if the error or its cause is a Dolt
// serialization failure (deadlock or lock wait timeout).
func IsDoltSerializationError(err error) bool {
	if err == nil {
		return false
	}
	var dbErr *DatabaseError
	if errors.As(err, &dbErr) {
		return dbErr.Code == 1213 || dbErr.Code == 1205
	}
	// Fallback to checking message if it's a raw SQL error we haven't wrapped yet
	msg := err.Error()
	return strings.Contains(msg, "Error 1213") || strings.Contains(msg, "Error 1205")
}

// IsRetryable checks if an error or any error in its chain is retryable.
// It returns true if the error itself is retryable, or if any wrapped error
// is marked as retryable.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Check GitHubError
	var ghErr *GitHubError
	if errors.As(err, &ghErr) {
		return ghErr.Retryable
	}

	// Check AIError
	var aiErr *AIError
	if errors.As(err, &aiErr) {
		return aiErr.Retryable
	}

	// Check JiraError
	var jiraErr *JiraError
	if errors.As(err, &jiraErr) {
		return jiraErr.Retryable
	}

	// Check BeadsError
	var beadsErr *BeadsError
	if errors.As(err, &beadsErr) {
		return beadsErr.Retryable
	}

	// Check WorkflowError
	var wfErr *WorkflowError
	if errors.As(err, &wfErr) {
		return wfErr.Retryable
	}

	// Check DatabaseError
	var dbErr *DatabaseError
	if errors.As(err, &dbErr) {
		return dbErr.Retryable
	}

	return false
}

// IsConfigError checks if an error or any error in its chain is a ConfigError.
func IsConfigError(err error) bool {
	var configErr *ConfigError
	return errors.As(err, &configErr)
}

// IsGitHubError checks if an error or any error in its chain is a GitHubError.
func IsGitHubError(err error) bool {
	var ghErr *GitHubError
	return errors.As(err, &ghErr)
}

// IsAIError checks if an error or any error in its chain is an AIError.
func IsAIError(err error) bool {
	var aiErr *AIError
	return errors.As(err, &aiErr)
}

// IsJiraError checks if an error or any error in its chain is a JiraError.
func IsJiraError(err error) bool {
	var jiraErr *JiraError
	return errors.As(err, &jiraErr)
}

// IsBeadsError checks if an error or any error in its chain is a BeadsError.
func IsBeadsError(err error) bool {
	var beadsErr *BeadsError
	return errors.As(err, &beadsErr)
}

// IsWorkflowError checks if an error or any error in its chain is a WorkflowError.
func IsWorkflowError(err error) bool {
	var wfErr *WorkflowError
	return errors.As(err, &wfErr)
}

// IsCapabilityError checks if an error or any error in its chain is a CapabilityError.
func IsCapabilityError(err error) bool {
	var capErr *CapabilityError
	return errors.As(err, &capErr)
}

// IsPluginError checks if an error or any error in its chain is a PluginError.
func IsPluginError(err error) bool {
	var pluginErr *PluginError
	return errors.As(err, &pluginErr)
}

// IsDaemonError checks if an error or any error in its chain is a DaemonError.
func IsDaemonError(err error) bool {
	var daemonErr *DaemonError
	return errors.As(err, &daemonErr)
}

// IsDatabaseError checks if an error or any error in its chain is a DatabaseError.
func IsDatabaseError(err error) bool {
	var dbErr *DatabaseError
	return errors.As(err, &dbErr)
}

// SplitBrainError represents an inconsistent state where the keychain was updated
// but the configuration write failed, and the subsequent rollback also failed.
type SplitBrainError struct {
	Key         string // Config key (e.g., "jira.token")
	Service     string // Keychain service (e.g., "rig")
	Account     string // Keychain account (e.g., "jira.token")
	PrimaryErr  error  // Why the config write failed
	RollbackErr error  // Why the rollback failed
	IsNew       bool   // Whether the keychain entry was newly created
	PriorConfig string // The config value before the failed write
}

// Error implements the error interface with a concise single-line message.
func (e *SplitBrainError) Error() string {
	return fmt.Sprintf("split-brain: config key %q inconsistent (write failed: %v, rollback failed: %v)", e.Key, e.PrimaryErr, e.RollbackErr)
}

// RecoveryInstructions returns multi-line manual recovery instructions for the user.
func (e *SplitBrainError) RecoveryInstructions() string {
	var sb strings.Builder
	sb.WriteString("MANUAL RECOVERY REQUIRED:\n")

	if e.IsNew {
		sb.WriteString("The keychain entry for this key was newly created, but the configuration write failed.\n")
		sb.WriteString("The attempt to delete the new entry ALSO failed.\n\n")
		sb.WriteString("CRITICAL: The keychain entry may now contain the NEW secret, but your config does not reference it.\n")
		sb.WriteString("To resolve this, manually delete the keychain entry and try again:\n\n")
		fmt.Fprintf(&sb, "  macOS: security delete-generic-password -s %q -a %q\n", e.Service, e.Account)
		fmt.Fprintf(&sb, "  Linux: secret-tool clear service %q account %q\n", e.Service, e.Account)
	} else {
		sb.WriteString("The keychain entry for this key was updated, but the configuration write failed.\n")
		sb.WriteString("The attempt to restore the previous value ALSO failed.\n\n")
		sb.WriteString("CRITICAL: The keychain entry now contains the NEW secret.\n")

		uri := "keychain://" + e.Service + "/" + e.Account
		switch e.PriorConfig {
		case uri:
			sb.WriteString("The system already references this keychain entry, so it is now using the NEW secret stored there.\n")
			sb.WriteString("If this is acceptable, no further action is required.\n\n")
		case "":
			sb.WriteString("The user configuration file does not currently contain this key (it may be inherited from defaults, env, or project config).\n")
			sb.WriteString("The keychain entry is now inconsistent with the active configuration.\n\n")
		default:
			sb.WriteString("The system currently resolves this key to a DIFFERENT value in your configuration.\n")
			sb.WriteString("DO NOT delete the keychain entry unless you have a backup of the credential.\n\n")
		}

		sb.WriteString("To resolve this, either:\n")
		sb.WriteString("1. Manually update your user config file to reference the new secret (set to: " + uri + ").\n")
		sb.WriteString("2. Manually restore the OLD secret to the keychain using your system tools.\n")
	}

	return sb.String()
}

// Unwrap returns the primary error that triggered the failure.
func (e *SplitBrainError) Unwrap() error {
	return e.PrimaryErr
}

// NewSplitBrainError creates a new SplitBrainError.
func NewSplitBrainError(key, service, account string, primaryErr, rollbackErr error, isNew bool, priorConfig string) *SplitBrainError {
	return &SplitBrainError{
		Key:         key,
		Service:     service,
		Account:     account,
		PrimaryErr:  primaryErr,
		RollbackErr: rollbackErr,
		IsNew:       isNew,
		PriorConfig: priorConfig,
	}
}

// IsSplitBrainError checks if an error or any error in its chain is a SplitBrainError.
func IsSplitBrainError(err error) bool {
	var sbErr *SplitBrainError
	return errors.As(err, &sbErr)
}

// isRetryableHTTPStatus returns true for HTTP status codes that are typically retryable.
func isRetryableHTTPStatus(statusCode int) bool {
	switch statusCode {
	case 408, // Request Timeout
		429, // Too Many Requests
		500, // Internal Server Error
		502, // Bad Gateway
		503, // Service Unavailable
		504: // Gateway Timeout
		return true
	default:
		return false
	}
}

// Re-export commonly used functions from cockroachdb/errors for convenience.
// This allows consumers to use rigerrors.Wrap() instead of importing two packages.
var (
	// New creates a new error with the given message.
	New = errors.New

	// Newf creates a new error with formatted message.
	Newf = errors.Newf

	// Wrap wraps an error with additional context.
	Wrap = errors.Wrap

	// Wrapf wraps an error with formatted additional context.
	Wrapf = errors.Wrapf

	// Is reports whether any error in err's chain matches target.
	Is = errors.Is

	// As finds the first error in err's chain that matches target.
	As = errors.As

	// Cause returns the root cause of an error.
	Cause = errors.Cause

	// CombineErrors combines two errors into one.
	CombineErrors = errors.CombineErrors
)
