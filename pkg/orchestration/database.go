package orchestration

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/cockroachdb/errors"
	_ "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
)

// DatabaseManager handles Dolt database operations for the orchestration engine.
type DatabaseManager struct {
	db      *sql.DB
	Verbose bool
}

// NewDatabaseManager creates a new DatabaseManager and establishes a connection.
// DSN example: "user:password@tcp(127.0.0.1:3306)/database?parseTime=true"
func NewDatabaseManager(dsn string, verbose bool) (*DatabaseManager, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open dolt database")
	}

	// Verify connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, errors.Wrap(err, "failed to ping dolt database")
	}

	return &DatabaseManager{
		db:      db,
		Verbose: verbose,
	}, nil
}

// Close closes the database connection.
func (dm *DatabaseManager) Close() error {
	if dm.db != nil {
		return dm.db.Close()
	}
	return nil
}

// IsAvailable checks if the database is accessible.
func (dm *DatabaseManager) IsAvailable() bool {
	if dm.db == nil {
		return false
	}
	err := dm.db.Ping()
	if err != nil {
		if dm.Verbose {
			fmt.Printf("Dolt database not available: %v\n", err)
		}
		return false
	}
	return true
}

// InitSchema initializes the database tables if they don't exist.
func (dm *DatabaseManager) InitSchema() error {
	if !dm.IsAvailable() {
		return errors.New("database not available")
	}

	for _, ddl := range AllTableDDLs() {
		if dm.Verbose {
			fmt.Printf("Executing DDL:\n%s\n", ddl)
		}
		if _, err := dm.db.Exec(ddl); err != nil {
			return errors.Wrapf(err, "failed to execute DDL: %s", ddl)
		}
	}

	return nil
}

// --- Phase 3: Workflow Definition CRUD + Dolt Versioning ---

// CreateWorkflow inserts a new workflow record.
func (dm *DatabaseManager) CreateWorkflow(ctx context.Context, w *Workflow) error {
	if w.ID == "" {
		w.ID = uuid.New().String()
	}
	if w.Status == "" {
		w.Status = WorkflowStatusDraft
	}

	query := `INSERT INTO workflows (id, name, description, version, status, created_at, updated_at)
	          VALUES (?, ?, ?, ?, ?, ?, ?)`
	now := time.Now()
	if w.CreatedAt.IsZero() {
		w.CreatedAt = now
	}
	if w.UpdatedAt.IsZero() {
		w.UpdatedAt = now
	}

	_, err := dm.db.ExecContext(ctx, query, w.ID, w.Name, w.Description, w.Version, w.Status, w.CreatedAt, w.UpdatedAt)
	if err != nil {
		return errors.Wrap(err, "failed to insert workflow")
	}

	return dm.autoCommit(ctx, "Create workflow: "+w.Name)
}

// GetWorkflow retrieves a workflow by ID.
func (dm *DatabaseManager) GetWorkflow(ctx context.Context, id string) (*Workflow, error) {
	query := `SELECT id, name, description, version, status, created_at, updated_at FROM workflows WHERE id = ?`
	w := &Workflow{}
	err := dm.db.QueryRowContext(ctx, query, id).Scan(
		&w.ID, &w.Name, &w.Description, &w.Version, &w.Status, &w.CreatedAt, &w.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("workflow not found")
		}
		return nil, errors.Wrap(err, "failed to get workflow")
	}
	return w, nil
}

// UpdateWorkflow updates an existing workflow and increments its version.
func (dm *DatabaseManager) UpdateWorkflow(ctx context.Context, w *Workflow) error {
	active, err := dm.HasActiveExecutions(ctx, w.ID)
	if err != nil {
		return err
	}
	if active {
		return errors.New("cannot update workflow with active executions")
	}

	w.Version++
	w.UpdatedAt = time.Now()

	query := `UPDATE workflows SET name = ?, description = ?, version = ?, status = ?, updated_at = ? WHERE id = ?`
	_, err = dm.db.ExecContext(ctx, query, w.Name, w.Description, w.Version, w.Status, w.UpdatedAt, w.ID)
	if err != nil {
		return errors.Wrap(err, "failed to update workflow")
	}

	return dm.autoCommit(ctx, fmt.Sprintf("Update workflow: %s (v%d)", w.Name, w.Version))
}

