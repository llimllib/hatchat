package db

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"runtime"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// reference: https://kerkour.com/sqlite-for-servers
type DB struct {
	ReadDB  *sql.DB
	WriteDB *sql.DB
	logger  *slog.Logger
	mu      sync.RWMutex
}

func NewDB(dbUrl string, logger *slog.Logger) (*DB, error) {
	// we want to add a few parameters, so parse the db URL
	readUrl, err := url.Parse(dbUrl)
	if err != nil {
		return nil, fmt.Errorf("error parsing connection string: %v", err)
	}

	// make a copy of the URL so we can safely append write params
	writeUrl := *readUrl

	// add readonly mode flag and open database
	// docs on connection flags:
	// https://pkg.go.dev/github.com/mattn/go-sqlite3#readme-connection-string
	readParams := readUrl.Query()
	readParams.Add("mode", "ro")
	readUrl.RawQuery = readParams.Encode()
	logger.Debug("connecting read db", "url", readUrl.String())
	readDB, err := sql.Open("sqlite3", readUrl.String())
	if err != nil {
		return nil, err
	}
	readDB.SetMaxOpenConns(max(4, runtime.NumCPU()))
	setSQLitePragmas(readDB)

	// Add the _txlock=immediate flag
	writeParams := writeUrl.Query()
	writeParams.Add("_txlock", "immediate")
	writeUrl.RawQuery = writeParams.Encode()

	logger.Debug("connecting write db", "url", writeUrl.String())
	writeDB, err := sql.Open("sqlite3", writeUrl.String())
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

// Make a query using the read connection
func (db *DB) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	db.logger.Debug("querying", "query", query, "args", args)
	t := time.Now()
	rows, err := db.ReadDB.QueryContext(ctx, query, args...)
	db.logger.Debug("querying", "query", query, "args", args, "duration", time.Since(t))
	return rows, err
}

// Make a query using the read connection and return the first row
func (db *DB) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	db.mu.RLock()
	defer db.mu.RUnlock()
	t := time.Now()
	row := db.ReadDB.QueryRowContext(ctx, query, args...)
	db.logger.Debug("querying row", "query", query, "args", args, "duration", time.Since(t))
	return row
}

// Execute a query using the write connection
func (db *DB) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	t := time.Now()
	res, err := db.WriteDB.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	db.logger.Debug("executed", "query", query, "args", args, "duration", time.Since(t))
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

// RunSQLFile executes the SQL statements in the given file on the write
// connection
func (db *DB) RunSQLFile(filePath string) error {
	sqlfile, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	_, err = db.ExecContext(context.Background(), string(sqlfile))
	if err != nil {
		return err
	}

	return nil
}
