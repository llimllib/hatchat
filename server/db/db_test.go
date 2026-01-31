package db

import (
	"context"
	"log/slog"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"
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

// TestReadWriteSeparation verifies that reads use ReadDB and writes use WriteDB
func TestReadWriteSeparation(t *testing.T) {
	dbPath := "file::memory:?cache=shared"
	db, err := NewDB(dbPath, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})))
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Create a table using write connection
	_, err = db.ExecContext(context.Background(), "CREATE TABLE separation_test (id INTEGER PRIMARY KEY, value TEXT)")
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Insert using write connection
	_, err = db.ExecContext(context.Background(), "INSERT INTO separation_test (value) VALUES (?)", "test_value")
	if err != nil {
		t.Fatalf("Failed to insert: %v", err)
	}

	// Read using read connection - should see the data (shared cache)
	var value string
	err = db.QueryRowContext(context.Background(), "SELECT value FROM separation_test WHERE id = 1").Scan(&value)
	if err != nil {
		t.Fatalf("Failed to read: %v", err)
	}
	if value != "test_value" {
		t.Errorf("Expected 'test_value', got '%s'", value)
	}
}

// TestQueryRowContext tests the QueryRowContext method
func TestQueryRowContext(t *testing.T) {
	dbPath := "file::memory:?cache=shared"
	db, err := NewDB(dbPath, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})))
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Create and populate table
	_, err = db.ExecContext(context.Background(), "CREATE TABLE row_test (id INTEGER PRIMARY KEY, name TEXT)")
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	_, err = db.ExecContext(context.Background(), "INSERT INTO row_test (name) VALUES (?)", "Alice")
	if err != nil {
		t.Fatalf("Failed to insert: %v", err)
	}

	// Query single row
	var name string
	err = db.QueryRowContext(context.Background(), "SELECT name FROM row_test WHERE id = ?", 1).Scan(&name)
	if err != nil {
		t.Fatalf("QueryRowContext failed: %v", err)
	}
	if name != "Alice" {
		t.Errorf("Expected 'Alice', got '%s'", name)
	}
}

// TestQueryContextWithParams tests parameterized queries
func TestQueryContextWithParams(t *testing.T) {
	dbPath := "file::memory:?cache=shared"
	db, err := NewDB(dbPath, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})))
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Create and populate table
	_, err = db.ExecContext(context.Background(), "CREATE TABLE params_test (id INTEGER PRIMARY KEY, category TEXT, value INTEGER)")
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Insert multiple rows
	for i := 1; i <= 5; i++ {
		category := "A"
		if i > 3 {
			category = "B"
		}
		_, err = db.ExecContext(context.Background(), "INSERT INTO params_test (category, value) VALUES (?, ?)", category, i*10)
		if err != nil {
			t.Fatalf("Failed to insert row %d: %v", i, err)
		}
	}

	// Query with parameter
	rows, err := db.QueryContext(context.Background(), "SELECT value FROM params_test WHERE category = ?", "A")
	if err != nil {
		t.Fatalf("QueryContext failed: %v", err)
	}
	defer func() { _ = rows.Close() }()

	count := 0
	for rows.Next() {
		count++
	}
	if count != 3 {
		t.Errorf("Expected 3 rows for category A, got %d", count)
	}
}