// ListWorkflows retrieves all workflows.
func (dm *DatabaseManager) ListWorkflows(ctx context.Context) ([]*Workflow, error) {
	query := `SELECT id, name, description, version, status, created_at, updated_at FROM workflows ORDER BY created_at DESC`
	rows, err := dm.db.QueryContext(ctx, query)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list workflows")
	}
	defer rows.Close()

	var workflows []*Workflow
	for rows.Next() {
		w := &Workflow{}
		if err := rows.Scan(&w.ID, &w.Name, &w.Description, &w.Version, &w.Status, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, errors.Wrap(err, "failed to scan workflow")
		}
		workflows = append(workflows, w)
	}
	return workflows, nil
}

// CreateNode inserts a new node record.
func (dm *DatabaseManager) CreateNode(ctx context.Context, n *Node) error {
	if n.ID == "" {
		n.ID = uuid.New().String()
	}
	if n.CreatedAt.IsZero() {
		n.CreatedAt = time.Now()
	}

	query := `INSERT INTO nodes (id, workflow_id, name, type, config, created_at) VALUES (?, ?, ?, ?, ?, ?)`
	_, err := dm.db.ExecContext(ctx, query, n.ID, n.WorkflowID, n.Name, n.Type, n.Config, n.CreatedAt)
	if err != nil {
		return errors.Wrap(err, "failed to insert node")
	}
	return nil
}

// GetNodesByWorkflow retrieves all nodes for a given workflow.
func (dm *DatabaseManager) GetNodesByWorkflow(ctx context.Context, workflowID string) ([]*Node, error) {
	query := `SELECT id, workflow_id, name, type, config, created_at FROM nodes WHERE workflow_id = ? ORDER BY created_at ASC`
	rows, err := dm.db.QueryContext(ctx, query, workflowID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get nodes")
	}
	defer rows.Close()

	var nodes []*Node
	for rows.Next() {
		n := &Node{}
		if err := rows.Scan(&n.ID, &n.WorkflowID, &n.Name, &n.Type, &n.Config, &n.CreatedAt); err != nil {
			return nil, errors.Wrap(err, "failed to scan node")
		}
		nodes = append(nodes, n)
	}
	return nodes, nil
}

// CreateEdge inserts a new edge record.
func (dm *DatabaseManager) CreateEdge(ctx context.Context, e *Edge) error {
	if e.ID == "" {
		e.ID = uuid.New().String()
	}

	query := `INSERT INTO edges (id, workflow_id, source_node_id, target_node_id, condition) VALUES (?, ?, ?, ?, ?)`
	_, err := dm.db.ExecContext(ctx, query, e.ID, e.WorkflowID, e.SourceNodeID, e.TargetNodeID, e.Condition)
	if err != nil {
		return errors.Wrap(err, "failed to insert edge")
	}
	return nil
}

// GetEdgesByWorkflow retrieves all edges for a given workflow.
func (dm *DatabaseManager) GetEdgesByWorkflow(ctx context.Context, workflowID string) ([]*Edge, error) {
	query := `SELECT id, workflow_id, source_node_id, target_node_id, condition FROM edges WHERE workflow_id = ?`
	rows, err := dm.db.QueryContext(ctx, query, workflowID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get edges")
	}
	defer rows.Close()

	var edges []*Edge
	for rows.Next() {
		e := &Edge{}
		if err := rows.Scan(&e.ID, &e.WorkflowID, &e.SourceNodeID, &e.TargetNodeID, &e.Condition); err != nil {
			return nil, errors.Wrap(err, "failed to scan edge")
		}
		edges = append(edges, e)
	}
	return edges, nil
}

