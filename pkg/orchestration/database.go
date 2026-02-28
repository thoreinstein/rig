package orchestration

import (
	"context"
	"database/sql"
	"fmt"
	"log"
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
	if err := dm.db.Ping(); err != nil {
		if dm.Verbose {
			log.Printf("Dolt database not available: %v", err)
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
			log.Printf("Executing DDL:\n%s", ddl)
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
	tx, err := dm.db.BeginTx(ctx, nil)
	if err != nil {
		return errors.Wrap(err, "failed to begin transaction")
	}
	defer tx.Rollback() //nolint:errcheck

	if w.ID == "" {
		w.ID = uuid.New().String()
	}
	if w.Status == "" {
		w.Status = WorkflowStatusDraft
	}
	if w.Version == 0 {
		w.Version = 1
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

	if _, err := tx.ExecContext(ctx, query, w.ID, w.Name, w.Description, w.Version, w.Status, w.CreatedAt, w.UpdatedAt); err != nil {
		return errors.Wrap(err, "failed to insert workflow")
	}

	// Run Dolt versioning on the transaction so it's atomic with data changes.
	if err := txAutoCommit(ctx, tx, "Create workflow: "+w.Name); err != nil {
		return errors.Wrap(err, "failed to dolt-commit create workflow")
	}

	if err := tx.Commit(); err != nil {
		return errors.Wrap(err, "failed to commit transaction")
	}

	return nil
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
	tx, err := dm.db.BeginTx(ctx, nil)
	if err != nil {
		return errors.Wrap(err, "failed to begin transaction")
	}
	defer tx.Rollback() //nolint:errcheck

	// Lock the workflow row and fetch current state to ensure monotonic version and race-free guard
	current := &Workflow{}
	row := tx.QueryRowContext(ctx, "SELECT version, status FROM workflows WHERE id = ? FOR UPDATE", w.ID)
	if err := row.Scan(&current.Version, &current.Status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("workflow not found")
		}
		return errors.Wrap(err, "failed to fetch current workflow in tx")
	}

	// Guard against active executions inside the locked transaction
	active, err := dm.txHasActiveExecutions(ctx, tx, w.ID)
	if err != nil {
		return err
	}
	if active {
		return errors.New("cannot update workflow with active executions")
	}

	newVersion := current.Version + 1
	now := time.Now()

	// Merge status
	status := w.Status
	if status == "" {
		status = current.Status
	}

	query := `UPDATE workflows SET name = ?, description = ?, version = ?, status = ?, updated_at = ? WHERE id = ?`
	if _, err := tx.ExecContext(ctx, query, w.Name, w.Description, newVersion, status, now, w.ID); err != nil {
		return errors.Wrap(err, "failed to update workflow in tx")
	}

	// Run Dolt versioning on the transaction so it's atomic with data changes.
	if err := txAutoCommit(ctx, tx, fmt.Sprintf("Update workflow: %s (v%d)", w.Name, newVersion)); err != nil {
		return errors.Wrap(err, "failed to dolt-commit update workflow")
	}

	if err := tx.Commit(); err != nil {
		return errors.Wrap(err, "failed to commit transaction")
	}

	// Only mutate the caller's struct after all DB operations succeed
	w.Version = newVersion
	w.Status = status
	w.UpdatedAt = now
	return nil
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
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "error iterating workflow rows")
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

	query := `INSERT INTO nodes (id, workflow_id, workflow_version, name, type, config, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`
	_, err := dm.db.ExecContext(ctx, query, n.ID, n.WorkflowID, n.WorkflowVersion, n.Name, n.Type, n.Config, n.CreatedAt)
	if err != nil {
		return errors.Wrap(err, "failed to insert node")
	}
	return nil
}

// GetNodesByWorkflow retrieves all nodes for a given workflow version.
func (dm *DatabaseManager) GetNodesByWorkflow(ctx context.Context, workflowID string, version int) ([]*Node, error) {
	query := `SELECT id, workflow_id, workflow_version, name, type, config, created_at FROM nodes WHERE workflow_id = ? AND workflow_version = ? ORDER BY created_at ASC`
	rows, err := dm.db.QueryContext(ctx, query, workflowID, version)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get nodes")
	}
	defer rows.Close()

	var nodes []*Node
	for rows.Next() {
		n := &Node{}
		if err := rows.Scan(&n.ID, &n.WorkflowID, &n.WorkflowVersion, &n.Name, &n.Type, &n.Config, &n.CreatedAt); err != nil {
			return nil, errors.Wrap(err, "failed to scan node")
		}
		nodes = append(nodes, n)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "error iterating node rows")
	}
	return nodes, nil
}