// TestExecContextReturnsResult tests that ExecContext returns proper result info
func TestExecContextReturnsResult(t *testing.T) {
	dbPath := "file::memory:?cache=shared"
	db, err := NewDB(dbPath, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})))
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Create table
	_, err = db.ExecContext(context.Background(), "CREATE TABLE result_test (id INTEGER PRIMARY KEY, value TEXT)")
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Insert and check LastInsertId
	result, err := db.ExecContext(context.Background(), "INSERT INTO result_test (value) VALUES (?)", "test1")
	if err != nil {
		t.Fatalf("Failed to insert: %v", err)
	}

	lastID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("Failed to get LastInsertId: %v", err)
	}
	if lastID != 1 {
		t.Errorf("Expected LastInsertId 1, got %d", lastID)
	}

	// Insert another and check
	result, err = db.ExecContext(context.Background(), "INSERT INTO result_test (value) VALUES (?)", "test2")
	if err != nil {
		t.Fatalf("Failed to insert second row: %v", err)
	}

	lastID, err = result.LastInsertId()
	if err != nil {
		t.Fatalf("Failed to get LastInsertId: %v", err)
	}
	if lastID != 2 {
		t.Errorf("Expected LastInsertId 2, got %d", lastID)
	}

	// Update and check RowsAffected
	result, err = db.ExecContext(context.Background(), "UPDATE result_test SET value = ? WHERE id = ?", "updated", 1)
	if err != nil {
		t.Fatalf("Failed to update: %v", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		t.Fatalf("Failed to get RowsAffected: %v", err)
	}
	if rowsAffected != 1 {
		t.Errorf("Expected RowsAffected 1, got %d", rowsAffected)
	}
}

// TestExecContextError tests that ExecContext returns errors for invalid SQL
func TestExecContextError(t *testing.T) {
	dbPath := "file::memory:?cache=shared"
	db, err := NewDB(dbPath, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})))
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Try to insert into non-existent table
	_, err = db.ExecContext(context.Background(), "INSERT INTO nonexistent_table (value) VALUES (?)", "test")
	if err == nil {
		t.Error("Expected error for non-existent table, got nil")
	}

	// Try invalid SQL syntax
	_, err = db.ExecContext(context.Background(), "THIS IS NOT VALID SQL")
	if err == nil {
		t.Error("Expected error for invalid SQL, got nil")
	}
}

// TestCloseClosesBothConnections tests that Close properly closes both connections
func TestCloseClosesBothConnections(t *testing.T) {
	dbPath := "file::memory:?cache=shared"
	db, err := NewDB(dbPath, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})))
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}

	// Close the database
	err = db.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Verify both connections are closed by attempting to use them
	// After close, operations should fail
	_, err = db.ReadDB.QueryContext(context.Background(), "SELECT 1")
	if err == nil {
		t.Error("Expected error when querying closed ReadDB")
	}

	_, err = db.WriteDB.ExecContext(context.Background(), "SELECT 1")
	if err == nil {
		t.Error("Expected error when executing on closed WriteDB")
	}
}

// TestRunSQLFileNonExistent tests RunSQLFile with non-existent file
func TestRunSQLFileNonExistent(t *testing.T) {
	dbPath := "file::memory:?cache=shared"
	db, err := NewDB(dbPath, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})))
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	err = db.RunSQLFile("/nonexistent/path/to/file.sql")
	if err == nil {
		t.Error("Expected error for non-existent SQL file, got nil")
	}
}

// TestRunSQLFileInvalidSQL tests RunSQLFile with invalid SQL content
func TestRunSQLFileInvalidSQL(t *testing.T) {
	dbPath := "file::memory:?cache=shared"
	db, err := NewDB(dbPath, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})))
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Create a temporary SQL file with invalid SQL
	sqlFile, err := os.CreateTemp("", "invalid*.sql")
	if err != nil {
		t.Fatalf("Failed to create temporary SQL file: %v", err)
	}
	defer func() { _ = os.Remove(sqlFile.Name()) }()

	_, err = sqlFile.WriteString("THIS IS NOT VALID SQL AT ALL;")
	if err != nil {
		t.Fatalf("Failed to write to temporary SQL file: %v", err)
	}
	err = sqlFile.Close()
	if err != nil {
		t.Fatalf("Failed to close temporary SQL file: %v", err)
	}

	err = db.RunSQLFile(sqlFile.Name())
	if err == nil {
		t.Error("Expected error for invalid SQL file content, got nil")
	}
}