// SaveWorkflowDefinition transactionally saves a full workflow definition and creates a Dolt commit.
func (dm *DatabaseManager) SaveWorkflowDefinition(ctx context.Context, w *Workflow, nodes []*Node, edges []*Edge) error {
	if w.ID != "" {
		active, err := dm.HasActiveExecutions(ctx, w.ID)
		if err != nil {
			return err
		}
		if active {
			return errors.New("cannot update workflow definition with active executions")
		}
	}

	tx, err := dm.db.BeginTx(ctx, nil)
	if err != nil {
		return errors.Wrap(err, "failed to begin transaction")
	}
	defer tx.Rollback() //nolint:errcheck

	// 1. Update/Create Workflow
	if w.ID == "" {
		w.ID = uuid.New().String()
		w.Version = 1
		w.CreatedAt = time.Now()
		w.UpdatedAt = w.CreatedAt
		query := `INSERT INTO workflows (id, name, description, version, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`
		if _, err := tx.ExecContext(ctx, query, w.ID, w.Name, w.Description, w.Version, w.Status, w.CreatedAt, w.UpdatedAt); err != nil {
			return errors.Wrap(err, "failed to insert workflow in tx")
		}
	} else {
		w.Version++
		w.UpdatedAt = time.Now()
		query := `UPDATE workflows SET name = ?, description = ?, version = ?, status = ?, updated_at = ? WHERE id = ?`
		if _, err := tx.ExecContext(ctx, query, w.Name, w.Description, w.Version, w.Status, w.UpdatedAt, w.ID); err != nil {
			return errors.Wrap(err, "failed to update workflow in tx")
		}
	}

	// 2. Clean existing nodes/edges if updating
	// This is a simplified approach: replace definition entirely.
	if _, err := tx.ExecContext(ctx, "DELETE FROM edges WHERE workflow_id = ?", w.ID); err != nil {
		return errors.Wrap(err, "failed to clean edges")
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM nodes WHERE workflow_id = ?", w.ID); err != nil {
		return errors.Wrap(err, "failed to clean nodes")
	}

	// 3. Insert new nodes
	for _, n := range nodes {
		n.WorkflowID = w.ID
		if n.ID == "" {
			n.ID = uuid.New().String()
		}
		if n.CreatedAt.IsZero() {
			n.CreatedAt = time.Now()
		}
		query := `INSERT INTO nodes (id, workflow_id, name, type, config, created_at) VALUES (?, ?, ?, ?, ?, ?)`
		if _, err := tx.ExecContext(ctx, query, n.ID, n.WorkflowID, n.Name, n.Type, n.Config, n.CreatedAt); err != nil {
			return errors.Wrap(err, "failed to insert node in tx")
		}
	}

	// 4. Insert new edges
	for _, e := range edges {
		e.WorkflowID = w.ID
		if e.ID == "" {
			e.ID = uuid.New().String()
		}
		query := `INSERT INTO edges (id, workflow_id, source_node_id, target_node_id, condition) VALUES (?, ?, ?, ?, ?)`
		if _, err := tx.ExecContext(ctx, query, e.ID, e.WorkflowID, e.SourceNodeID, e.TargetNodeID, e.Condition); err != nil {
			return errors.Wrap(err, "failed to insert edge in tx")
		}
	}

	if err := tx.Commit(); err != nil {
		return errors.Wrap(err, "failed to commit transaction")
	}

	return dm.autoCommit(ctx, fmt.Sprintf("Save workflow definition: %s (v%d)", w.Name, w.Version))
}

// --- Phase 4: Execution State Management ---

