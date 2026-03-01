package events

import (
	"database/sql"
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
func (dm *DatabaseManager) InitDatabase() error {
	// Ensure directory exists
	if err := os.MkdirAll(dm.dataPath, 0700); err != nil {
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
