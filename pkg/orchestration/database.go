package orchestration

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/dolthub/driver"
	"github.com/google/uuid"

	rigerrors "thoreinstein.com/rig/pkg/errors"
)

// DatabaseManager handles Dolt database operations for the orchestration engine.
type DatabaseManager struct {
	db       *sql.DB
	dataPath string
	Verbose  bool
}

// NewDatabaseManager creates a new DatabaseManager using the embedded Dolt driver.
func NewDatabaseManager(dataPath, commitName, commitEmail string, verbose bool) (*DatabaseManager, error) {
	// Ensure absolute path for a well-formed file URL
	absPath, err := filepath.Abs(dataPath)
	if err != nil {
		return nil, rigerrors.Wrap(err, "failed to resolve absolute data path")
	}

	u := &url.URL{
		Scheme: "file",
		Path:   absPath,
		RawQuery: url.Values{
			"commitname":  {commitName},
			"commitemail": {commitEmail},
			"database":    {"rig_orchestration"},
		}.Encode(),
	}
	dsn := u.String()

	db, err := sql.Open("dolt", dsn)
	if err != nil {
		return nil, rigerrors.Wrap(err, "failed to open embedded dolt database")
	}

	return &DatabaseManager{
		db:       db,
		dataPath: absPath,
		Verbose:  verbose,
	}, nil
}

// Close closes the database connection.
func (dm *DatabaseManager) Close() error {
	if dm.db != nil {
		return dm.db.Close()
	}
	return nil
}

// InitDatabase initializes the database and creates tables.
// All DDL runs on a single connection to ensure USE session state persists
// across statements in the connection pool.
func (dm *DatabaseManager) InitDatabase() error {
	// Ensure directory exists
	if err := os.MkdirAll(dm.dataPath, 0700); err != nil {
		return rigerrors.Wrap(err, "failed to create data path")
	}

	ctx := context.Background()
	conn, err := dm.db.Conn(ctx)
	if err != nil {
		return rigerrors.Wrap(err, "failed to acquire database connection")
	}
	defer conn.Close()

	// Create database if not exists
	if _, err := conn.ExecContext(ctx, "CREATE DATABASE IF NOT EXISTS rig_orchestration"); err != nil {
		return rigerrors.Wrap(err, "failed to create rig_orchestration database")
	}

	// Use database (session state persists on this single connection)
	if _, err := conn.ExecContext(ctx, "USE rig_orchestration"); err != nil {
		return rigerrors.Wrap(err, "failed to use rig_orchestration database")
	}

	// Initialize migration framework
	if _, err := conn.ExecContext(ctx, SchemaMigrationsTableDDL); err != nil {
		return rigerrors.Wrap(err, "failed to create schema_migrations table")
	}

	return dm.migrateWithConn(ctx, conn)
}

// Migrate applies any pending schema migrations.
func (dm *DatabaseManager) Migrate(ctx context.Context) error {
	conn, err := dm.db.Conn(ctx)
	if err != nil {
		return rigerrors.Wrap(err, "failed to acquire database connection for migration")
	}
	defer conn.Close()

	// Ensure we are using the correct database if it wasn't already set in DSN
	if _, err := conn.ExecContext(ctx, "USE rig_orchestration"); err != nil {
		return rigerrors.Wrap(err, "failed to use rig_orchestration database")
	}

	return dm.migrateWithConn(ctx, conn)
}

func (dm *DatabaseManager) migrateWithConn(ctx context.Context, conn *sql.Conn) error {
	// 1. Get current version
	var currentVersion int
	query := "SELECT COALESCE(MAX(version), 0) FROM schema_migrations"
	// Use conn.QueryRowContext instead of dm.db.QueryRowContext to stay on the same session
	err := conn.QueryRowContext(ctx, query).Scan(&currentVersion)
	if err != nil {
		// Dolt returns sql.ErrNoRows for aggregates on an empty table
		if !rigerrors.Is(err, sql.ErrNoRows) {
			return rigerrors.Wrap(err, "failed to read current migration version from schema_migrations")
		}
		currentVersion = 0
	}

	// 2. Apply pending migrations
	for _, m := range AllMigrations() {
		if m.Version > currentVersion {
			if dm.Verbose {
				log.Printf("Applying migration v%d: %s", m.Version, m.Description)
			}
			if err := dm.applyMigration(ctx, conn, m); err != nil {
				return err
			}
		}
	}

	return nil
}