// CreateExecution inserts a new execution record.
func (dm *DatabaseManager) CreateExecution(ctx context.Context, e *Execution) error {
	if e.ID == "" {
		e.ID = uuid.New().String()
	}
	if e.Status == "" {
		e.Status = ExecutionStatusPending
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now()
	}

	query := `INSERT INTO executions (id, workflow_id, workflow_version, status, created_at) VALUES (?, ?, ?, ?, ?)`
	_, err := dm.db.ExecContext(ctx, query, e.ID, e.WorkflowID, e.WorkflowVersion, e.Status, e.CreatedAt)
	if err != nil {
		return errors.Wrap(err, "failed to insert execution")
	}
	return nil
}

// GetExecution retrieves an execution by ID.
func (dm *DatabaseManager) GetExecution(ctx context.Context, id string) (*Execution, error) {
	query := `SELECT id, workflow_id, workflow_version, status, started_at, completed_at, error, created_at FROM executions WHERE id = ?`
	e := &Execution{}
	err := dm.db.QueryRowContext(ctx, query, id).Scan(
		&e.ID, &e.WorkflowID, &e.WorkflowVersion, &e.Status, &e.StartedAt, &e.CompletedAt, &e.Error, &e.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("execution not found")
		}
		return nil, errors.Wrap(err, "failed to get execution")
	}
	return e, nil
}

// UpdateExecutionStatus transitions an execution to a new status.
// It also sets started_at or completed_at based on the status.
func (dm *DatabaseManager) UpdateExecutionStatus(ctx context.Context, id string, status ExecutionStatus) error {
	current, err := dm.GetExecution(ctx, id)
	if err != nil {
		return err
	}

	// Simple state machine validation
	if !isValidExecutionTransition(current.Status, status) {
		return errors.Errorf("invalid execution status transition from %s to %s", current.Status, status)
	}

	var query string
	var args []interface{}
	now := time.Now()

	switch status {
	case ExecutionStatusRunning:
		query = `UPDATE executions SET status = ?, started_at = ? WHERE id = ?`
		args = []interface{}{status, now, id}
	case ExecutionStatusSuccess, ExecutionStatusFailed, ExecutionStatusCancelled:
		query = `UPDATE executions SET status = ?, completed_at = ? WHERE id = ?`
		if status == ExecutionStatusFailed {
			// Error message should be set separately or we could add an argument
			query = `UPDATE executions SET status = ?, completed_at = ? WHERE id = ?`
		}
		args = []interface{}{status, now, id}
	default:
		query = `UPDATE executions SET status = ? WHERE id = ?`
		args = []interface{}{status, id}
	}

	_, err = dm.db.ExecContext(ctx, query, args...)
	if err != nil {
		return errors.Wrap(err, "failed to update execution status")
	}
	return nil
}

// CreateNodeState inserts a initial state for a node in an execution.
func (dm *DatabaseManager) CreateNodeState(ctx context.Context, ns *NodeState) error {
	if ns.ID == "" {
		ns.ID = uuid.New().String()
	}
	if ns.Status == "" {
		ns.Status = NodeStatusPending
	}
	if ns.CreatedAt.IsZero() {
		ns.CreatedAt = time.Now()
	}

	query := `INSERT INTO node_states (id, execution_id, node_id, status, created_at) VALUES (?, ?, ?, ?, ?)`
	_, err := dm.db.ExecContext(ctx, query, ns.ID, ns.ExecutionID, ns.NodeID, ns.Status, ns.CreatedAt)
	if err != nil {
		return errors.Wrap(err, "failed to insert node state")
	}
	return nil
}

// UpdateNodeStatus updates the status and result of a node in an execution.
func (dm *DatabaseManager) UpdateNodeStatus(ctx context.Context, id string, status NodeStatus, result []byte, errMsg string) error {
	var query string
	var args []interface{}
	now := time.Now()

	switch status {
	case NodeStatusRunning:
		query = `UPDATE node_states SET status = ?, started_at = ? WHERE id = ?`
		args = []interface{}{status, now, id}
	case NodeStatusSuccess, NodeStatusFailed, NodeStatusSkipped:
		query = `UPDATE node_states SET status = ?, completed_at = ?, result = ?, error = ? WHERE id = ?`
		args = []interface{}{status, now, result, errMsg, id}
	default:
		query = `UPDATE node_states SET status = ? WHERE id = ?`
		args = []interface{}{status, id}
	}

	_, err := dm.db.ExecContext(ctx, query, args...)
	if err != nil {
		return errors.Wrap(err, "failed to update node status")
	}
	return nil
}

