package events

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	_ "github.com/dolthub/driver"
)

// DatabaseManager handles embedded Dolt database operations for event tracking.
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
		return nil, errors.Wrap(err, "failed to resolve absolute data path")
	}

	u := &url.URL{
		Scheme: "file",
		Path:   absPath,
		RawQuery: url.Values{
			"commitname":  {commitName},
			"commitemail": {commitEmail},
			"database":    {"rig_events"},
		}.Encode(),
	}
	dsn := u.String()

	db, err := sql.Open("dolt", dsn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open embedded dolt database")
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
		return errors.Wrap(err, "failed to create data path")
	}

	ctx := context.Background()
	conn, err := dm.db.Conn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to acquire database connection")
	}
	defer conn.Close()

	// Create database if not exists
	if _, err := conn.ExecContext(ctx, "CREATE DATABASE IF NOT EXISTS rig_events"); err != nil {
		return errors.Wrap(err, "failed to create rig_events database")
	}

	// Use database (session state persists on this single connection)
	if _, err := conn.ExecContext(ctx, "USE rig_events"); err != nil {
		return errors.Wrap(err, "failed to use rig_events database")
	}

	// Run table DDLs
	for _, ddl := range AllTableDDLs() {
		if _, err := conn.ExecContext(ctx, ddl); err != nil {
			return errors.Wrap(err, "failed to execute table DDL")
		}
	}

	return nil
}

// BackfillTicket retroactively tags events with a ticket ID in the metadata column.
// Only updates rows where metadata IS NULL, preserving any previously-set metadata.
func (dm *DatabaseManager) BackfillTicket(ctx context.Context, correlationID, ticket string) error {
	m := map[string]string{"ticket": ticket}
	metadata, err := json.Marshal(m)
	if err != nil {
		return errors.Wrap(err, "failed to marshal metadata")
	}

	query := `UPDATE workflow_events SET metadata = ? WHERE correlation_id = ? AND metadata IS NULL`
	result, err := dm.db.ExecContext(ctx, query, string(metadata), correlationID)
	if err != nil {
		return errors.Wrap(err, "failed to backfill ticket metadata")
	}

	if dm.Verbose {
		if n, rowErr := result.RowsAffected(); rowErr == nil {
			fmt.Fprintf(os.Stderr, "[events] backfilled %d event(s) with ticket %s\n", n, ticket)
		}
	}

	return nil
}

// PruneEvents removes events older than the specified cutoff time.
// If dryRun is true, it returns the count of rows that would be deleted without deleting them.
func (dm *DatabaseManager) PruneEvents(ctx context.Context, cutoff time.Time, dryRun bool) (*PruneResult, error) {
	conn, err := dm.db.Conn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to acquire database connection")
	}
	defer conn.Close()

	// Ensure we are using the correct database for Dolt system procedures
	if _, err := conn.ExecContext(ctx, "USE rig_events"); err != nil {
		return nil, errors.Wrap(err, "failed to use rig_events database")
	}

	// 1. Count rows to be pruned
	var count int
	countQuery := `SELECT COUNT(*) FROM workflow_events WHERE created_at < ?`
	if err := conn.QueryRowContext(ctx, countQuery, cutoff).Scan(&count); err != nil {
		return nil, errors.Wrap(err, "failed to count events for pruning")
	}

	res := &PruneResult{
		EventsDeleted: count,
		CutoffTime:    cutoff,
	}

	if dryRun || count == 0 {
		return res, nil
	}

	// 2. Perform deletion
	deleteQuery := `DELETE FROM workflow_events WHERE created_at < ?`
	if _, err := conn.ExecContext(ctx, deleteQuery, cutoff); err != nil {
		return nil, errors.Wrap(err, "failed to prune events")
	}

	// 3. Create Dolt commit for the deletion
	msg := fmt.Sprintf("events: Pruned %d events older than %s", count, cutoff.Format(time.RFC3339))
	if _, err := conn.ExecContext(ctx, "CALL DOLT_ADD('-A')"); err != nil {
		return nil, errors.Wrap(err, "failed to CALL DOLT_ADD after prune")
	}
	if _, err := conn.ExecContext(ctx, "CALL DOLT_COMMIT('-m', ?)", msg); err != nil {
		return nil, errors.Wrap(err, "failed to CALL DOLT_COMMIT after prune")
	}

	return res, nil
}

// DoltGC runs the Dolt garbage collection procedure to reclaim space.
// This is a best-effort operation as the embedded driver may not support it.
func (dm *DatabaseManager) DoltGC(ctx context.Context) error {
	conn, err := dm.db.Conn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to acquire database connection")
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, "USE rig_events"); err != nil {
		return errors.Wrap(err, "failed to use rig_events database")
	}

	if _, err := conn.ExecContext(ctx, "CALL DOLT_GC()"); err != nil {
		errMsg := err.Error()
		// Silently skip if the procedure is not supported by the driver/version.
		if strings.Contains(errMsg, "not supported") || strings.Contains(errMsg, "unknown procedure") || strings.Contains(errMsg, "not found") {
			if dm.Verbose {
				fmt.Fprintf(os.Stderr, "[events] Dolt GC not supported: %v\n", err)
			}
			return nil
		}
		return errors.Wrap(err, "failed to CALL DOLT_GC")
	}

	return nil
}
