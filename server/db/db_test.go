package db

import (
	"context"
	"log/slog"
	"os"
	"runtime"
	"testing"
)

func TestNewDB(t *testing.T) {
	dbPath := "file::memory:?cache=shared"

	// Test case: NewDB creates a valid database instance
	db, err := NewDB(dbPath, slog.Default())
	if err != nil {
		t.Errorf("NewDB failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Test case: ReadDB has the correct maximum open connections
	expectedReadConns := int(max(4, float64(runtime.NumCPU())))
	if db.ReadDB.Stats().MaxOpenConnections != expectedReadConns {
		t.Errorf("ReadDB MaxOpenConns: expected %d, got %d", expectedReadConns, db.ReadDB.Stats().MaxOpenConnections)
	}

	// Test case: WriteDB has the correct maximum open connections
	if db.WriteDB.Stats().MaxOpenConnections != 1 {
		t.Errorf("WriteDB MaxOpenConns: expected 1, got %d", db.WriteDB.Stats().MaxOpenConnections)
	}
}

func TestSelect(t *testing.T) {
	// Create an in-memory SQLite database
	// > Each connection to ":memory:" opens a brand new in-memory sql
	// > database, so if the stdlib's sql engine happens to open another
	// > connection and you've only specified ":memory:", that connection will
	// > see a brand new database. A workaround is to use
	// > "file::memory:?cache=shared" (or
	// > "file:foobar?mode=memory&cache=shared"). Every connection to this
	// > string will point to the same in-memory database.
	//
	// https://github.com/mattn/go-sqlite3/tree/3c0390b77?tab=readme-ov-file#faq
	dbPath := "file::memory:?cache=shared"
	db, err := NewDB(dbPath, slog.Default())
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Create a test table
	_, err = db.ExecContext(context.Background(), "CREATE TABLE test (id INTEGER PRIMARY KEY, name TEXT)")
	if err != nil {
		t.Fatalf("Failed to create test table: %v", err)
	}

	// Test case: Select from an empty table
	rows, err := db.QueryContext(context.Background(), "SELECT * FROM test")
	if err != nil {
		t.Fatalf("Select failed: %v", err)
	}
	defer func() { _ = rows.Close() }()

	// Test case: Execute INSERT and SELECT
	_, err = db.ExecContext(context.Background(), "INSERT INTO test (name) VALUES (?)", "John Doe")
	if err != nil {
		t.Fatalf("Execute INSERT failed: %v", err)
	}

	rows, err = db.QueryContext(context.Background(), "SELECT * FROM test")
	if err != nil {
		t.Fatalf("Select failed: %v", err)
	}
	defer func() { _ = rows.Close() }()

	// Check if there is at least one row
	if !rows.Next() {
		t.Error("Expected at least one row in the result set")
	}
}

func TestRunSQLFile(t *testing.T) {
	// Create an in-memory SQLite database
	dbPath := "file::memory:?cache=shared"
	db, err := NewDB(dbPath, slog.Default())
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Create a temporary SQL file
	sqlFile, err := os.CreateTemp("", "test*.sql")
	if err != nil {
		t.Fatalf("Failed to create temporary SQL file: %v", err)
	}
	defer func() { _ = os.Remove(sqlFile.Name()) }()

	// Write SQL statements to the temporary file
	_, err = sqlFile.WriteString(`
		CREATE TABLE users (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL
		);

		-- Comment

		CREATE INDEX idx_users_name ON users(name);
	`)
	if err != nil {
		t.Fatalf("Failed to write to temporary SQL file: %v", err)
	}
	err = sqlFile.Close()
	if err != nil {
		t.Fatalf("Failed to close temporary SQL file: %v", err)
	}

	// Execute the SQL file
	err = db.RunSQLFile(sqlFile.Name())
	if err != nil {
		t.Errorf("RunSQLFile failed: %v", err)
	}

	// Check if the table and index were created
	_, err = db.QueryContext(context.Background(), "SELECT * FROM users")
	if err != nil {
		t.Errorf("Failed to select from users table: %v", err)
	}

	_, err = db.QueryContext(context.Background(), "SELECT * FROM sqlite_master WHERE type='index' AND name='idx_users_name'")
	if err != nil {
		t.Errorf("Failed to check for index: %v", err)
	}
}
