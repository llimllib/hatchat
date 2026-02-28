package db

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
)

// SchemaVersion is the schema version that this server expects.
// Increment this when making schema changes.
//
// IMPORTANT: Schema changes must be backward-compatible. This means:
// - New columns must have DEFAULT values
// - Don't remove columns (mark as deprecated instead)
// - Don't rename columns
// - Don't change column types
//
// This allows older servers to run against newer schemas (for rollbacks).
const SchemaVersion = 1

// ErrSchemaVersionMismatch is returned when the database schema version
// is older than what the server requires.
var ErrSchemaVersionMismatch = errors.New("database schema version mismatch")

// CheckSchemaVersion verifies that the database schema version is compatible
// with this server. Returns an error if the database version is less than
// the expected version. Logs a warning if the database version is greater
// (which is allowed for rollback scenarios).
func CheckSchemaVersion(ctx context.Context, db *DB, logger *slog.Logger) error {
	var dbVersion int
	row := db.QueryRowContext(ctx, "SELECT version FROM schema_version LIMIT 1")
	if err := row.Scan(&dbVersion); err != nil {
		return fmt.Errorf("failed to read schema version: %w", err)
	}

	if dbVersion < SchemaVersion {
		return fmt.Errorf("%w: database is at version %d, server requires version %d; "+
			"please run migrations to update the database schema",
			ErrSchemaVersionMismatch, dbVersion, SchemaVersion)
	}

	if dbVersion > SchemaVersion {
		logger.Warn("database schema version is newer than server expects",
			"db_version", dbVersion,
			"server_version", SchemaVersion,
			"note", "This is OK for rollbacks since schema changes are backward-compatible")
	} else {
		logger.Debug("schema version check passed", "version", dbVersion)
	}

	return nil
}

// UpdateSchemaVersion sets the database schema version. This should be called
// after running migrations.
func UpdateSchemaVersion(ctx context.Context, db *DB, version int) error {
	_, err := db.ExecContext(ctx, "UPDATE schema_version SET version = ?", version)
	return err
}