// CreateEdge inserts a new edge record.
func (dm *DatabaseManager) CreateEdge(ctx context.Context, e *Edge) error {
	if e.ID == "" {
		e.ID = uuid.New().String()
	}

	query := `INSERT INTO edges (id, workflow_id, workflow_version, source_node_id, target_node_id, condition) VALUES (?, ?, ?, ?, ?, ?)`
	_, err := dm.db.ExecContext(ctx, query, e.ID, e.WorkflowID, e.WorkflowVersion, e.SourceNodeID, e.TargetNodeID, e.Condition)
	if err != nil {
		return errors.Wrap(err, "failed to insert edge")
	}
	return nil
}

// GetEdgesByWorkflow retrieves all edges for a given workflow version.
func (dm *DatabaseManager) GetEdgesByWorkflow(ctx context.Context, workflowID string, version int) ([]*Edge, error) {
	query := `SELECT id, workflow_id, workflow_version, source_node_id, target_node_id, condition FROM edges WHERE workflow_id = ? AND workflow_version = ?`
	rows, err := dm.db.QueryContext(ctx, query, workflowID, version)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get edges")
	}
	defer rows.Close()

	var edges []*Edge
	for rows.Next() {
		e := &Edge{}
		if err := rows.Scan(&e.ID, &e.WorkflowID, &e.WorkflowVersion, &e.SourceNodeID, &e.TargetNodeID, &e.Condition); err != nil {
			return nil, errors.Wrap(err, "failed to scan edge")
		}
		edges = append(edges, e)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "error iterating edge rows")
	}
	return edges, nil
}

