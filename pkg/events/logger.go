package events

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

// EventLogger defines the interface for logging workflow events.
type EventLogger interface {
	LogStepStarted(ctx context.Context, correlationID, step string) error
	LogStepCompleted(ctx context.Context, correlationID, step string) error
	LogStepFailed(ctx context.Context, correlationID, step, errMsg string) error
	LogWorkflowCompleted(ctx context.Context, correlationID string) error
	LogWorkflowFailed(ctx context.Context, correlationID, errMsg string) error
	Close() error
}

// DoltEventLogger is a Dolt-backed implementation of EventLogger.
type DoltEventLogger struct {
	dm     *DatabaseManager
	ticket string
	mu     sync.RWMutex
}

// NewDoltEventLogger creates a new DoltEventLogger.
func NewDoltEventLogger(dm *DatabaseManager) *DoltEventLogger {
	return &DoltEventLogger{dm: dm}
}

// SetTicket sets the ticket ID for metadata tagging.
func (l *DoltEventLogger) SetTicket(ticket string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.ticket = ticket
}

// BackfillTicket retroactively tags events with a ticket ID.
func (l *DoltEventLogger) BackfillTicket(ctx context.Context, correlationID, ticket string) error {
	return l.dm.BackfillTicket(ctx, correlationID, ticket)
}

func (l *DoltEventLogger) LogStepStarted(ctx context.Context, correlationID, step string) error {
	return l.log(ctx, correlationID, step, "STARTED", "")
}

func (l *DoltEventLogger) LogStepCompleted(ctx context.Context, correlationID, step string) error {
	return l.log(ctx, correlationID, step, "COMPLETED", "")
}

func (l *DoltEventLogger) LogStepFailed(ctx context.Context, correlationID, step, errMsg string) error {
	return l.log(ctx, correlationID, step, "FAILED", errMsg)
}

func (l *DoltEventLogger) LogWorkflowCompleted(ctx context.Context, correlationID string) error {
	if err := l.log(ctx, correlationID, "workflow", "COMPLETED", ""); err != nil {
		return err
	}
	return l.commitEvents(ctx, fmt.Sprintf("Workflow %s completed", correlationID))
}

func (l *DoltEventLogger) LogWorkflowFailed(ctx context.Context, correlationID, errMsg string) error {
	if err := l.log(ctx, correlationID, "workflow", "FAILED", errMsg); err != nil {
		return err
	}
	return l.commitEvents(ctx, fmt.Sprintf("Workflow %s failed", correlationID))
}

// commitEvents stages all changes and creates a Dolt commit.
func (l *DoltEventLogger) commitEvents(ctx context.Context, msg string) error {
	if _, err := l.dm.db.ExecContext(ctx, "CALL DOLT_ADD('-A')"); err != nil {
		return errors.Wrap(err, "failed to CALL DOLT_ADD")
	}
	if _, err := l.dm.db.ExecContext(ctx, "CALL DOLT_COMMIT('-m', ?)", msg); err != nil {
		return errors.Wrap(err, "failed to CALL DOLT_COMMIT")
	}
	return nil
}

func (l *DoltEventLogger) Close() error {
	return l.dm.Close()
}

func (l *DoltEventLogger) log(ctx context.Context, correlationID, step, status, msg string) error {
	l.mu.RLock()
	ticket := l.ticket
	l.mu.RUnlock()

	var metadata any
	if ticket != "" {
		m := map[string]string{"ticket": ticket}
		b, err := json.Marshal(m)
		if err != nil {
			return errors.Wrap(err, "failed to marshal metadata")
		}
		metadata = string(b)
	}

	id := uuid.New().String()
	query := `INSERT INTO workflow_events (id, correlation_id, step, status, message, metadata) VALUES (?, ?, ?, ?, ?, ?)`
	_, err := l.dm.db.ExecContext(ctx, query, id, correlationID, step, status, msg, metadata)
	if err != nil {
		return errors.Wrapf(err, "failed to log event %s:%s", step, status)
	}
	return nil
}

// NoopEventLogger is a no-op implementation of EventLogger.
type NoopEventLogger struct{}

func (l NoopEventLogger) LogStepStarted(ctx context.Context, correlationID, step string) error {
	return nil
}

func (l NoopEventLogger) LogStepCompleted(ctx context.Context, correlationID, step string) error {
	return nil
}

func (l NoopEventLogger) LogStepFailed(ctx context.Context, correlationID, step, errMsg string) error {
	return nil
}

func (l NoopEventLogger) LogWorkflowCompleted(ctx context.Context, correlationID string) error {
	return nil
}

func (l NoopEventLogger) LogWorkflowFailed(ctx context.Context, correlationID, errMsg string) error {
	return nil
}

func (l NoopEventLogger) Close() error {
	return nil
}
