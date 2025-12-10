package history

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// DatabaseManager handles SQLite history database operations
type DatabaseManager struct {
	DatabasePath string
	Verbose      bool
}

// NewDatabaseManager creates a new DatabaseManager
func NewDatabaseManager(databasePath string, verbose bool) *DatabaseManager {
	return &DatabaseManager{
		DatabasePath: databasePath,
		Verbose:      verbose,
	}
}

// Command represents a command from the history database
type Command struct {
	ID        int64
	Command   string
	Timestamp time.Time
	Duration  int64 // milliseconds
	ExitCode  int
	Directory string
	Session   string
	Host      string
}

// QueryOptions defines filtering options for history queries
type QueryOptions struct {
	Since     *time.Time
	Until     *time.Time
	Directory string
	Session   string
	Ticket    string
	ExitCode  *int
	Limit     int
	Pattern   string
}

// IsAvailable checks if the history database exists and is accessible
func (dm *DatabaseManager) IsAvailable() bool {
	if _, err := os.Stat(dm.DatabasePath); os.IsNotExist(err) {
		if dm.Verbose {
			fmt.Printf("History database not found at: %s\n", dm.DatabasePath)
		}
		return false
	}

	// Try to open and query the database
	db, err := sql.Open("sqlite3", dm.DatabasePath)
	if err != nil {
		if dm.Verbose {
			fmt.Printf("Failed to open history database: %v\n", err)
		}
		return false
	}
	defer db.Close()

	// Check if the expected tables exist
	var tableName string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name IN ('commands', 'history')").Scan(&tableName)
	if err != nil {
		if dm.Verbose {
			fmt.Printf("History database doesn't contain expected tables: %v\n", err)
		}
		return false
	}

	return true
}

// QueryCommands queries the history database for commands matching the given options
func (dm *DatabaseManager) QueryCommands(options QueryOptions) ([]Command, error) {
	if !dm.IsAvailable() {
		return nil, errors.New("history database not available")
	}

	db, err := sql.Open("sqlite3", dm.DatabasePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Detect database schema (zsh-histdb vs atuin)
	schema, err := dm.detectSchema(db)
	if err != nil {
		return nil, fmt.Errorf("failed to detect database schema: %w", err)
	}

	query, args := dm.buildQuery(schema, options)

	if dm.Verbose {
		fmt.Printf("Executing query: %s\n", query)
		fmt.Printf("With args: %v\n", args)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	var commands []Command
	for rows.Next() {
		command, err := dm.scanCommand(rows, schema)
		if err != nil {
			return nil, fmt.Errorf("failed to scan command: %w", err)
		}
		commands = append(commands, command)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error during row iteration: %w", err)
	}

	return commands, nil
}

// DatabaseSchema represents the type of history database
type DatabaseSchema string

const (
	SchemaZshHistdb DatabaseSchema = "zsh-histdb"
	SchemaAtuin     DatabaseSchema = "atuin"
	SchemaUnknown   DatabaseSchema = "unknown"
)

// detectSchema detects which type of history database we're working with
func (dm *DatabaseManager) detectSchema(db *sql.DB) (DatabaseSchema, error) {
	// Check for zsh-histdb tables
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name IN ('commands', 'places', 'sessions')").Scan(&count)
	if err == nil && count >= 2 {
		return SchemaZshHistdb, nil
	}

	// Check for atuin tables
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name IN ('history')").Scan(&count)
	if err == nil && count >= 1 {
		return SchemaAtuin, nil
	}

	return SchemaUnknown, errors.New("unknown database schema")
}

// buildQuery builds the SQL query based on schema and options
func (dm *DatabaseManager) buildQuery(schema DatabaseSchema, options QueryOptions) (string, []interface{}) {
	var query string
	var args []interface{}

	switch schema {
	case SchemaZshHistdb:
		query, args = dm.buildZshHistdbQuery(options)
	case SchemaAtuin:
		query, args = dm.buildAtuinQuery(options)
	default:
		// Fallback - try a generic query
		query = "SELECT 1, 'unknown', datetime('now'), 0, 0, '', '', ''"
		args = []interface{}{}
	}

	return query, args
}

// buildZshHistdbQuery builds a query for zsh-histdb schema
func (dm *DatabaseManager) buildZshHistdbQuery(options QueryOptions) (string, []interface{}) {
	query := `
		SELECT 
			c.rowid,
			c.argv,
			datetime(c.start_time, 'unixepoch'),
			COALESCE(c.duration, 0),
			COALESCE(c.exit_status, 0),
			COALESCE(p.dir, ''),
			COALESCE(s.session, ''),
			COALESCE(c.hostname, '')
		FROM commands c
		LEFT JOIN places p ON c.place_id = p.id
		LEFT JOIN sessions s ON c.session_id = s.id
		WHERE 1=1`

	var args []interface{}

	if options.Since != nil {
		query += " AND c.start_time >= ?"
		args = append(args, options.Since.Unix())
	}

	if options.Until != nil {
		query += " AND c.start_time <= ?"
		args = append(args, options.Until.Unix())
	}

	if options.Directory != "" {
		query += " AND p.dir LIKE ?"
		args = append(args, options.Directory+"%")
	}

	if options.Session != "" {
		query += " AND s.session LIKE ?"
		args = append(args, "%"+options.Session+"%")
	}

	if options.ExitCode != nil {
		query += " AND c.exit_status = ?"
		args = append(args, *options.ExitCode)
	}

	if options.Pattern != "" {
		query += " AND c.argv LIKE ?"
		args = append(args, "%"+options.Pattern+"%")
	}

	// Filter by ticket if specified (look for ticket in environment or session)
	if options.Ticket != "" && strings.TrimSpace(options.Ticket) != "" {
		query += " AND (s.session LIKE ? OR c.argv LIKE ?)"
		args = append(args, "%"+options.Ticket+"%", "%"+options.Ticket+"%")
	}

	query += " ORDER BY c.start_time ASC"

	if options.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, options.Limit)
	}

	return query, args
}

