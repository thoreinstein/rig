// Package debrief provides an AI-powered debrief system for development work.
//
// The debrief system aggregates context from multiple sources (GitHub PRs, Jira tickets,
// git history, shell commands) and conducts an interactive Q&A session with the developer
// to generate a structured summary of the work completed, decisions made, and lessons learned.
package debrief

import "time"

// Context holds all aggregated context for the debrief session.
// Fields may be empty if the corresponding source is not available.
type Context struct {
	// From GitHub
	PRTitle      string
	PRBody       string
	PRComments   []string
	Commits      []CommitSummary
	FilesChanged []string

	// From Jira
	TicketID          string
	TicketSummary     string
	TicketType        string
	TicketDescription string

	// From Git
	BranchName string
	BaseBranch string
	DiffStats  DiffStats

	// From Notes (if available)
	ExistingNotes string

	// From Shell History (if available)
	RelevantCommands []string

	// Metadata
	Duration  time.Duration // How long the work took
	StartedAt time.Time
}

// CommitSummary represents a single commit in the PR.
type CommitSummary struct {
	SHA     string
	Message string
	Author  string
	Date    time.Time
}

// DiffStats summarizes the code changes in the PR.
type DiffStats struct {
	FilesChanged int
	Insertions   int
	Deletions    int
}

// Session represents an active debrief session.
type Session struct {
	Context   *Context
	Questions []Question
	Answers   map[string]string
	Summary   string
	StartedAt time.Time
}

// Question represents a question for the debrief.
type Question struct {
	ID       string
	Text     string
	Purpose  string // Why we're asking this
	Required bool
}

// Output is the final debrief output.
type Output struct {
	Summary        string
	KeyDecisions   []string
	Challenges     []string
	LessonsLearned []string
	FollowUps      []string
	GeneratedAt    time.Time
}
