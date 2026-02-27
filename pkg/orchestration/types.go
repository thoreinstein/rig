package orchestration

import (
	"encoding/json"
	"time"
)

// WorkflowStatus identifies the current state of a workflow definition.
type WorkflowStatus string

const (
	// WorkflowStatusDraft indicates the workflow is being defined and not yet ready for execution.
	WorkflowStatusDraft WorkflowStatus = "DRAFT"
	// WorkflowStatusActive indicates the workflow is ready for execution.
	WorkflowStatusActive WorkflowStatus = "ACTIVE"
	// WorkflowStatusDeprecated indicates the workflow is no longer recommended for new executions.
	WorkflowStatusDeprecated WorkflowStatus = "DEPRECATED"
)

// ExecutionStatus identifies the current state of a workflow run.
type ExecutionStatus string

const (
	// ExecutionStatusPending indicates the execution is queued but not yet started.
	ExecutionStatusPending ExecutionStatus = "PENDING"
	// ExecutionStatusRunning indicates the execution is currently in progress.
	ExecutionStatusRunning ExecutionStatus = "RUNNING"
	// ExecutionStatusSuccess indicates the execution completed successfully.
	ExecutionStatusSuccess ExecutionStatus = "SUCCESS"
	// ExecutionStatusFailed indicates the execution terminated with an error.
	ExecutionStatusFailed ExecutionStatus = "FAILED"
	// ExecutionStatusCancelled indicates the execution was manually stopped.
	ExecutionStatusCancelled ExecutionStatus = "CANCELLED"
)

// NodeStatus identifies the current state of a node within an execution.
type NodeStatus string

const (
	// NodeStatusPending indicates the node is waiting for dependencies to complete.
	NodeStatusPending NodeStatus = "PENDING"
	// NodeStatusRunning indicates the node is currently executing its task.
	NodeStatusRunning NodeStatus = "RUNNING"
	// NodeStatusSuccess indicates the node task completed successfully.
	NodeStatusSuccess NodeStatus = "SUCCESS"
	// NodeStatusFailed indicates the node task failed.
	NodeStatusFailed NodeStatus = "FAILED"
	// NodeStatusSkipped indicates the node was not executed due to conditions or failures.
	NodeStatusSkipped NodeStatus = "SKIPPED"
)

// Workflow represents a versioned DAG definition.
type Workflow struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Version     int            `json:"version"`
	Status      WorkflowStatus `json:"status"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// Node represents a single task/step in a workflow.
type Node struct {
	ID              string          `json:"id"`
	WorkflowID      string          `json:"workflow_id"`
	WorkflowVersion int             `json:"workflow_version"`
	Name            string          `json:"name"`
	Type            string          `json:"type"`
	Config          json.RawMessage `json:"config"`
	CreatedAt       time.Time       `json:"created_at"`
}

// Edge represents a dependency relationship between nodes in a DAG.
type Edge struct {
	ID              string `json:"id"`
	WorkflowID      string `json:"workflow_id"`
	WorkflowVersion int    `json:"workflow_version"`
	SourceNodeID    string `json:"source_node_id"`
	TargetNodeID    string `json:"target_node_id"`
	Condition       string `json:"condition,omitempty"`
}

// Execution represents a single run of a workflow.
type Execution struct {
	ID              string          `json:"id"`
	WorkflowID      string          `json:"workflow_id"`
	WorkflowVersion int             `json:"workflow_version"`
	Status          ExecutionStatus `json:"status"`
	StartedAt       *time.Time      `json:"started_at,omitempty"`
	CompletedAt     *time.Time      `json:"completed_at,omitempty"`
	Error           string          `json:"error,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
}

// NodeState tracks the status and results of a node within a specific execution.
type NodeState struct {
	ID          string          `json:"id"`
	ExecutionID string          `json:"execution_id"`
	NodeID      string          `json:"node_id"`
	Status      NodeStatus      `json:"status"`
	StartedAt   *time.Time      `json:"started_at,omitempty"`
	CompletedAt *time.Time      `json:"completed_at,omitempty"`
	Result      json.RawMessage `json:"result,omitempty"`
	Error       string          `json:"error,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
}

// DoltCommitInfo represents metadata for a Dolt version commit.
type DoltCommitInfo struct {
	Hash      string    `json:"hash"`
	Author    string    `json:"author"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}