// applyMigration runs a single migration inside a manual transaction on conn.
// Extracting this from the loop ensures the deferred rollback is scoped to one
// migration instead of stacking N defers.
func (dm *DatabaseManager) applyMigration(ctx context.Context, conn *sql.Conn, m Migration) error {
	if _, err := conn.ExecContext(ctx, "START TRANSACTION"); err != nil {
		return rigerrors.Wrapf(err, "failed to start transaction for migration v%d", m.Version)
	}

	committed := false
	defer func() {
		if !committed {
			// Use Background so rollback succeeds even if parent ctx is cancelled.
			conn.ExecContext(context.Background(), "ROLLBACK") //nolint:errcheck
		}
	}()

	for _, ddl := range m.DDLs {
		if _, err := conn.ExecContext(ctx, ddl); err != nil {
			return rigerrors.Wrapf(err, "failed to execute DDL in migration v%d", m.Version)
		}
	}

	insertQuery := "INSERT INTO schema_migrations (version, description) VALUES (?, ?)"
	if _, err := conn.ExecContext(ctx, insertQuery, m.Version, m.Description); err != nil {
		return rigerrors.Wrapf(err, "failed to record migration v%d", m.Version)
	}

	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return rigerrors.Wrapf(err, "failed to commit migration v%d", m.Version)
	}
	committed = true
	return nil
}

// --- Phase 3: Workflow Definition CRUD + Dolt Versioning ---

// CreateWorkflow inserts a new workflow record.
func (dm *DatabaseManager) CreateWorkflow(ctx context.Context, w *Workflow) error {
	return dm.withRetry(ctx, "CreateWorkflow", func() error {
		tx, err := dm.db.BeginTx(ctx, nil)
		if err != nil {
			return rigerrors.Wrap(err, "failed to begin transaction")
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
			return rigerrors.Wrap(err, "failed to insert workflow")
		}

		// Run Dolt versioning on the transaction so it's atomic with data changes.
		if err := txAutoCommit(ctx, tx, "Create workflow: "+w.Name); err != nil {
			return rigerrors.Wrap(err, "failed to dolt-commit create workflow")
		}

		if err := tx.Commit(); err != nil {
			return rigerrors.Wrap(err, "failed to commit transaction")
		}

		return nil
	})
}

// GetWorkflow retrieves a workflow by ID.
func (dm *DatabaseManager) GetWorkflow(ctx context.Context, id string) (*Workflow, error) {
	query := `SELECT id, name, description, version, status, created_at, updated_at FROM workflows WHERE id = ?`
	w := &Workflow{}
	err := dm.db.QueryRowContext(ctx, query, id).Scan(
		&w.ID, &w.Name, &w.Description, &w.Version, &w.Status, &w.CreatedAt, &w.UpdatedAt,
	)
	if err != nil {
		if rigerrors.Is(err, sql.ErrNoRows) {
			return nil, rigerrors.New("workflow not found")
		}
		return nil, rigerrors.Wrap(err, "failed to get workflow")
	}
	return w, nil
}

// UpdateWorkflow updates an existing workflow and increments its version.
func (dm *DatabaseManager) UpdateWorkflow(ctx context.Context, w *Workflow) error {
	return dm.withRetry(ctx, "UpdateWorkflow", func() error {
		tx, err := dm.db.BeginTx(ctx, nil)
		if err != nil {
			return rigerrors.Wrap(err, "failed to begin transaction")
		}
		defer tx.Rollback() //nolint:errcheck

		// Lock the workflow row and fetch current state to ensure monotonic version and race-free guard
		current := &Workflow{}
		row := tx.QueryRowContext(ctx, "SELECT version, status FROM workflows WHERE id = ? FOR UPDATE", w.ID)
		if err := row.Scan(&current.Version, &current.Status); err != nil {
			if rigerrors.Is(err, sql.ErrNoRows) {
				return rigerrors.New("workflow not found")
			}
			return rigerrors.Wrap(err, "failed to fetch current workflow in tx")
		}

		// Guard against active executions inside the locked transaction
		active, err := dm.txHasActiveExecutions(ctx, tx, w.ID)
		if err != nil {
			return err
		}
		if active {
			return rigerrors.New("cannot update workflow with active executions")
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
			return rigerrors.Wrap(err, "failed to update workflow in tx")
		}

		// Run Dolt versioning on the transaction so it's atomic with data changes.
		if err := txAutoCommit(ctx, tx, fmt.Sprintf("Update workflow: %s (v%d)", w.Name, newVersion)); err != nil {
			return rigerrors.Wrap(err, "failed to dolt-commit update workflow")
		}

		if err := tx.Commit(); err != nil {
			return rigerrors.Wrap(err, "failed to commit transaction")
		}

		// Only mutate the caller's struct after all DB operations succeed
		w.Version = newVersion
		w.Status = status
		w.UpdatedAt = now
		return nil
	})
}

