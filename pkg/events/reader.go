package events

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/cockroachdb/errors"
)

// QueryEventsByTicket retrieves workflow events tagged with a specific ticket ID.
func (dm *DatabaseManager) QueryEventsByTicket(ctx context.Context, ticket string) ([]WorkflowEvent, error) {
	// We use JSON_EXTRACT to filter by the ticket tag in metadata.
	// Fallback to LIKE if JSON_EXTRACT is not available in older Dolt versions.
	query := `
		SELECT id, correlation_id, step, status, message, metadata, created_at
		FROM workflow_events
		WHERE JSON_EXTRACT(metadata, '$.ticket') = ?
		ORDER BY created_at ASC
	`
	rows, err := dm.db.QueryContext(ctx, query, ticket)
	if err != nil {
		// Fallback for older Dolt versions without JSON_EXTRACT support
		fallbackQuery := `
			SELECT id, correlation_id, step, status, message, metadata, created_at
			FROM workflow_events
			WHERE metadata LIKE ?
			ORDER BY created_at ASC
		`
		pattern := fmt.Sprintf("%%\"ticket\":\"%s\"%%", ticket)
		rows, err = dm.db.QueryContext(ctx, fallbackQuery, pattern)
		if err != nil {
			return nil, errors.Wrap(err, "failed to query events by ticket")
		}
	}
	defer rows.Close()

	var events []WorkflowEvent
	for rows.Next() {
		var e WorkflowEvent
		var metadata []byte
		if err := rows.Scan(&e.ID, &e.CorrelationID, &e.Step, &e.Status, &e.Message, &metadata, &e.CreatedAt); err != nil {
			return nil, errors.Wrap(err, "failed to scan workflow event")
		}
		e.Metadata = metadata
		events = append(events, e)
	}

	return events, nil
}

// DoltDiffEntry represents a change in the workflow_events table as reported by dolt_diff.
type DoltDiffEntry struct {
	DiffType  string    `json:"diff_type"`
	Step      string    `json:"step"`
	Status    string    `json:"status"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

// QueryDiffForCorrelation retrieves the diffs associated with a workflow completion commit.
func (dm *DatabaseManager) QueryDiffForCorrelation(ctx context.Context, correlationID string) ([]DoltDiffEntry, error) {
	// 1. Find the commit that completed this workflow.
	// Our LogWorkflowCompleted uses the message: "Workflow <correlationID> completed"
	commitMsg := fmt.Sprintf("Workflow %s completed", correlationID)
	var commitHash string
	err := dm.db.QueryRowContext(ctx, "SELECT commit_hash FROM dolt_log WHERE message = ? LIMIT 1", commitMsg).Scan(&commitHash)
	if err != nil {
		// No commit found for this workflow completion - return nil gracefully.
		return nil, nil
	}

	// 2. Query the diff between this commit and its parent for workflow_events.
	// Dolt system table dolt_diff_workflow_events provides this.
	query := `
		SELECT diff_type, to_step, to_status, to_message, to_created_at
		FROM dolt_diff_workflow_events
		WHERE to_commit = ?
		ORDER BY to_created_at ASC
	`
	rows, err := dm.db.QueryContext(ctx, query, commitHash)
	if err != nil {
		// If system table doesn't exist or other error, return nil gracefully.
		if dm.Verbose {
			fmt.Printf("Warning: failed to query dolt_diff_workflow_events: %v\n", err)
		}
		return nil, nil
	}
	defer rows.Close()

	var diffs []DoltDiffEntry
	for rows.Next() {
		var d DoltDiffEntry
		var step, status, msg sql.NullString
		var createdAt sql.NullTime
		if err := rows.Scan(&d.DiffType, &step, &status, &msg, &createdAt); err != nil {
			return nil, errors.Wrap(err, "failed to scan diff entry")
		}
		d.Step = step.String
		d.Status = status.String
		d.Message = msg.String
		if createdAt.Valid {
			d.CreatedAt = createdAt.Time
		}
		diffs = append(diffs, d)
	}

	return diffs, nil
}
