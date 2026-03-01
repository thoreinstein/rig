package events

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"

	"github.com/cockroachdb/errors"
	_ "github.com/dolthub/driver"
)

// DatabaseManager handles embedded Dolt database operations for event tracking.
type DatabaseManager struct {
	db      *sql.DB
	Verbose bool
}

// NewDatabaseManager creates a new DatabaseManager using the embedded Dolt driver.
func NewDatabaseManager(dataPath, commitName, commitEmail string, verbose bool) (*DatabaseManager, error) {
	// Build file-based DSN: file:///path/to/dbs?commitname=...&commitemail=...&database=...
	dsn := fmt.Sprintf("file://%s?commitname=%s&commitemail=%s&database=rig_events",
		dataPath,
		url.QueryEscape(commitName),
		url.QueryEscape(commitEmail))

	db, err := sql.Open("dolt", dsn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open embedded dolt database")
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

// InitDatabase initializes the database and creates tables.
func (dm *DatabaseManager) InitDatabase(dataPath string) error {
	// Ensure directory exists
	if err := os.MkdirAll(dataPath, 0755); err != nil {
		return errors.Wrap(err, "failed to create data path")
	}

	// Create database if not exists
	if _, err := dm.db.Exec("CREATE DATABASE IF NOT EXISTS rig_events"); err != nil {
		return errors.Wrap(err, "failed to create rig_events database")
	}

	// Use database
	if _, err := dm.db.Exec("USE rig_events"); err != nil {
		return errors.Wrap(err, "failed to use rig_events database")
	}

	// Run table DDLs
	for _, ddl := range AllTableDDLs() {
		if _, err := dm.db.Exec(ddl); err != nil {
			return errors.Wrap(err, "failed to execute table DDL")
		}
	}

	return nil
}