// ListWorkflows retrieves all workflows.
func (dm *DatabaseManager) ListWorkflows(ctx context.Context) ([]*Workflow, error) {
	query := `SELECT id, name, description, version, status, created_at, updated_at FROM workflows ORDER BY created_at DESC`
	rows, err := dm.db.QueryContext(ctx, query)
	if err != nil {
		return nil, rigerrors.Wrap(err, "failed to list workflows")
	}
	defer rows.Close()

	var workflows []*Workflow
	for rows.Next() {
		w := &Workflow{}
		if err := rows.Scan(&w.ID, &w.Name, &w.Description, &w.Version, &w.Status, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, rigerrors.Wrap(err, "failed to scan workflow")
		}
		workflows = append(workflows, w)
	}
	if err := rows.Err(); err != nil {
		return nil, rigerrors.Wrap(err, "error iterating workflow rows")
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
	// Ensure valid JSON for Dolt
	if len(n.Config) == 0 {
		n.Config = []byte("{}")
	}

	query := `INSERT INTO nodes (id, workflow_id, workflow_version, name, type, config, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`
	_, err := dm.db.ExecContext(ctx, query, n.ID, n.WorkflowID, n.WorkflowVersion, n.Name, n.Type, n.Config, n.CreatedAt)
	if err != nil {
		return rigerrors.Wrap(err, "failed to insert node")
	}
	return nil
}

// GetNodesByWorkflow retrieves all nodes for a given workflow version.
func (dm *DatabaseManager) GetNodesByWorkflow(ctx context.Context, workflowID string, version int) ([]*Node, error) {
	query := `SELECT id, workflow_id, workflow_version, name, type, config, created_at FROM nodes WHERE workflow_id = ? AND workflow_version = ? ORDER BY created_at ASC`
	rows, err := dm.db.QueryContext(ctx, query, workflowID, version)
	if err != nil {
		return nil, rigerrors.Wrap(err, "failed to get nodes")
	}
	defer rows.Close()

	var nodes []*Node
	for rows.Next() {
		n := &Node{}
		var config []byte
		if err := rows.Scan(&n.ID, &n.WorkflowID, &n.WorkflowVersion, &n.Name, &n.Type, &config, &n.CreatedAt); err != nil {
			return nil, rigerrors.Wrap(err, "failed to scan node")
		}
		n.Config = config
		nodes = append(nodes, n)
	}
	if err := rows.Err(); err != nil {
		return nil, rigerrors.Wrap(err, "error iterating node rows")
	}
	return nodes, nil
}

// CreateEdge inserts a new edge record.
func (dm *DatabaseManager) CreateEdge(ctx context.Context, e *Edge) error {
	if e.ID == "" {
		e.ID = uuid.New().String()
	}

	query := "INSERT INTO edges (id, workflow_id, workflow_version, source_node_id, target_node_id, `condition`) VALUES (?, ?, ?, ?, ?, ?)"
	_, err := dm.db.ExecContext(ctx, query, e.ID, e.WorkflowID, e.WorkflowVersion, e.SourceNodeID, e.TargetNodeID, e.Condition)
	if err != nil {
		return rigerrors.Wrap(err, "failed to insert edge")
	}
	return nil
}

// GetEdgesByWorkflow retrieves all edges for a given workflow version.
func (dm *DatabaseManager) GetEdgesByWorkflow(ctx context.Context, workflowID string, version int) ([]*Edge, error) {
	query := "SELECT id, workflow_id, workflow_version, source_node_id, target_node_id, `condition` FROM edges WHERE workflow_id = ? AND workflow_version = ?"
	rows, err := dm.db.QueryContext(ctx, query, workflowID, version)
	if err != nil {
		return nil, rigerrors.Wrap(err, "failed to get edges")
	}
	defer rows.Close()

	var edges []*Edge
	for rows.Next() {
		e := &Edge{}
		if err := rows.Scan(&e.ID, &e.WorkflowID, &e.WorkflowVersion, &e.SourceNodeID, &e.TargetNodeID, &e.Condition); err != nil {
			return nil, rigerrors.Wrap(err, "failed to scan edge")
		}
		edges = append(edges, e)
	}
	if err := rows.Err(); err != nil {
		return nil, rigerrors.Wrap(err, "error iterating edge rows")
	}
	return edges, nil
}

// SaveWorkflowDefinition transactionally saves a full workflow definition and creates a Dolt commit.
func (dm *DatabaseManager) SaveWorkflowDefinition(ctx context.Context, w *Workflow, nodes []*Node, edges []*Edge) error {
	return dm.withRetry(ctx, "SaveWorkflowDefinition", func() error {
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
			return rigerrors.Wrap(err, "invalid workflow definition")
		}

		tx, err := dm.db.BeginTx(ctx, nil)
		if err != nil {
			return rigerrors.Wrap(err, "failed to begin transaction")
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
				return rigerrors.Wrap(err, "failed to insert workflow in tx")
			}
		} else {
			deferredID = w.ID
			// Lock the workflow row and fetch current state inside tx to ensure atomic merge, monotonic version, and race-free guard
			current := &Workflow{}
			row := tx.QueryRowContext(ctx, "SELECT version, status FROM workflows WHERE id = ? FOR UPDATE", w.ID)
			if err := row.Scan(&current.Version, &current.Status); err != nil {
				if rigerrors.Is(err, sql.ErrNoRows) {
					return rigerrors.New("workflow not found")
				}
				return rigerrors.Wrap(err, "failed to fetch current workflow in tx")
			}

			// Guard against active executions inside the locked transaction
			active, err := dm.txHasActiveExecutions(ctx, tx, w.ID)
			if err != nil {
				return err
			}
			if active {
				return rigerrors.New("cannot update workflow definition with active executions")
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
				return rigerrors.Wrap(err, "failed to update workflow in tx")
			}
		}

		// 3. Clean existing edges for THIS version if updating.
		// We don't clean old versions because they are referenced by historical executions.
		// But since we increment version on every SaveWorkflowDefinition, there shouldn't be
		// any existing edges for 'deferredVersion' anyway. We clean just in case of retries/idempotency.
		if _, err := tx.ExecContext(ctx, "DELETE FROM edges WHERE workflow_id = ? AND workflow_version = ?", deferredID, deferredVersion); err != nil {
			return rigerrors.Wrap(err, "failed to clean edges")
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM nodes WHERE workflow_id = ? AND workflow_version = ?", deferredID, deferredVersion); err != nil {
			return rigerrors.Wrap(err, "failed to clean nodes")
		}

		// 4. Insert new nodes
		for _, n := range nodes {
			n.WorkflowID = deferredID
			n.WorkflowVersion = deferredVersion
			if n.CreatedAt.IsZero() {
				n.CreatedAt = time.Now()
			}
			// Ensure valid JSON for Dolt
			if len(n.Config) == 0 {
				n.Config = []byte("{}")
			}
			query := `INSERT INTO nodes (id, workflow_id, workflow_version, name, type, config, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`
			if _, err := tx.ExecContext(ctx, query, n.ID, n.WorkflowID, n.WorkflowVersion, n.Name, n.Type, n.Config, n.CreatedAt); err != nil {
				return rigerrors.Wrap(err, "failed to insert node in tx")
			}
		}

		// 5. Insert new edges
		for _, e := range edges {
			e.WorkflowID = deferredID
			e.WorkflowVersion = deferredVersion
			query := `INSERT INTO edges (id, workflow_id, workflow_version, source_node_id, target_node_id, ` + "`condition`" + `) VALUES (?, ?, ?, ?, ?, ?)`
			if _, err := tx.ExecContext(ctx, query, e.ID, e.WorkflowID, e.WorkflowVersion, e.SourceNodeID, e.TargetNodeID, e.Condition); err != nil {
				return rigerrors.Wrap(err, "failed to insert edge in tx")
			}
		}

		// Run Dolt versioning on the transaction so it's atomic with data changes.
		if err := txAutoCommit(ctx, tx, fmt.Sprintf("Save workflow definition: %s (v%d)", w.Name, deferredVersion)); err != nil {
			return rigerrors.Wrap(err, "failed to dolt-commit save workflow definition")
		}

		if err := tx.Commit(); err != nil {
			return rigerrors.Wrap(err, "failed to commit transaction")
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
	})
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
		return rigerrors.Wrap(err, "failed to insert execution")
	}
	return nil
}

