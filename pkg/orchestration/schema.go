package orchestration

// Schema definitions for the orchestration engine.
// We use MySQL-compatible SQL for Dolt.

const (
	// WorkflowsTableDDL defines the workflows table.
	WorkflowsTableDDL = `
CREATE TABLE IF NOT EXISTS workflows (
    id VARCHAR(36) PRIMARY KEY,
    name VARCHAR(255) UNIQUE NOT NULL,
    description TEXT,
    version INT DEFAULT 1,
    status ENUM('DRAFT', 'ACTIVE', 'DEPRECATED') DEFAULT 'DRAFT',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);`

	// NodesTableDDL defines the nodes table.
	NodesTableDDL = `
CREATE TABLE IF NOT EXISTS nodes (
    id VARCHAR(36) PRIMARY KEY,
    workflow_id VARCHAR(36) NOT NULL,
    workflow_version INT NOT NULL,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(50) NOT NULL,
    config JSON,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_node_workflow FOREIGN KEY (workflow_id) REFERENCES workflows(id) ON DELETE CASCADE,
    UNIQUE(workflow_id, workflow_version, name)
);`

	// EdgesTableDDL defines the edges table.
	EdgesTableDDL = `
CREATE TABLE IF NOT EXISTS edges (
    id VARCHAR(36) PRIMARY KEY,
    workflow_id VARCHAR(36) NOT NULL,
    workflow_version INT NOT NULL,
    source_node_id VARCHAR(36) NOT NULL,
    target_node_id VARCHAR(36) NOT NULL,
    condition TEXT,
    CONSTRAINT fk_edge_workflow FOREIGN KEY (workflow_id) REFERENCES workflows(id) ON DELETE CASCADE,
    CONSTRAINT fk_edge_source FOREIGN KEY (source_node_id) REFERENCES nodes(id) ON DELETE CASCADE,
    CONSTRAINT fk_edge_target FOREIGN KEY (target_node_id) REFERENCES nodes(id) ON DELETE CASCADE,
    UNIQUE(workflow_id, workflow_version, source_node_id, target_node_id)
);`

	// ExecutionsTableDDL defines the executions table.
	ExecutionsTableDDL = `
CREATE TABLE IF NOT EXISTS executions (
    id VARCHAR(36) PRIMARY KEY,
    workflow_id VARCHAR(36) NOT NULL,
    workflow_version INT NOT NULL,
    status ENUM('PENDING', 'RUNNING', 'SUCCESS', 'FAILED', 'CANCELLED') DEFAULT 'PENDING',
    started_at TIMESTAMP NULL,
    completed_at TIMESTAMP NULL,
    error TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_execution_workflow FOREIGN KEY (workflow_id) REFERENCES workflows(id) ON DELETE CASCADE,
    INDEX idx_workflow_status (workflow_id, status)
);`

	// NodeStatesTableDDL defines the node_states table.
	NodeStatesTableDDL = `
CREATE TABLE IF NOT EXISTS node_states (
    id VARCHAR(36) PRIMARY KEY,
    execution_id VARCHAR(36) NOT NULL,
    node_id VARCHAR(36) NOT NULL,
    status ENUM('PENDING', 'RUNNING', 'SUCCESS', 'FAILED', 'SKIPPED') DEFAULT 'PENDING',
    started_at TIMESTAMP NULL,
    completed_at TIMESTAMP NULL,
    result JSON,
    error TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_nodestate_execution FOREIGN KEY (execution_id) REFERENCES executions(id) ON DELETE CASCADE,
    CONSTRAINT fk_nodestate_node FOREIGN KEY (node_id) REFERENCES nodes(id),
    UNIQUE(execution_id, node_id)
);`
)

// AllTableDDLs returns all DDL statements in order of creation to satisfy dependencies.
func AllTableDDLs() []string {
	return []string{
		WorkflowsTableDDL,
		NodesTableDDL,
		EdgesTableDDL,
		ExecutionsTableDDL,
		NodeStatesTableDDL,
	}
}