// GetNodeStatesByExecution retrieves all node states for a given execution.
func (dm *DatabaseManager) GetNodeStatesByExecution(ctx context.Context, executionID string) ([]*NodeState, error) {
	query := `SELECT id, execution_id, node_id, status, started_at, completed_at, result, error, created_at FROM node_states WHERE execution_id = ?`
	rows, err := dm.db.QueryContext(ctx, query, executionID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get node states")
	}
	defer rows.Close()

	var states []*NodeState
	for rows.Next() {
		ns := &NodeState{}
		if err := rows.Scan(&ns.ID, &ns.ExecutionID, &ns.NodeID, &ns.Status, &ns.StartedAt, &ns.CompletedAt, &ns.Result, &ns.Error, &ns.CreatedAt); err != nil {
			return nil, errors.Wrap(err, "failed to scan node state")
		}
		states = append(states, ns)
	}
	return states, nil
}

// --- Phase 5: Backward Compatibility Guard ---

// HasActiveExecutions checks if there are any PENDING or RUNNING executions for a given workflow.
func (dm *DatabaseManager) HasActiveExecutions(ctx context.Context, workflowID string) (bool, error) {
	query := `SELECT COUNT(*) FROM executions WHERE workflow_id = ? AND status IN ('PENDING', 'RUNNING')`
	var count int
	err := dm.db.QueryRowContext(ctx, query, workflowID).Scan(&count)
	if err != nil {
		return false, errors.Wrap(err, "failed to check active executions")
	}
	return count > 0, nil
}

func isValidExecutionTransition(from, to ExecutionStatus) bool {
	switch from {
	case ExecutionStatusPending:
		return to == ExecutionStatusRunning || to == ExecutionStatusCancelled || to == ExecutionStatusFailed
	case ExecutionStatusRunning:
		return to == ExecutionStatusSuccess || to == ExecutionStatusFailed || to == ExecutionStatusCancelled
	default:
		return false
	}
}

// --- Dolt Versioning Helpers ---

func (dm *DatabaseManager) autoCommit(ctx context.Context, message string) error {
	if err := dm.doltAdd(ctx); err != nil {
		return err
	}
	return dm.doltCommit(ctx, message)
}

func (dm *DatabaseManager) doltAdd(ctx context.Context) error {
	_, err := dm.db.ExecContext(ctx, "CALL DOLT_ADD('-A')")
	if err != nil {
		return errors.Wrap(err, "failed to CALL DOLT_ADD")
	}
	return nil
}

func (dm *DatabaseManager) doltCommit(ctx context.Context, message string) error {
	_, err := dm.db.ExecContext(ctx, "CALL DOLT_COMMIT('-m', ?)", message)
	if err != nil {
		return errors.Wrap(err, "failed to CALL DOLT_COMMIT")
	}
	return nil
}

// DoltLog retrieves commit information from the dolt_log system table.
func (dm *DatabaseManager) DoltLog(ctx context.Context, limit int) ([]DoltCommitInfo, error) {
	query := fmt.Sprintf("SELECT commit_hash, committer, message, date FROM dolt_log LIMIT %d", limit)
	rows, err := dm.db.QueryContext(ctx, query)
	if err != nil {
		return nil, errors.Wrap(err, "failed to query dolt_log")
	}
	defer rows.Close()

	var commits []DoltCommitInfo
	for rows.Next() {
		var c DoltCommitInfo
		if err := rows.Scan(&c.Hash, &c.Author, &c.Message, &c.Timestamp); err != nil {
			return nil, errors.Wrap(err, "failed to scan commit info")
		}
		commits = append(commits, c)
	}
	return commits, nil
}
