package events

import (
	"encoding/json"
	"time"
)

// WorkflowEvent represents a single step transition in a workflow.
type WorkflowEvent struct {
	ID            string          `json:"id"`
	CorrelationID string          `json:"correlation_id"`
	Step          string          `json:"step"`
	Status        string          `json:"status"`
	Message       string          `json:"message,omitempty"`
	Metadata      json.RawMessage `json:"metadata,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
}

// PruneResult contains the results of a database pruning operation.
type PruneResult struct {
	EventsDeleted int
	CutoffTime    time.Time
}
