package db

import (
	"bufio"
	"database/sql"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"runtime"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

// reference: https://kerkour.com/sqlite-for-servers
type DB struct {
	ReadDB  *sql.DB
	WriteDB *sql.DB
	logger  *slog.Logger
}

func NewDB(dbPath string, logger *slog.Logger) (*DB, error) {
	readDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}
	readDB.SetMaxOpenConns(max(4, runtime.NumCPU()))
	setSQLitePragmas(readDB)

	// add _txlock=immediate for the write database
	u, err := url.Parse(dbPath)
	if err != nil {
		return nil, fmt.Errorf("error parsing connection string: %v", err)
	}

	// Add the _txlock=immediate parameter
	q := u.Query()
	q.Add("_txlock", "immediate")
	u.RawQuery = q.Encode()

	writeDB, err := sql.Open("sqlite3", u.String())
	if err != nil {
		readDB.Close()
		return nil, err
	}
	writeDB.SetMaxOpenConns(1)
	setSQLitePragmas(writeDB)

	return &DB{
		ReadDB:  readDB,
		WriteDB: writeDB,
		logger:  logger,
	}, nil
}

// Select executes a SELECT statement using the read connection
func (db *DB) Select(query string, args ...interface{}) (*sql.Rows, error) {
	db.logger.Debug("querying", "query", query, "args", args)
	return db.ReadDB.Query(query, args...)
}

// Execute executes a non-SELECT statement using the write connection
func (db *DB) Exec(query string, args ...interface{}) (sql.Result, error) {
	db.logger.Debug("executing", "query", query, "args", args)
	tx, err := db.WriteDB.Begin()
	if err != nil {
		return nil, err
	}

	res, err := tx.Exec(query, args...)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return res, nil
}

// Close closes both read and write connections
func (db *DB) Close() error {
	err1 := db.ReadDB.Close()
	err2 := db.WriteDB.Close()
	if err1 != nil {
		return err1
	}
	return err2
}

// Helper functions
func must(_ any, err error) {
	if err != nil {
		panic(err)
	}
}

func setSQLitePragmas(conn *sql.DB) {
	pragmas := []string{
		"PRAGMA journal_mode = WAL;",
		"PRAGMA busy_timeout = 5000;",
		"PRAGMA synchronous = NORMAL;",
		"PRAGMA cache_size = 1000000000;",
		"PRAGMA foreign_keys = true;",
		"PRAGMA temp_store = memory;",
	}
	for _, pragma := range pragmas {
		must(conn.Exec(pragma))
	}
}

// RunSQLFile executes the SQL statements in the given file on the write connection
func (db *DB) RunSQLFile(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var queries []string
	var currentQuery strings.Builder

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		if strings.HasPrefix(strings.TrimSpace(line), "--") {
			continue // Skip comments
		}

		currentQuery.WriteString(line)
		currentQuery.WriteString(" ")

		if strings.HasSuffix(strings.TrimSpace(line), ";") {
			queries = append(queries, currentQuery.String())
			currentQuery.Reset()
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	for _, query := range queries {
		_, err := db.Exec(query)
		if err != nil {
			return err
		}
	}

	return nil
}