// SaveWorkflowDefinition transactionally saves a full workflow definition and creates a Dolt commit.
func (dm *DatabaseManager) SaveWorkflowDefinition(ctx context.Context, w *Workflow, nodes []*Node, edges []*Edge) error {
	// 0. Assign stable IDs before validation so empty-ID nodes don't collapse.
	for _, n := range nodes {
		if n.ID == "" {
			n.ID = uuid.New().String()
		}
	}
	for _, e := range edges {
		if e.ID == "" {
			e.ID = uuid.New().String()
		}
	}

	// 1. Validate DAG
	if err := ValidateWorkflow(nodes, edges); err != nil {
		return errors.Wrap(err, "invalid workflow definition")
	}

	tx, err := dm.db.BeginTx(ctx, nil)
	if err != nil {
		return errors.Wrap(err, "failed to begin transaction")
	}
	defer tx.Rollback() //nolint:errcheck

	// 2. Update/Create Workflow
	// Track deferred mutations to apply only after all DB ops succeed.
	var deferredID string
	var deferredVersion int
	var deferredStatus WorkflowStatus
	var deferredCreatedAt, deferredUpdatedAt time.Time
	isNew := w.ID == ""

	if isNew {
		deferredID = uuid.New().String()
		deferredVersion = 1
		deferredCreatedAt = time.Now()
		deferredUpdatedAt = deferredCreatedAt
		deferredStatus = w.Status
		if deferredStatus == "" {
			deferredStatus = WorkflowStatusDraft
		}
		query := `INSERT INTO workflows (id, name, description, version, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`
		if _, err := tx.ExecContext(ctx, query, deferredID, w.Name, w.Description, deferredVersion, deferredStatus, deferredCreatedAt, deferredUpdatedAt); err != nil {
			return errors.Wrap(err, "failed to insert workflow in tx")
		}
	} else {
		deferredID = w.ID
		// Lock the workflow row and fetch current state inside tx to ensure atomic merge, monotonic version, and race-free guard
		current := &Workflow{}
		row := tx.QueryRowContext(ctx, "SELECT version, status FROM workflows WHERE id = ? FOR UPDATE", w.ID)
		if err := row.Scan(&current.Version, &current.Status); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return errors.New("workflow not found")
			}
			return errors.Wrap(err, "failed to fetch current workflow in tx")
		}

		// Guard against active executions inside the locked transaction
		active, err := dm.txHasActiveExecutions(ctx, tx, w.ID)
		if err != nil {
			return err
		}
		if active {
			return errors.New("cannot update workflow definition with active executions")
		}

		deferredVersion = current.Version + 1
		deferredUpdatedAt = time.Now()
		deferredCreatedAt = w.CreatedAt // Preserve original creation time

		deferredStatus = w.Status
		if deferredStatus == "" {
			deferredStatus = current.Status
		}

		query := `UPDATE workflows SET name = ?, description = ?, version = ?, status = ?, updated_at = ? WHERE id = ?`
		if _, err := tx.ExecContext(ctx, query, w.Name, w.Description, deferredVersion, deferredStatus, deferredUpdatedAt, w.ID); err != nil {
			return errors.Wrap(err, "failed to update workflow in tx")
		}
	}

	// 3. Clean existing edges for THIS version if updating.
	// We don't clean old versions because they are referenced by historical executions.
	// But since we increment version on every SaveWorkflowDefinition, there shouldn't be
	// any existing edges for 'deferredVersion' anyway. We clean just in case of retries/idempotency.
	if _, err := tx.ExecContext(ctx, "DELETE FROM edges WHERE workflow_id = ? AND workflow_version = ?", deferredID, deferredVersion); err != nil {
		return errors.Wrap(err, "failed to clean edges")
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM nodes WHERE workflow_id = ? AND workflow_version = ?", deferredID, deferredVersion); err != nil {
		return errors.Wrap(err, "failed to clean nodes")
	}

	// 4. Insert new nodes
	for _, n := range nodes {
		n.WorkflowID = deferredID
		n.WorkflowVersion = deferredVersion
		if n.CreatedAt.IsZero() {
			n.CreatedAt = time.Now()
		}
		query := `INSERT INTO nodes (id, workflow_id, workflow_version, name, type, config, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`
		if _, err := tx.ExecContext(ctx, query, n.ID, n.WorkflowID, n.WorkflowVersion, n.Name, n.Type, n.Config, n.CreatedAt); err != nil {
			return errors.Wrap(err, "failed to insert node in tx")
		}
	}

	// 5. Insert new edges
	for _, e := range edges {
		e.WorkflowID = deferredID
		e.WorkflowVersion = deferredVersion
		query := `INSERT INTO edges (id, workflow_id, workflow_version, source_node_id, target_node_id, condition) VALUES (?, ?, ?, ?, ?, ?)`
		if _, err := tx.ExecContext(ctx, query, e.ID, e.WorkflowID, e.WorkflowVersion, e.SourceNodeID, e.TargetNodeID, e.Condition); err != nil {
			return errors.Wrap(err, "failed to insert edge in tx")
		}
	}

	// Run Dolt versioning on the transaction so it's atomic with data changes.
	if err := txAutoCommit(ctx, tx, fmt.Sprintf("Save workflow definition: %s (v%d)", w.Name, deferredVersion)); err != nil {
		return errors.Wrap(err, "failed to dolt-commit save workflow definition")
	}

	if err := tx.Commit(); err != nil {
		return errors.Wrap(err, "failed to commit transaction")
	}

	// Only mutate the caller's struct after all DB operations succeed
	w.ID = deferredID
	w.Version = deferredVersion
	w.Status = deferredStatus
	w.UpdatedAt = deferredUpdatedAt
	if isNew {
		w.CreatedAt = deferredCreatedAt
	}
	return nil
}

// --- Phase 4: Execution State Management ---
// Note: Execution state changes (CreateExecution, CreateNodeState, UpdateExecutionStatus,
// UpdateNodeStatus) intentionally do not create Dolt commits. Only workflow *definitions*
// are versioned via Dolt. Execution state is transient runtime data.

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

