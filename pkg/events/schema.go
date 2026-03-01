package events

// Schema definitions for the embedded Dolt event store.

const (
	// WorkflowEventsTableDDL defines the workflow_events table.
	WorkflowEventsTableDDL = `
CREATE TABLE IF NOT EXISTS workflow_events (
    id VARCHAR(36) PRIMARY KEY,
    correlation_id VARCHAR(36) NOT NULL,
    step VARCHAR(50) NOT NULL,
    status ENUM('STARTED', 'COMPLETED', 'FAILED') NOT NULL,
    message TEXT,
    metadata JSON,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_correlation_step (correlation_id, step)
);`
)

// AllTableDDLs returns all DDL statements in order of creation.
func AllTableDDLs() []string {
	return []string{
		WorkflowEventsTableDDL,
	}
}
