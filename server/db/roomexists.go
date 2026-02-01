package db

import (
	"context"
)

// RoomExistsByName checks if a room with the given name already exists.
func RoomExistsByName(ctx context.Context, db *DB, name string) (bool, error) {
	const sqlstr = `SELECT EXISTS(` +
		`SELECT 1 FROM rooms ` +
		`WHERE name = $1` +
		`)`
	db.logger.Debug("querying", "query", sqlstr, "args", []any{name})
	var exists bool
	if err := db.QueryRowContext(ctx, sqlstr, name).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}