// CreateExecutionWithInitialStates creates an execution and initializes all node states.
func (dm *DatabaseManager) CreateExecutionWithInitialStates(ctx context.Context, workflowID string, version int) (*Execution, error) {
	nodes, err := dm.GetNodesByWorkflow(ctx, workflowID, version)
	if err != nil {
		return nil, rigerrors.Wrap(err, "failed to get nodes")
	}

	tx, err := dm.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, rigerrors.Wrap(err, "failed to begin transaction")
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
		return nil, rigerrors.Wrap(err, "failed to insert execution")
	}

	nodeStateQuery := `INSERT INTO node_states (id, execution_id, node_id, status, result, created_at) VALUES (?, ?, ?, ?, ?, ?)`
	for _, node := range nodes {
		nsID := uuid.New().String()
		if _, err := tx.ExecContext(ctx, nodeStateQuery, nsID, e.ID, node.ID, NodeStatusPending, []byte("{}"), e.CreatedAt); err != nil {
			return nil, rigerrors.Wrap(err, "failed to insert node state")
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, rigerrors.Wrap(err, "failed to commit transaction")
	}

	return e, nil
}

// GetExecution retrieves an execution by ID.
func (dm *DatabaseManager) GetExecution(ctx context.Context, id string) (*Execution, error) {
	query := `SELECT id, workflow_id, workflow_version, status, started_at, completed_at, COALESCE(error, ''), created_at FROM executions WHERE id = ?`
	e := &Execution{}
	err := dm.db.QueryRowContext(ctx, query, id).Scan(
		&e.ID, &e.WorkflowID, &e.WorkflowVersion, &e.Status, &e.StartedAt, &e.CompletedAt, &e.Error, &e.CreatedAt,
	)
	if err != nil {
		if rigerrors.Is(err, sql.ErrNoRows) {
			return nil, rigerrors.New("execution not found")
		}
		return nil, rigerrors.Wrap(err, "failed to get execution")
	}
	return e, nil
}

