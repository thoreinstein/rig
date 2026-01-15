// Package beads provides integration with the beads issue tracking system.
//
// Beads is a lightweight, file-based issue tracker that stores issues in
// .beads/beads.jsonl. This package provides a Go client for interacting
// with the bd CLI tool.
package beads

// IssueInfo holds beads issue information returned from bd show --json.
type IssueInfo struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Type        string   `json:"type"`     // task, bug, feature, epic
	Status      string   `json:"status"`   // open, in_progress, closed
	Priority    int      `json:"priority"` // 0-4 (0=critical, 4=backlog)
	Labels      []string `json:"labels"`
	Description string   `json:"description"`
	Owner       string   `json:"owner"`
}

// ValidStatuses defines the allowed status values for beads issues.
var ValidStatuses = []string{"open", "in_progress", "closed"}

// ValidTypes defines the allowed type values for beads issues.
var ValidTypes = []string{"task", "bug", "feature", "epic"}

// IsValidStatus checks if a status string is a valid beads status.
func IsValidStatus(status string) bool {
	for _, s := range ValidStatuses {
		if s == status {
			return true
		}
	}
	return false
}

// IsValidType checks if a type string is a valid beads type.
func IsValidType(issueType string) bool {
	for _, t := range ValidTypes {
		if t == issueType {
			return true
		}
	}
	return false
}