// CreateExecutionWithInitialStates creates an execution and initializes all node states.
func (dm *DatabaseManager) CreateExecutionWithInitialStates(ctx context.Context, workflowID string, version int) (*Execution, error) {
	nodes, err := dm.GetNodesByWorkflow(ctx, workflowID, version)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get nodes")
	}

	tx, err := dm.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to begin transaction")
	}
	defer tx.Rollback() //nolint:errcheck

	e := &Execution{
		ID:              uuid.New().String(),
		WorkflowID:      workflowID,
		WorkflowVersion: version,
		Status:          ExecutionStatusPending,
		CreatedAt:       time.Now(),
	}

	execQuery := `INSERT INTO executions (id, workflow_id, workflow_version, status, created_at) VALUES (?, ?, ?, ?, ?)`
	if _, err := tx.ExecContext(ctx, execQuery, e.ID, e.WorkflowID, e.WorkflowVersion, e.Status, e.CreatedAt); err != nil {
		return nil, errors.Wrap(err, "failed to insert execution")
	}

	nodeStateQuery := `INSERT INTO node_states (id, execution_id, node_id, status, created_at) VALUES (?, ?, ?, ?, ?)`
	for _, node := range nodes {
		nsID := uuid.New().String()
		if _, err := tx.ExecContext(ctx, nodeStateQuery, nsID, e.ID, node.ID, NodeStatusPending, e.CreatedAt); err != nil {
			return nil, errors.Wrap(err, "failed to insert node state")
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, errors.Wrap(err, "failed to commit transaction")
	}

	return e, nil
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
// For FAILED status, errMsg is recorded in the error column.
func (dm *DatabaseManager) UpdateExecutionStatus(ctx context.Context, id string, status ExecutionStatus, errMsg string) error {
	tx, err := dm.db.BeginTx(ctx, nil)
	if err != nil {
		return errors.Wrap(err, "failed to begin transaction")
	}
	defer tx.Rollback() //nolint:errcheck

	// Lock the row and fetch current status atomically
	var currentStatus ExecutionStatus
	row := tx.QueryRowContext(ctx, "SELECT status FROM executions WHERE id = ? FOR UPDATE", id)
	if err := row.Scan(&currentStatus); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("execution not found")
		}
		return errors.Wrap(err, "failed to fetch current execution status")
	}

	if !isValidExecutionTransition(currentStatus, status) {
		return errors.Errorf("invalid execution status transition from %s to %s", currentStatus, status)
	}

	var query string
	var args []interface{}
	now := time.Now()

	switch status {
	case ExecutionStatusRunning:
		query = `UPDATE executions SET status = ?, started_at = COALESCE(started_at, ?) WHERE id = ?`
		args = []interface{}{status, now, id}
	case ExecutionStatusFailed:
		query = `UPDATE executions SET status = ?, completed_at = ?, error = ? WHERE id = ?`
		args = []interface{}{status, now, errMsg, id}
	case ExecutionStatusSuccess, ExecutionStatusCancelled:
		query = `UPDATE executions SET status = ?, completed_at = ? WHERE id = ?`
		args = []interface{}{status, now, id}
	default:
		query = `UPDATE executions SET status = ? WHERE id = ?`
		args = []interface{}{status, id}
	}

	if _, err := tx.ExecContext(ctx, query, args...); err != nil {
		return errors.Wrap(err, "failed to update execution status")
	}

	if err := tx.Commit(); err != nil {
		return errors.Wrap(err, "failed to commit execution status update")
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
	tx, err := dm.db.BeginTx(ctx, nil)
	if err != nil {
		return errors.Wrap(err, "failed to begin transaction")
	}
	defer tx.Rollback() //nolint:errcheck

	// Lock the row and fetch current status atomically
	var currentStatus NodeStatus
	row := tx.QueryRowContext(ctx, "SELECT status FROM node_states WHERE id = ? FOR UPDATE", id)
	if err := row.Scan(&currentStatus); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("node state not found")
		}
		return errors.Wrap(err, "failed to fetch current node status")
	}

	if !isValidNodeTransition(currentStatus, status) {
		return errors.Errorf("invalid node status transition from %s to %s", currentStatus, status)
	}

	var query string
	var args []interface{}
	now := time.Now()

	switch status {
	case NodeStatusRunning:
		query = `UPDATE node_states SET status = ?, started_at = COALESCE(started_at, ?) WHERE id = ?`
		args = []interface{}{status, now, id}
	case NodeStatusSuccess, NodeStatusFailed, NodeStatusSkipped:
		query = `UPDATE node_states SET status = ?, completed_at = ?, result = ?, error = ? WHERE id = ?`
		args = []interface{}{status, now, result, errMsg, id}
	default:
		query = `UPDATE node_states SET status = ? WHERE id = ?`
		args = []interface{}{status, id}
	}

	if _, err := tx.ExecContext(ctx, query, args...); err != nil {
		return errors.Wrap(err, "failed to update node status")
	}

	if err := tx.Commit(); err != nil {
		return errors.Wrap(err, "failed to commit node status update")
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
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "error iterating node state rows")
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

