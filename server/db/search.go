package db

import (
	"context"
	"fmt"
	"strings"

	"github.com/llimllib/hatchat/server/protocol"
)

// SearchMessages performs a full-text search across messages the user has access to.
// Returns results with snippets showing matched text with ** highlighting.
func (db *DB) SearchMessages(
	ctx context.Context,
	userID string,
	query string,
	roomID string, // optional: filter to specific room
	filterUserID string, // optional: filter to specific user
	cursor string, // pagination cursor (offset as string)
	limit int,
) ([]protocol.SearchResult, string, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	// Parse cursor as offset
	offset := 0
	if cursor != "" {
		if _, err := fmt.Sscanf(cursor, "%d", &offset); err != nil {
			offset = 0
		}
	}

	// Build the query dynamically based on filters
	// FTS5 MATCH syntax: we need to escape the query for FTS5
	ftsQuery := escapeFTS5Query(query)

	args := []any{ftsQuery, userID}
	argIndex := 3

	// Base query with room membership check
	sql := `
		SELECT m.id, m.room_id, r.name, m.user_id, u.username,
		       snippet(messages_fts, 0, '**', '**', '...', 20) as snippet,
		       m.created_at
		FROM messages_fts
		JOIN messages m ON messages_fts.rowid = m.rowid
		JOIN rooms r ON m.room_id = r.id
		JOIN users u ON m.user_id = u.id
		WHERE messages_fts MATCH $1
		  AND m.deleted_at IS NULL
		  AND m.room_id IN (SELECT room_id FROM rooms_members WHERE user_id = $2)
	`

	// Add optional room filter
	if roomID != "" {
		sql += fmt.Sprintf(" AND m.room_id = $%d", argIndex)
		args = append(args, roomID)
		argIndex++
	}

	// Add optional user filter
	if filterUserID != "" {
		sql += fmt.Sprintf(" AND m.user_id = $%d", argIndex)
		args = append(args, filterUserID)
		argIndex++
	}

	// Order by recency and paginate
	sql += fmt.Sprintf(" ORDER BY m.created_at DESC LIMIT $%d OFFSET $%d", argIndex, argIndex+1)
	args = append(args, limit+1, offset) // Fetch one extra to check if there are more

	rows, err := db.QueryContext(ctx, sql, args...)
	if err != nil {
		return nil, "", fmt.Errorf("search query failed: %w", err)
	}
	defer rows.Close()

	var results []protocol.SearchResult
	for rows.Next() {
		var r protocol.SearchResult
		if err := rows.Scan(&r.MessageID, &r.RoomID, &r.RoomName, &r.UserID, &r.Username, &r.Snippet, &r.CreatedAt); err != nil {
			return nil, "", fmt.Errorf("scanning search result: %w", err)
		}
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("iterating search results: %w", err)
	}

	// Determine next cursor
	var nextCursor string
	if len(results) > limit {
		// There are more results
		results = results[:limit] // Trim to requested limit
		nextCursor = fmt.Sprintf("%d", offset+limit)
	}

	return results, nextCursor, nil
}

// escapeFTS5Query escapes a user query for safe use with FTS5 MATCH.
// FTS5 has special syntax for operators like AND, OR, NOT, NEAR, etc.
// We wrap each word in quotes to treat them as literal terms and add
// prefix matching (*) for a more intuitive search experience.
func escapeFTS5Query(query string) string {
	// Split on whitespace and quote each term
	words := strings.Fields(query)
	if len(words) == 0 {
		return `""`
	}

	// Quote each word to make it a literal phrase, add * for prefix matching
	// This prevents FTS5 operator injection while allowing partial word matches
	quoted := make([]string, len(words))
	for i, word := range words {
		// Escape any internal quotes
		escaped := strings.ReplaceAll(word, `"`, `""`)
		// Add * for prefix matching (e.g., "test" matches "testing")
		quoted[i] = `"` + escaped + `"*`
	}

	// Join with spaces - FTS5 will AND them together by default
	return strings.Join(quoted, " ")
}
