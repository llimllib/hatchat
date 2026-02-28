package db

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"
)

// testDBWithVersion creates a test database with just the schema_version table
func testDBWithVersion(t *testing.T, version int) *DB {
	t.Helper()
	dbPath := "file::memory:?cache=shared"
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	database, err := NewDB(dbPath, logger)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Create schema_version table
	_, err = database.ExecContext(context.Background(), `
		DROP TABLE IF EXISTS schema_version;
		CREATE TABLE schema_version (version INTEGER NOT NULL) STRICT;
		INSERT INTO schema_version (version) VALUES (?);
	`, version)
	if err != nil {
		t.Fatalf("Failed to create schema_version table: %v", err)
	}

	return database
}

func TestCheckSchemaVersion(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	ctx := context.Background()

	t.Run("matching version passes", func(t *testing.T) {
		db := testDBWithVersion(t, SchemaVersion)
		defer func() { _ = db.Close() }()

		err := CheckSchemaVersion(ctx, db, logger)
		if err != nil {
			t.Errorf("expected no error for matching version, got: %v", err)
		}
	})

	t.Run("older database version fails", func(t *testing.T) {
		oldVersion := SchemaVersion - 1
		if oldVersion < 0 {
			t.Skip("SchemaVersion is 0, cannot test older version")
		}
		db := testDBWithVersion(t, oldVersion)
		defer func() { _ = db.Close() }()

		err := CheckSchemaVersion(ctx, db, logger)
		if err == nil {
			t.Error("expected error for older database version, got nil")
		}
		if !strings.Contains(err.Error(), "schema version mismatch") {
			t.Errorf("expected error to mention 'schema version mismatch', got: %v", err)
		}
	})

	t.Run("newer database version passes with warning", func(t *testing.T) {
		newerVersion := SchemaVersion + 1
		db := testDBWithVersion(t, newerVersion)
		defer func() { _ = db.Close() }()

		// Should not error - newer database versions are allowed for rollbacks
		err := CheckSchemaVersion(ctx, db, logger)
		if err != nil {
			t.Errorf("expected no error for newer database version (rollback scenario), got: %v", err)
		}
	})
}

func TestUpdateSchemaVersion(t *testing.T) {
	ctx := context.Background()
	db := testDBWithVersion(t, 1)
	defer func() { _ = db.Close() }()

	// Update to a new version
	newVersion := 99
	err := UpdateSchemaVersion(ctx, db, newVersion)
	if err != nil {
		t.Fatalf("UpdateSchemaVersion failed: %v", err)
	}

	// Verify the version was updated
	var version int
	row := db.QueryRowContext(ctx, "SELECT version FROM schema_version")
	if err := row.Scan(&version); err != nil {
		t.Fatalf("failed to read version: %v", err)
	}

	if version != newVersion {
		t.Errorf("expected version %d, got %d", newVersion, version)
	}
}