// buildAtuinQuery builds a query for atuin schema
func (dm *DatabaseManager) buildAtuinQuery(options QueryOptions) (string, []interface{}) {
	query := `
		SELECT 
			rowid,
			command,
			datetime(timestamp / 1000000000, 'unixepoch'),
			duration,
			exit,
			cwd,
			session,
			hostname
		FROM history
		WHERE 1=1`

	var args []interface{}

	if options.Since != nil {
		query += " AND timestamp >= ?"
		args = append(args, options.Since.UnixNano())
	}

	if options.Until != nil {
		query += " AND timestamp <= ?"
		args = append(args, options.Until.UnixNano())
	}

	if options.Directory != "" {
		query += " AND cwd LIKE ?"
		args = append(args, options.Directory+"%")
	}

	if options.Session != "" {
		query += " AND session LIKE ?"
		args = append(args, "%"+options.Session+"%")
	}

	if options.ExitCode != nil {
		query += " AND exit = ?"
		args = append(args, *options.ExitCode)
	}

	if options.Pattern != "" {
		query += " AND command LIKE ?"
		args = append(args, "%"+options.Pattern+"%")
	}

	if options.Ticket != "" {
		query += " AND (session LIKE ? OR command LIKE ?)"
		args = append(args, "%"+options.Ticket+"%", "%"+options.Ticket+"%")
	}

	query += " ORDER BY timestamp ASC"

	if options.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, options.Limit)
	}

	return query, args
}

// scanCommand scans a database row into a Command struct
func (dm *DatabaseManager) scanCommand(rows *sql.Rows, schema DatabaseSchema) (Command, error) {
	var cmd Command
	var timestampStr string

	err := rows.Scan(
		&cmd.ID,
		&cmd.Command,
		&timestampStr,
		&cmd.Duration,
		&cmd.ExitCode,
		&cmd.Directory,
		&cmd.Session,
		&cmd.Host,
	)

	if err != nil {
		return cmd, err
	}

	// Parse timestamp
	timestamp, err := time.Parse("2006-01-02 15:04:05", timestampStr)
	if err != nil {
		// Try alternative format
		timestamp, err = time.Parse(time.RFC3339, timestampStr)
		if err != nil {
			if dm.Verbose {
				fmt.Printf("Warning: Could not parse timestamp: %s\n", timestampStr)
			}
			timestamp = time.Now()
		}
	}
	cmd.Timestamp = timestamp

	return cmd, nil
}

// GetDatabaseInfo returns information about the history database
func (dm *DatabaseManager) GetDatabaseInfo() (map[string]interface{}, error) {
	info := make(map[string]interface{})

	// Check if database exists
	if _, err := os.Stat(dm.DatabasePath); os.IsNotExist(err) {
		info["exists"] = false
		info["path"] = dm.DatabasePath
		return info, nil
	}

	info["exists"] = true
	info["path"] = dm.DatabasePath

	// Get file info
	fileInfo, err := os.Stat(dm.DatabasePath)
	if err != nil {
		return info, fmt.Errorf("failed to get file info: %w", err)
	}

	info["size"] = fileInfo.Size()
	info["modified"] = fileInfo.ModTime()

	// Open database and get more info
	db, err := sql.Open("sqlite3", dm.DatabasePath)
	if err != nil {
		info["error"] = err.Error()
		return info, nil
	}
	defer db.Close()

	// Detect schema
	schema, err := dm.detectSchema(db)
	if err != nil {
		info["schema"] = "unknown"
		info["error"] = err.Error()
	} else {
		info["schema"] = string(schema)

		// Get command count
		// Note: tableName is safe for SQL concatenation because it can only be one of two
		// hardcoded string literals ("history" or "commands") determined by internal schema
		// detection logic. This is not user-controlled input.
		var count int64
		tableName := "history"
		if schema == SchemaZshHistdb {
			tableName = "commands"
		}

		err = db.QueryRow("SELECT COUNT(*) FROM " + tableName).Scan(&count)
		if err == nil {
			info["command_count"] = count
		}
	}

	return info, nil
}
