package events

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
)

// likeEscaper escapes SQL LIKE metacharacters to prevent wildcard injection.
var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// QueryEventsByTicket retrieves workflow events tagged with a specific ticket ID, optionally filtered by time.
func (dm *DatabaseManager) QueryEventsByTicket(ctx context.Context, ticket string, since, until time.Time) ([]WorkflowEvent, error) {
	query := `
		SELECT id, correlation_id, step, status, message, metadata, created_at
		FROM workflow_events
		WHERE JSON_EXTRACT(metadata, '$.ticket') = ?
	`
	var args []interface{}
	args = append(args, ticket)

	if !since.IsZero() {
		query += " AND created_at >= ?"
		args = append(args, since)
	}
	if !until.IsZero() {
		query += " AND created_at <= ?"
		args = append(args, until)
	}

	query += " ORDER BY created_at ASC, id ASC"

	rows, err := dm.db.QueryContext(ctx, query, args...)
	if err != nil {
		// Only fall back to LIKE for JSON_EXTRACT-related errors (function not supported).
		// Propagate all other errors (connection, context cancellation, missing table).
		errMsg := err.Error()
		if !strings.Contains(errMsg, "JSON_EXTRACT") || !strings.Contains(errMsg, "no such function") {
			return nil, errors.Wrap(err, "failed to query events by ticket")
		}

		fallbackQuery := `
			SELECT id, correlation_id, step, status, message, metadata, created_at
			FROM workflow_events
			WHERE metadata LIKE ? ESCAPE '\'
		`
		escaped := likeEscaper.Replace(ticket)
		pattern := fmt.Sprintf(`%%"ticket":"%s"%%`, escaped)

		var fallbackArgs []interface{}
		fallbackArgs = append(fallbackArgs, pattern)

		if !since.IsZero() {
			fallbackQuery += " AND created_at >= ?"
			fallbackArgs = append(fallbackArgs, since)
		}
		if !until.IsZero() {
			fallbackQuery += " AND created_at <= ?"
			fallbackArgs = append(fallbackArgs, until)
		}

		fallbackQuery += " ORDER BY created_at ASC, id ASC"

		rows, err = dm.db.QueryContext(ctx, fallbackQuery, fallbackArgs...)
		if err != nil {
			return nil, errors.Wrap(err, "failed to query events by ticket (LIKE fallback)")
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
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "error iterating workflow events")
	}

	return events, nil
}

// QueryEventsByTimeRange retrieves workflow events within the specified time range.
func (dm *DatabaseManager) QueryEventsByTimeRange(ctx context.Context, since, until time.Time) ([]WorkflowEvent, error) {
	query := `
		SELECT id, correlation_id, step, status, message, metadata, created_at
		FROM workflow_events
		WHERE created_at BETWEEN ? AND ?
		ORDER BY created_at ASC, id ASC
	`
	rows, err := dm.db.QueryContext(ctx, query, since, until)
	if err != nil {
		return nil, errors.Wrap(err, "failed to query events by time range")
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
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "error iterating workflow events")
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
	commitMsg := "events: Workflow " + correlationID + " completed"
	var commitHash string
	err := dm.db.QueryRowContext(ctx, "SELECT commit_hash FROM dolt_log WHERE message = ? ORDER BY date DESC LIMIT 1", commitMsg).Scan(&commitHash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, errors.Wrap(err, "failed to query dolt_log for commit hash")
	}

	query := `
		SELECT diff_type, to_step, to_status, to_message, to_created_at
		FROM dolt_diff_workflow_events
		WHERE to_commit = ? AND to_correlation_id = ?
		ORDER BY to_created_at ASC
	`
	rows, err := dm.db.QueryContext(ctx, query, commitHash, correlationID)
	if err != nil {
		// The dolt_diff_* system table may not exist on fresh databases with no commits.
		// Treat table-not-found as a graceful no-op; propagate all other errors.
		errMsg := err.Error()
		if strings.Contains(errMsg, "table not found") || strings.Contains(errMsg, "doesn't exist") {
			return nil, nil
		}
		return nil, errors.Wrap(err, "failed to query dolt_diff_workflow_events")
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
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "error iterating diff entries")
	}

	return diffs, nil
}