// UpdateExecutionStatus transitions an execution to a new status.
// It also sets started_at or completed_at based on the status.
// For FAILED status, errMsg is recorded in the error column.
func (dm *DatabaseManager) UpdateExecutionStatus(ctx context.Context, id string, status ExecutionStatus, errMsg string) error {
	return dm.withRetry(ctx, "UpdateExecutionStatus", func() error {
		tx, err := dm.db.BeginTx(ctx, nil)
		if err != nil {
			return rigerrors.Wrap(err, "failed to begin transaction")
		}
		defer tx.Rollback() //nolint:errcheck

		// Lock the row and fetch current status atomically
		var currentStatus ExecutionStatus
		row := tx.QueryRowContext(ctx, "SELECT status FROM executions WHERE id = ? FOR UPDATE", id)
		if err := row.Scan(&currentStatus); err != nil {
			if rigerrors.Is(err, sql.ErrNoRows) {
				return rigerrors.New("execution not found")
			}
			return rigerrors.Wrap(err, "failed to fetch current execution status")
		}

		if !isValidExecutionTransition(currentStatus, status) {
			return rigerrors.Newf("invalid execution status transition from %s to %s", currentStatus, status)
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
			return rigerrors.Wrap(err, "failed to update execution status")
		}

		if err := tx.Commit(); err != nil {
			return rigerrors.Wrap(err, "failed to commit execution status update")
		}
		return nil
	})
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
	// Ensure valid JSON for Dolt
	if len(ns.Result) == 0 {
		ns.Result = []byte("{}")
	}

	query := `INSERT INTO node_states (id, execution_id, node_id, status, result, created_at) VALUES (?, ?, ?, ?, ?, ?)`
	_, err := dm.db.ExecContext(ctx, query, ns.ID, ns.ExecutionID, ns.NodeID, ns.Status, ns.Result, ns.CreatedAt)
	if err != nil {
		return rigerrors.Wrap(err, "failed to insert node state")
	}
	return nil
}

// GetNodeState retrieves a specific node state by ID.
func (dm *DatabaseManager) GetNodeState(ctx context.Context, id string) (*NodeState, error) {
	query := `SELECT id, execution_id, node_id, status, started_at, completed_at, result, COALESCE(error, ''), created_at FROM node_states WHERE id = ?`
	ns := &NodeState{}
	var result []byte
	err := dm.db.QueryRowContext(ctx, query, id).Scan(
		&ns.ID, &ns.ExecutionID, &ns.NodeID, &ns.Status, &ns.StartedAt, &ns.CompletedAt, &result, &ns.Error, &ns.CreatedAt,
	)
	if err != nil {
		if rigerrors.Is(err, sql.ErrNoRows) {
			return nil, rigerrors.New("node state not found")
		}
		return nil, rigerrors.Wrap(err, "failed to get node state")
	}
	ns.Result = result
	return ns, nil
}

