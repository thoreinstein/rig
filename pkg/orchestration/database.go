package orchestration

import (
	"database/sql"
	"fmt"

	"github.com/cockroachdb/errors"
	_ "github.com/go-sql-driver/mysql"
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