// txHasActiveExecutions is like HasActiveExecutions but operates within a transaction.
func (dm *DatabaseManager) txHasActiveExecutions(ctx context.Context, tx *sql.Tx, workflowID string) (bool, error) {
	query := `SELECT COUNT(*) FROM executions WHERE workflow_id = ? AND status IN ('PENDING', 'RUNNING')`
	var count int
	err := tx.QueryRowContext(ctx, query, workflowID).Scan(&count)
	if err != nil {
		return false, errors.Wrap(err, "failed to check active executions in tx")
	}
	return count > 0, nil
}

func isValidExecutionTransition(from, to ExecutionStatus) bool {
	switch from {
	case ExecutionStatusPending:
		return to == ExecutionStatusRunning || to == ExecutionStatusCancelled || to == ExecutionStatusFailed
	case ExecutionStatusRunning:
		// RUNNING -> RUNNING is permitted for crash recovery: the orchestrator
		// re-drives interrupted executions, and the COALESCE on started_at
		// prevents the original timestamp from being overwritten.
		return to == ExecutionStatusRunning || to == ExecutionStatusSuccess || to == ExecutionStatusFailed || to == ExecutionStatusCancelled
	default:
		return false
	}
}

func isValidNodeTransition(from, to NodeStatus) bool {
	switch from {
	case NodeStatusPending:
		return to == NodeStatusRunning || to == NodeStatusSkipped || to == NodeStatusFailed
	case NodeStatusRunning:
		// RUNNING -> RUNNING is permitted for crash recovery: nodes interrupted
		// mid-execution are re-dispatched, and the COALESCE on started_at
		// prevents the original timestamp from being overwritten.
		return to == NodeStatusRunning || to == NodeStatusSuccess || to == NodeStatusFailed || to == NodeStatusSkipped
	default:
		return false
	}
}

// --- Dolt Versioning Helpers ---

// txAutoCommit runs DOLT_ADD and DOLT_COMMIT on a transaction handle so the
// Dolt commit is atomic with the surrounding SQL changes.
func txAutoCommit(ctx context.Context, tx *sql.Tx, message string) error {
	if _, err := tx.ExecContext(ctx, "CALL DOLT_ADD('-A')"); err != nil {
		return errors.Wrap(err, "failed to CALL DOLT_ADD in tx")
	}
	if _, err := tx.ExecContext(ctx, "CALL DOLT_COMMIT('-m', ?)", message); err != nil {
		return errors.Wrap(err, "failed to CALL DOLT_COMMIT in tx")
	}
	return nil
}

// DoltLog retrieves commit information from the dolt_log system table.
func (dm *DatabaseManager) DoltLog(ctx context.Context, limit int) ([]DoltCommitInfo, error) {
	if limit <= 0 {
		limit = 10
	}
	query := `SELECT commit_hash, committer, message, date FROM dolt_log LIMIT ?`
	rows, err := dm.db.QueryContext(ctx, query, limit)
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
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "error iterating dolt log rows")
	}
	return commits, nil
}