// UpdateNodeStatus updates the status and result of a node in an execution.
func (dm *DatabaseManager) UpdateNodeStatus(ctx context.Context, id string, status NodeStatus, result []byte, errMsg string) error {
	return dm.withRetry(ctx, "UpdateNodeStatus", func() error {
		// Ensure valid JSON for Dolt
		if len(result) == 0 {
			result = []byte("{}")
		}

		tx, err := dm.db.BeginTx(ctx, nil)
		if err != nil {
			return rigerrors.Wrap(err, "failed to begin transaction")
		}
		defer tx.Rollback() //nolint:errcheck

		// Lock the row and fetch current status atomically
		var currentStatus NodeStatus
		row := tx.QueryRowContext(ctx, "SELECT status FROM node_states WHERE id = ? FOR UPDATE", id)
		if err := row.Scan(&currentStatus); err != nil {
			if rigerrors.Is(err, sql.ErrNoRows) {
				return rigerrors.New("node state not found")
			}
			return rigerrors.Wrap(err, "failed to fetch current node status")
		}

		if !isValidNodeTransition(currentStatus, status) {
			return rigerrors.Newf("invalid node status transition from %s to %s", currentStatus, status)
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
			return rigerrors.Wrap(err, "failed to update node status")
		}

		if err := tx.Commit(); err != nil {
			return rigerrors.Wrap(err, "failed to commit node status update")
		}
		return nil
	})
}

// GetNodeStatesByExecution retrieves all node states for a given execution.
func (dm *DatabaseManager) GetNodeStatesByExecution(ctx context.Context, executionID string) ([]*NodeState, error) {
	query := `SELECT id, execution_id, node_id, status, started_at, completed_at, result, COALESCE(error, ''), created_at FROM node_states WHERE execution_id = ?`
	rows, err := dm.db.QueryContext(ctx, query, executionID)
	if err != nil {
		return nil, rigerrors.Wrap(err, "failed to get node states")
	}
	defer rows.Close()

	var states []*NodeState
	for rows.Next() {
		ns := &NodeState{}
		var result []byte
		if err := rows.Scan(&ns.ID, &ns.ExecutionID, &ns.NodeID, &ns.Status, &ns.StartedAt, &ns.CompletedAt, &result, &ns.Error, &ns.CreatedAt); err != nil {
			return nil, rigerrors.Wrap(err, "failed to scan node state")
		}
		ns.Result = result
		states = append(states, ns)
	}
	if err := rows.Err(); err != nil {
		return nil, rigerrors.Wrap(err, "error iterating node state rows")
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
		return false, rigerrors.Wrap(err, "failed to check active executions")
	}
	return count > 0, nil
}

// txHasActiveExecutions is like HasActiveExecutions but operates within a transaction.
func (dm *DatabaseManager) txHasActiveExecutions(ctx context.Context, tx *sql.Tx, workflowID string) (bool, error) {
	query := `SELECT COUNT(*) FROM executions WHERE workflow_id = ? AND status IN ('PENDING', 'RUNNING')`
	var count int
	err := tx.QueryRowContext(ctx, query, workflowID).Scan(&count)
	if err != nil {
		return false, rigerrors.Wrap(err, "failed to check active executions in tx")
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
		return rigerrors.Wrap(err, "failed to CALL DOLT_ADD in tx")
	}
	if _, err := tx.ExecContext(ctx, "CALL DOLT_COMMIT('-m', ?)", message); err != nil {
		return rigerrors.Wrap(err, "failed to CALL DOLT_COMMIT in tx")
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
		return nil, rigerrors.Wrap(err, "failed to query dolt_log")
	}
	defer rows.Close()

	var commits []DoltCommitInfo
	for rows.Next() {
		var c DoltCommitInfo
		if err := rows.Scan(&c.Hash, &c.Author, &c.Message, &c.Timestamp); err != nil {
			return nil, rigerrors.Wrap(err, "failed to scan commit info")
		}
		commits = append(commits, c)
	}
	if err := rows.Err(); err != nil {
		return nil, rigerrors.Wrap(err, "error iterating dolt log rows")
	}
	return commits, nil
}

