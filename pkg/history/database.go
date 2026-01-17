package history

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	_ "modernc.org/sqlite"
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

// IsAvailable checks if the history database exists and is accessible
func (dm *DatabaseManager) IsAvailable() bool {
	if _, err := os.Stat(dm.DatabasePath); os.IsNotExist(err) {
		if dm.Verbose {
			fmt.Printf("History database not found at: %s\n", dm.DatabasePath)
		}
		return false
	}

	// Try to open and query the database
	db, err := sql.Open("sqlite", dm.DatabasePath)
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

	db, err := sql.Open("sqlite", dm.DatabasePath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open database")
	}
	defer db.Close()

	// Detect database schema (zsh-histdb vs atuin)
	schema, err := dm.detectSchema(db)
	if err != nil {
		return nil, errors.Wrap(err, "failed to detect database schema")
	}

	query, args := dm.buildQuery(schema, options)

	if dm.Verbose {
		fmt.Printf("Executing query: %s\n", query)
		fmt.Printf("With args: %v\n", args)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute query")
	}
	defer rows.Close()

	var commands []Command
	for rows.Next() {
		command, err := dm.scanCommand(rows, schema)
		if err != nil {
			return nil, errors.Wrap(err, "failed to scan command")
		}
		commands = append(commands, command)
	}

	if err = rows.Err(); err != nil {
		return nil, errors.Wrap(err, "error during row iteration")
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

	if options.SessionID != "" {
		query += " AND s.session = ?"
		args = append(args, options.SessionID)
	}

	if options.ExitCode != nil {
		query += " AND c.exit_status = ?"
		args = append(args, *options.ExitCode)
	}

	if options.MinDuration > 0 {
		query += " AND c.duration >= ?"
		// zsh-histdb duration is usually seconds
		args = append(args, int64(options.MinDuration.Seconds()))
	}

	if options.Pattern != "" {
		query += " AND c.argv LIKE ?"
		args = append(args, "%"+options.Pattern+"%")
	}

	// Filter by ticket or project paths
	if (options.Ticket != "" && strings.TrimSpace(options.Ticket) != "") || len(options.ProjectPaths) > 0 {
		var orConditions []string

		if options.Ticket != "" && strings.TrimSpace(options.Ticket) != "" {
			orConditions = append(orConditions, "s.session LIKE ?", "c.argv LIKE ?")
			args = append(args, "%"+options.Ticket+"%", "%"+options.Ticket+"%")
		}

		for _, path := range options.ProjectPaths {
			orConditions = append(orConditions, "p.dir LIKE ?")
			args = append(args, path+"%")
		}

		if len(orConditions) > 0 {
			query += " AND (" + strings.Join(orConditions, " OR ") + ")"
		}
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

	if options.SessionID != "" {
		query += " AND session = ?"
		args = append(args, options.SessionID)
	}

	if options.ExitCode != nil {
		query += " AND exit = ?"
		args = append(args, *options.ExitCode)
	}

	if options.MinDuration > 0 {
		query += " AND duration >= ?"
		// Atuin duration is nanoseconds
		args = append(args, options.MinDuration.Nanoseconds())
	}

	if options.Pattern != "" {
		query += " AND command LIKE ?"
		args = append(args, "%"+options.Pattern+"%")
	}

	// Filter by ticket or project paths
	if (options.Ticket != "" && strings.TrimSpace(options.Ticket) != "") || len(options.ProjectPaths) > 0 {
		var orConditions []string

		if options.Ticket != "" && strings.TrimSpace(options.Ticket) != "" {
			orConditions = append(orConditions, "session LIKE ?", "command LIKE ?")
			args = append(args, "%"+options.Ticket+"%", "%"+options.Ticket+"%")
		}

		for _, path := range options.ProjectPaths {
			orConditions = append(orConditions, "cwd LIKE ?")
			args = append(args, path+"%")
		}

		if len(orConditions) > 0 {
			query += " AND (" + strings.Join(orConditions, " OR ") + ")"
		}
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
		return info, errors.Wrap(err, "failed to get file info")
	}

	info["size"] = fileInfo.Size()
	info["modified"] = fileInfo.ModTime()

	// Open database and get more info
	db, err := sql.Open("sqlite", dm.DatabasePath)
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
		// Determine table name using explicit switch for defense-in-depth.
		// Only hardcoded values are possible; this is not user-controlled input.
		var tableName string
		switch schema {
		case SchemaZshHistdb:
			tableName = "commands"
		default:
			tableName = "history"
		}

		var count int64
		err = db.QueryRow("SELECT COUNT(*) FROM " + tableName).Scan(&count)
		if err == nil {
			info["command_count"] = count
		}
	}

	return info, nil
}
