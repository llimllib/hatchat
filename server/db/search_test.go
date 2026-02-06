package db

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/llimllib/hatchat/server/models"
)

// setupSearchTestDB creates a test database with full schema including FTS5
func setupSearchTestDB(t *testing.T) *DB {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	testDB, err := NewDB("file::memory:?cache=shared", logger)
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}

	// Clean up any existing tables
	dropSchema := `
		DROP TABLE IF EXISTS messages_fts;
		DROP TABLE IF EXISTS reactions;
		DROP TABLE IF EXISTS messages;
		DROP TABLE IF EXISTS rooms_members;
		DROP TABLE IF EXISTS sessions;
		DROP TABLE IF EXISTS rooms;
		DROP TABLE IF EXISTS users;
		DROP TRIGGER IF EXISTS messages_fts_insert;
		DROP TRIGGER IF EXISTS messages_fts_update;
		DROP TRIGGER IF EXISTS messages_fts_delete;
	`
	_, err = testDB.ExecContext(context.Background(), dropSchema)
	if err != nil {
		t.Fatalf("failed to drop existing tables: %v", err)
	}

	if err := testDB.RunSQLFile("../../schema.sql"); err != nil {
		t.Fatalf("failed to run schema: %v", err)
	}
	return testDB
}

func TestSearchMessages_Basic(t *testing.T) {
	testDB := setupSearchTestDB(t)
	defer testDB.Close()

	ctx := context.Background()

	// Create user
	user := &models.User{
		ID:         "usr_test123456789a",
		Username:   "alice",
		Password:   "hash",
		LastRoom:   "roo_general1234",
		CreatedAt:  time.Now().Format(time.RFC3339),
		ModifiedAt: time.Now().Format(time.RFC3339),
	}
	_ = user.Insert(ctx, testDB)

	// Create room
	room := &models.Room{
		ID:        "roo_general1234",
		Name:      "general",
		RoomType:  "channel",
		IsPrivate: 0,
		IsDefault: 1,
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	_ = room.Insert(ctx, testDB)

	// Add user to room
	_, _ = testDB.ExecContext(ctx, "INSERT INTO rooms_members (user_id, room_id) VALUES ($1, $2)", user.ID, room.ID)

	// Create messages
	msg1 := &models.Message{
		ID:         "msg_test12345678",
		RoomID:     room.ID,
		UserID:     user.ID,
		Body:       "Hello world",
		CreatedAt:  time.Now().Format(time.RFC3339Nano),
		ModifiedAt: time.Now().Format(time.RFC3339Nano),
	}
	msg2 := &models.Message{
		ID:         "msg_test23456789",
		RoomID:     room.ID,
		UserID:     user.ID,
		Body:       "Goodbye world",
		CreatedAt:  time.Now().Add(time.Second).Format(time.RFC3339Nano),
		ModifiedAt: time.Now().Format(time.RFC3339Nano),
	}
	_ = msg1.Insert(ctx, testDB)
	_ = msg2.Insert(ctx, testDB)

	// Search for "world"
	results, nextCursor, err := testDB.SearchMessages(ctx, user.ID, "world", "", "", "", 20)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}

	if nextCursor != "" {
		t.Errorf("expected no next cursor, got %s", nextCursor)
	}

	// Check snippets contain highlighted term
	for _, r := range results {
		if r.Snippet == "" {
			t.Errorf("expected non-empty snippet")
		}
	}
}

func TestSearchMessages_FTS5Escaping(t *testing.T) {
	testDB := setupSearchTestDB(t)
	defer testDB.Close()

	ctx := context.Background()

	user := &models.User{
		ID:         "usr_test123456789a",
		Username:   "alice",
		Password:   "hash",
		LastRoom:   "roo_general1234",
		CreatedAt:  time.Now().Format(time.RFC3339),
		ModifiedAt: time.Now().Format(time.RFC3339),
	}
	_ = user.Insert(ctx, testDB)

	room := &models.Room{
		ID:        "roo_general1234",
		Name:      "general",
		RoomType:  "channel",
		IsPrivate: 0,
		IsDefault: 1,
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	_ = room.Insert(ctx, testDB)

	_, _ = testDB.ExecContext(ctx, "INSERT INTO rooms_members (user_id, room_id) VALUES ($1, $2)", user.ID, room.ID)

	msg := &models.Message{
		ID:         "msg_test12345678",
		RoomID:     room.ID,
		UserID:     user.ID,
		Body:       "Testing with special chars: AND OR NOT",
		CreatedAt:  time.Now().Format(time.RFC3339Nano),
		ModifiedAt: time.Now().Format(time.RFC3339Nano),
	}
	_ = msg.Insert(ctx, testDB)

	// Search with FTS5 operators as literal text - should not cause errors
	results, _, err := testDB.SearchMessages(ctx, user.ID, "AND OR", "", "", "", 20)
	if err != nil {
		t.Fatalf("search with operators should not fail: %v", err)
	}

	// The search should find the message (both AND and OR are in the body)
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestEscapeFTS5Query(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello", `"hello"*`},
		{"hello world", `"hello"* "world"*`},
		{"AND OR NOT", `"AND"* "OR"* "NOT"*`},
		{`test"quote`, `"test""quote"*`},
		{"", `""`},
		{"  multiple   spaces  ", `"multiple"* "spaces"*`},
	}

	for _, tc := range tests {
		result := escapeFTS5Query(tc.input)
		if result != tc.expected {
			t.Errorf("escapeFTS5Query(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}