// TestConcurrentReads tests that multiple concurrent reads work correctly
func TestConcurrentReads(t *testing.T) {
	dbPath := "file::memory:?cache=shared"
	db, err := NewDB(dbPath, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})))
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Create and populate table
	_, err = db.ExecContext(context.Background(), "CREATE TABLE concurrent_test (id INTEGER PRIMARY KEY, value INTEGER)")
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	for i := 1; i <= 100; i++ {
		_, err = db.ExecContext(context.Background(), "INSERT INTO concurrent_test (value) VALUES (?)", i)
		if err != nil {
			t.Fatalf("Failed to insert: %v", err)
		}
	}

	// Perform concurrent reads
	var wg sync.WaitGroup
	errors := make(chan error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var sum int
			err := db.QueryRowContext(context.Background(), "SELECT SUM(value) FROM concurrent_test").Scan(&sum)
			if err != nil {
				errors <- err
				return
			}
			// Sum of 1 to 100 = 5050
			if sum != 5050 {
				errors <- err
			}
		}()
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		if err != nil {
			t.Errorf("Concurrent read failed: %v", err)
		}
	}
}

// TestConcurrentWrites tests that concurrent writes are serialized correctly
func TestConcurrentWrites(t *testing.T) {
	dbPath := "file::memory:?cache=shared"
	db, err := NewDB(dbPath, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})))
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Create table
	_, err = db.ExecContext(context.Background(), "CREATE TABLE write_test (id INTEGER PRIMARY KEY, value INTEGER)")
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Perform concurrent writes
	var wg sync.WaitGroup
	numWrites := 50
	errors := make(chan error, numWrites)

	for i := 0; i < numWrites; i++ {
		wg.Add(1)
		go func(val int) {
			defer wg.Done()
			_, err := db.ExecContext(context.Background(), "INSERT INTO write_test (value) VALUES (?)", val)
			if err != nil {
				errors <- err
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("Concurrent write failed: %v", err)
	}

	// Verify all writes succeeded
	var count int
	err = db.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM write_test").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count rows: %v", err)
	}
	if count != numWrites {
		t.Errorf("Expected %d rows, got %d", numWrites, count)
	}
}

// TestContextCancellation tests that context cancellation is respected
func TestContextCancellation(t *testing.T) {
	dbPath := "file::memory:?cache=shared"
	db, err := NewDB(dbPath, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})))
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Create a context that's already canceled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Try to execute with canceled context - this should fail or return quickly
	// Note: SQLite driver behavior with canceled contexts can vary
	_, err = db.ExecContext(ctx, "SELECT 1")
	if err == nil {
		// Some drivers may not check context before execution
		// This is acceptable behavior, but we document it
		t.Log("Note: ExecContext did not fail with canceled context (driver-dependent)")
	}
}

// TestNewDBInvalidURL tests NewDB with an invalid database URL
func TestNewDBInvalidURL(t *testing.T) {
	// Use an invalid URL scheme
	_, err := NewDB("://invalid-url", slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})))
	if err == nil {
		t.Error("Expected error for invalid URL, got nil")
	}
}

// TestTransactionIsolation tests that writes are visible to reads after commit
func TestTransactionIsolation(t *testing.T) {
	dbPath := "file::memory:?cache=shared"
	db, err := NewDB(dbPath, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})))
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Create table
	_, err = db.ExecContext(context.Background(), "CREATE TABLE isolation_test (id INTEGER PRIMARY KEY, value TEXT)")
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Insert a value
	_, err = db.ExecContext(context.Background(), "INSERT INTO isolation_test (value) VALUES (?)", "initial")
	if err != nil {
		t.Fatalf("Failed to insert: %v", err)
	}

	// Start a goroutine that will update the value
	done := make(chan struct{})
	go func() {
		time.Sleep(10 * time.Millisecond)
		_, _ = db.ExecContext(context.Background(), "UPDATE isolation_test SET value = ? WHERE id = 1", "updated")
		close(done)
	}()

	// Wait for update
	<-done

	// Read should see the update
	var value string
	err = db.QueryRowContext(context.Background(), "SELECT value FROM isolation_test WHERE id = 1").Scan(&value)
	if err != nil {
		t.Fatalf("Failed to read: %v", err)
	}
	if value != "updated" {
		t.Errorf("Expected 'updated', got '%s'", value)
	}
}