// PruneExecutions removes executions and their associated node states that are older than the specified cutoff time.
// Only executions with terminal statuses (SUCCESS, FAILED, CANCELLED) are eligible for pruning.
// If dryRun is true, it returns the counts of rows that would be deleted without deleting them.
func (dm *DatabaseManager) PruneExecutions(ctx context.Context, cutoff time.Time, dryRun bool) (*PruneResult, error) {
	tx, err := dm.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, rigerrors.Wrap(err, "failed to begin transaction")
	}
	defer tx.Rollback() //nolint:errcheck

	// Ensure we are using the correct database for Dolt system procedures
	if _, err := tx.ExecContext(ctx, "USE rig_orchestration"); err != nil {
		return nil, rigerrors.Wrap(err, "failed to use rig_orchestration database")
	}

	// 1. Identify executions to be pruned (terminal status AND older than cutoff)
	terminalStatuses := []interface{}{string(ExecutionStatusSuccess), string(ExecutionStatusFailed), string(ExecutionStatusCancelled)}
	args := append(terminalStatuses, cutoff)

	// Dry-run: count and return without deleting
	if dryRun {
		var execCount int
		const countExecQuery = `SELECT COUNT(*) FROM executions WHERE status IN (?, ?, ?) AND created_at < ?`
		if err := tx.QueryRowContext(ctx, countExecQuery, args...).Scan(&execCount); err != nil {
			return nil, rigerrors.Wrap(err, "failed to count executions for pruning")
		}
		var nodeStateCount int
		const countNSQuery = `SELECT COUNT(*) FROM node_states WHERE execution_id IN (SELECT id FROM executions WHERE status IN (?, ?, ?) AND created_at < ?)`
		if err := tx.QueryRowContext(ctx, countNSQuery, args...).Scan(&nodeStateCount); err != nil {
			return nil, rigerrors.Wrap(err, "failed to count node states for pruning")
		}
		return &PruneResult{ExecutionsPruned: execCount, NodeStatesPruned: nodeStateCount, CutoffTime: cutoff}, nil
	}

	// 2. Count node_states before deletion (cascade will remove them with executions)
	var nodeStateCount int
	const countNSQuery = `SELECT COUNT(*) FROM node_states WHERE execution_id IN (SELECT id FROM executions WHERE status IN (?, ?, ?) AND created_at < ?)`
	if err := tx.QueryRowContext(ctx, countNSQuery, args...).Scan(&nodeStateCount); err != nil {
		return nil, rigerrors.Wrap(err, "failed to count node states for pruning")
	}

	// 3. Perform deletion and use RowsAffected as authoritative execution count
	const deleteExecQuery = `DELETE FROM executions WHERE status IN (?, ?, ?) AND created_at < ?`
	result, err := tx.ExecContext(ctx, deleteExecQuery, args...)
	if err != nil {
		return nil, rigerrors.Wrap(err, "failed to prune executions")
	}
	execAffected, err := result.RowsAffected()
	if err != nil {
		return nil, rigerrors.Wrap(err, "failed to get rows affected after execution prune")
	}

	res := &PruneResult{
		ExecutionsPruned: int(execAffected),
		NodeStatesPruned: nodeStateCount,
		CutoffTime:       cutoff,
	}

	if execAffected == 0 {
		return res, nil
	}

	// 4. Create Dolt commit for the deletion
	msg := fmt.Sprintf("orchestration: Pruned %d executions and %d node states older than %s", execAffected, nodeStateCount, cutoff.Format(time.RFC3339))
	if err := txAutoCommit(ctx, tx, msg); err != nil {
		return nil, rigerrors.Wrap(err, "failed to dolt-commit after prune")
	}

	if err := tx.Commit(); err != nil {
		return nil, rigerrors.Wrap(err, "failed to commit transaction")
	}

	return res, nil
}

// DoltGC runs the Dolt garbage collection procedure to reclaim space.
// This is a best-effort operation as the embedded driver may not support it.
func (dm *DatabaseManager) DoltGC(ctx context.Context) error {
	conn, err := dm.db.Conn(ctx)
	if err != nil {
		return rigerrors.Wrap(err, "failed to acquire database connection")
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, "USE rig_orchestration"); err != nil {
		return rigerrors.Wrap(err, "failed to use rig_orchestration database")
	}

	if _, err := conn.ExecContext(ctx, "CALL DOLT_GC()"); err != nil {
		errMsg := err.Error()
		// Silently skip if the procedure is not supported by the driver/version.
		if strings.Contains(errMsg, "not supported") || strings.Contains(errMsg, "unknown procedure") || strings.Contains(errMsg, "not found") {
			if dm.Verbose {
				fmt.Fprintf(os.Stderr, "[orchestration] Dolt GC not supported: %v\n", err)
			}
			return nil
		}
		return rigerrors.Wrap(err, "failed to CALL DOLT_GC")
	}

	return nil
}

// ExecutionArchive represents an execution and its associated node states for archival.
type ExecutionArchive struct {
	Execution  *Execution   `json:"execution"`
	NodeStates []*NodeState `json:"node_states"`
}

