package events

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

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
	result, err := dm.db.ExecContext(ctx, query, metadata, correlationID)
	if err != nil {
		return errors.Wrap(err, "failed to backfill ticket metadata")
	}

	if dm.Verbose {
		if n, rowErr := result.RowsAffected(); rowErr == nil {
			fmt.Printf("[events] backfilled %d event(s) with ticket %s\n", n, ticket)
		}
	}

	return nil
}