// ExportExecutionsBeforeCutoff exports executions and their associated node states older than the cutoff to a JSON file.
// Only executions with terminal statuses (SUCCESS, FAILED, CANCELLED) are eligible for archival.
// Returns the count of exported executions and the absolute path to the archive file.
func (dm *DatabaseManager) ExportExecutionsBeforeCutoff(ctx context.Context, cutoff time.Time, archiveDir string) (int, string, error) {
	// 1. Fetch executions to be archived (terminal status AND older than cutoff)
	terminalStatuses := []interface{}{string(ExecutionStatusSuccess), string(ExecutionStatusFailed), string(ExecutionStatusCancelled)}
	const execQuery = `SELECT id, workflow_id, workflow_version, status, started_at, completed_at, COALESCE(error, ''), created_at FROM executions WHERE status IN (?, ?, ?) AND created_at < ?`
	execArgs := append(terminalStatuses, cutoff)

	rows, err := dm.db.QueryContext(ctx, execQuery, execArgs...)
	if err != nil {
		return 0, "", rigerrors.Wrap(err, "failed to query executions for export")
	}
	defer rows.Close()

	var executions []*Execution
	for rows.Next() {
		e := &Execution{}
		if err := rows.Scan(&e.ID, &e.WorkflowID, &e.WorkflowVersion, &e.Status, &e.StartedAt, &e.CompletedAt, &e.Error, &e.CreatedAt); err != nil {
			return 0, "", rigerrors.Wrap(err, "failed to scan execution for export")
		}
		executions = append(executions, e)
	}
	if err := rows.Err(); err != nil {
		return 0, "", rigerrors.Wrap(err, "error iterating execution rows for export")
	}

	if len(executions) == 0 {
		return 0, "", nil
	}

	// 2. Batch-fetch all node states via JOIN (avoids N+1 queries and dynamic SQL)
	const nsQuery = `SELECT ns.id, ns.execution_id, ns.node_id, ns.status, ns.started_at, ns.completed_at, ns.result, COALESCE(ns.error, ''), ns.created_at
		FROM node_states ns INNER JOIN executions e ON ns.execution_id = e.id
		WHERE e.status IN (?, ?, ?) AND e.created_at < ?`

	nsRows, err := dm.db.QueryContext(ctx, nsQuery, execArgs...)
	if err != nil {
		return 0, "", rigerrors.Wrap(err, "failed to query node states for export")
	}
	defer nsRows.Close()

	statesByExec := make(map[string][]*NodeState)
	for nsRows.Next() {
		ns := &NodeState{}
		var result []byte
		if err := nsRows.Scan(&ns.ID, &ns.ExecutionID, &ns.NodeID, &ns.Status, &ns.StartedAt, &ns.CompletedAt, &result, &ns.Error, &ns.CreatedAt); err != nil {
			return 0, "", rigerrors.Wrap(err, "failed to scan node state for export")
		}
		ns.Result = result
		statesByExec[ns.ExecutionID] = append(statesByExec[ns.ExecutionID], ns)
	}
	if err := nsRows.Err(); err != nil {
		return 0, "", rigerrors.Wrap(err, "error iterating node state rows for export")
	}

	// 3. Assemble archives
	archives := make([]*ExecutionArchive, len(executions))
	for i, e := range executions {
		archives[i] = &ExecutionArchive{
			Execution:  e,
			NodeStates: statesByExec[e.ID],
		}
	}

	// 4. Write to file
	if err := os.MkdirAll(archiveDir, 0700); err != nil {
		return 0, "", rigerrors.Wrap(err, "failed to create archive directory")
	}

	fileName := fmt.Sprintf("orchestration_%s.json", cutoff.Format("2006-01-02_150405"))
	filePath := filepath.Join(archiveDir, fileName)

	data, err := json.MarshalIndent(archives, "", "  ")
	if err != nil {
		return 0, "", rigerrors.Wrap(err, "failed to marshal archives")
	}

	if err := os.WriteFile(filePath, data, 0600); err != nil {
		return 0, "", rigerrors.Wrap(err, "failed to write archive file")
	}

	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return 0, "", rigerrors.Wrap(err, "failed to resolve absolute path for archive")
	}

	return len(archives), absPath, nil
}

// withRetry wraps a database operation with exponential backoff retry logic.
// It is specifically designed to handle Dolt serialization errors (deadlocks).
// The provided function 'fn' should perform the entire transaction (Begin -> Commit).
func (dm *DatabaseManager) withRetry(ctx context.Context, operation string, fn func() error) error {
	cfg := rigerrors.RetryConfig{
		MaxRetries: 5,                      // A bit more than default for DB
		BaseDelay:  100 * time.Millisecond, // Dolt conflicts resolve quickly
		MaxDelay:   2 * time.Second,
		Jitter:     0.4,
	}

	return rigerrors.Retry(ctx, cfg, func() error {
		err := fn()
		if err == nil {
			return nil
		}

		// If it's a Dolt serialization error, wrap it in a retryable DatabaseError
		if rigerrors.IsDoltSerializationError(err) {
			if dm.Verbose {
				log.Printf("[orchestration] retrying %s due to conflict: %v", operation, err)
			}
			msg := err.Error()
			code := 1213
			if strings.Contains(msg, "Error 1205") {
				code = 1205
			}
			dbErr := rigerrors.NewDatabaseError(operation, msg, code)
			dbErr.Cause = err
			return dbErr
		}

		return err
	})
}
