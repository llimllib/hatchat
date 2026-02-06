package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/llimllib/hatchat/server/db"
	"github.com/llimllib/hatchat/server/models"
	"github.com/llimllib/hatchat/server/protocol"
)

// setupSearchTestDB creates a test database with FTS5 support
func setupSearchTestDB(t *testing.T) *db.DB {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	testDB, err := db.NewDB("file::memory:?cache=shared", logger)
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

func TestSearch_EmptyQuery(t *testing.T) {
	testDB := setupSearchTestDB(t)
	defer testDB.Close()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(testDB, logger)

	user := &models.User{ID: "usr_test123456789a"}

	// Empty query should return error
	req := protocol.SearchRequest{Query: ""}
	reqData, _ := json.Marshal(req)

	resp, err := api.Search(user, reqData)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Type != "error" {
		t.Errorf("expected error response, got %s", resp.Type)
	}
}

func TestSearch_ReturnsMatchingMessages(t *testing.T) {
	testDB := setupSearchTestDB(t)
	defer testDB.Close()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(testDB, logger)

	// Create test user
	user := &models.User{
		ID:         "usr_test123456789a",
		Username:   "alice",
		Password:   "hash",
		LastRoom:   "roo_test123456",
		CreatedAt:  time.Now().Format(time.RFC3339),
		ModifiedAt: time.Now().Format(time.RFC3339),
	}
	err := user.Insert(ctx, testDB)
	if err != nil {
		t.Fatalf("failed to insert user: %v", err)
	}

	// Create test room
	room := &models.Room{
		ID:        "roo_test123456",
		Name:      "general",
		RoomType:  "channel",
		IsPrivate: 0,
		IsDefault: 1,
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	err = room.Insert(ctx, testDB)
	if err != nil {
		t.Fatalf("failed to insert room: %v", err)
	}

	// Add user to room
	_, err = testDB.ExecContext(ctx, "INSERT INTO rooms_members (user_id, room_id) VALUES ($1, $2)", user.ID, room.ID)
	if err != nil {
		t.Fatalf("failed to add user to room: %v", err)
	}

	// Create messages
	messages := []struct {
		id   string
		body string
	}{
		{"msg_test1234567", "Hello world, this is a test message"},
		{"msg_test2345678", "Another message about testing"},
		{"msg_test3456789", "Something completely different"},
	}

	for _, m := range messages {
		msg := &models.Message{
			ID:         m.id,
			RoomID:     room.ID,
			UserID:     user.ID,
			Body:       m.body,
			CreatedAt:  time.Now().Format(time.RFC3339Nano),
			ModifiedAt: time.Now().Format(time.RFC3339Nano),
		}
		err = msg.Insert(ctx, testDB)
		if err != nil {
			t.Fatalf("failed to insert message: %v", err)
		}
	}

	// Search for "test"
	req := protocol.SearchRequest{Query: "test"}
	reqData, _ := json.Marshal(req)

	resp, err := api.Search(user, reqData)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Type != "search" {
		t.Fatalf("expected search response, got %s", resp.Type)
	}

	searchResp, ok := resp.Data.(protocol.SearchResponse)
	if !ok {
		t.Fatalf("expected SearchResponse, got %T", resp.Data)
	}

	// Should find 2 messages containing "test"
	if len(searchResp.Results) != 2 {
		t.Errorf("expected 2 results, got %d", len(searchResp.Results))
	}
}

func TestSearch_RespectsRoomMembership(t *testing.T) {
	testDB := setupSearchTestDB(t)
	defer testDB.Close()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(testDB, logger)

	// Create two users
	alice := &models.User{
		ID:         "usr_alice12345678",
		Username:   "alice",
		Password:   "hash",
		LastRoom:   "roo_general1234",
		CreatedAt:  time.Now().Format(time.RFC3339),
		ModifiedAt: time.Now().Format(time.RFC3339),
	}
	bob := &models.User{
		ID:         "usr_bob1234567890",
		Username:   "bob",
		Password:   "hash",
		LastRoom:   "roo_private1234",
		CreatedAt:  time.Now().Format(time.RFC3339),
		ModifiedAt: time.Now().Format(time.RFC3339),
	}
	_ = alice.Insert(ctx, testDB)
	_ = bob.Insert(ctx, testDB)

	// Create two rooms
	publicRoom := &models.Room{
		ID:        "roo_general1234",
		Name:      "general",
		RoomType:  "channel",
		IsPrivate: 0,
		IsDefault: 1,
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	privateRoom := &models.Room{
		ID:        "roo_private1234",
		Name:      "private",
		RoomType:  "channel",
		IsPrivate: 1,
		IsDefault: 0,
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	_ = publicRoom.Insert(ctx, testDB)
	_ = privateRoom.Insert(ctx, testDB)

	// Alice is member of public room only
	_, _ = testDB.ExecContext(ctx, "INSERT INTO rooms_members (user_id, room_id) VALUES ($1, $2)", alice.ID, publicRoom.ID)
	// Bob is member of both rooms
	_, _ = testDB.ExecContext(ctx, "INSERT INTO rooms_members (user_id, room_id) VALUES ($1, $2)", bob.ID, publicRoom.ID)
	_, _ = testDB.ExecContext(ctx, "INSERT INTO rooms_members (user_id, room_id) VALUES ($1, $2)", bob.ID, privateRoom.ID)

	// Create messages in both rooms with "secret" keyword
	publicMsg := &models.Message{
		ID:         "msg_public12345",
		RoomID:     publicRoom.ID,
		UserID:     bob.ID,
		Body:       "This is a secret message in public",
		CreatedAt:  time.Now().Format(time.RFC3339Nano),
		ModifiedAt: time.Now().Format(time.RFC3339Nano),
	}
	privateMsg := &models.Message{
		ID:         "msg_private1234",
		RoomID:     privateRoom.ID,
		UserID:     bob.ID,
		Body:       "This is a secret message in private",
		CreatedAt:  time.Now().Format(time.RFC3339Nano),
		ModifiedAt: time.Now().Format(time.RFC3339Nano),
	}
	_ = publicMsg.Insert(ctx, testDB)
	_ = privateMsg.Insert(ctx, testDB)

	// Alice searches for "secret" - should only see public room message
	req := protocol.SearchRequest{Query: "secret"}
	reqData, _ := json.Marshal(req)

	resp, _ := api.Search(alice, reqData)
	searchResp := resp.Data.(protocol.SearchResponse)

	if len(searchResp.Results) != 1 {
		t.Errorf("alice should see 1 result, got %d", len(searchResp.Results))
	}
	if len(searchResp.Results) > 0 && searchResp.Results[0].RoomID != publicRoom.ID {
		t.Errorf("alice should only see public room message")
	}

	// Bob searches for "secret" - should see both messages
	resp, _ = api.Search(bob, reqData)
	searchResp = resp.Data.(protocol.SearchResponse)

	if len(searchResp.Results) != 2 {
		t.Errorf("bob should see 2 results, got %d", len(searchResp.Results))
	}
}

func TestSearch_RoomFilter(t *testing.T) {
	testDB := setupSearchTestDB(t)
	defer testDB.Close()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(testDB, logger)

	// Create user and two rooms
	user := &models.User{
		ID:         "usr_test123456789a",
		Username:   "alice",
		Password:   "hash",
		LastRoom:   "roo_general1234",
		CreatedAt:  time.Now().Format(time.RFC3339),
		ModifiedAt: time.Now().Format(time.RFC3339),
	}
	_ = user.Insert(ctx, testDB)

	room1 := &models.Room{
		ID:        "roo_general1234",
		Name:      "general",
		RoomType:  "channel",
		IsPrivate: 0,
		IsDefault: 1,
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	room2 := &models.Room{
		ID:        "roo_random12345",
		Name:      "random",
		RoomType:  "channel",
		IsPrivate: 0,
		IsDefault: 0,
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	_ = room1.Insert(ctx, testDB)
	_ = room2.Insert(ctx, testDB)

	_, _ = testDB.ExecContext(ctx, "INSERT INTO rooms_members (user_id, room_id) VALUES ($1, $2)", user.ID, room1.ID)
	_, _ = testDB.ExecContext(ctx, "INSERT INTO rooms_members (user_id, room_id) VALUES ($1, $2)", user.ID, room2.ID)

	// Create messages in both rooms
	msg1 := &models.Message{
		ID:         "msg_general1234",
		RoomID:     room1.ID,
		UserID:     user.ID,
		Body:       "Hello from general",
		CreatedAt:  time.Now().Format(time.RFC3339Nano),
		ModifiedAt: time.Now().Format(time.RFC3339Nano),
	}
	msg2 := &models.Message{
		ID:         "msg_random12345",
		RoomID:     room2.ID,
		UserID:     user.ID,
		Body:       "Hello from random",
		CreatedAt:  time.Now().Format(time.RFC3339Nano),
		ModifiedAt: time.Now().Format(time.RFC3339Nano),
	}
	_ = msg1.Insert(ctx, testDB)
	_ = msg2.Insert(ctx, testDB)

	// Search with room filter
	req := protocol.SearchRequest{
		Query:  "Hello",
		RoomID: room1.ID,
	}
	reqData, _ := json.Marshal(req)

	resp, _ := api.Search(user, reqData)
	searchResp := resp.Data.(protocol.SearchResponse)

	if len(searchResp.Results) != 1 {
		t.Errorf("expected 1 result with room filter, got %d", len(searchResp.Results))
	}
	if len(searchResp.Results) > 0 && searchResp.Results[0].RoomID != room1.ID {
		t.Errorf("result should be from filtered room")
	}
}

func TestSearch_UserFilter(t *testing.T) {
	testDB := setupSearchTestDB(t)
	defer testDB.Close()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(testDB, logger)

	// Create two users
	alice := &models.User{
		ID:         "usr_alice12345678",
		Username:   "alice",
		Password:   "hash",
		LastRoom:   "roo_general1234",
		CreatedAt:  time.Now().Format(time.RFC3339),
		ModifiedAt: time.Now().Format(time.RFC3339),
	}
	bob := &models.User{
		ID:         "usr_bob1234567890",
		Username:   "bob",
		Password:   "hash",
		LastRoom:   "roo_general1234",
		CreatedAt:  time.Now().Format(time.RFC3339),
		ModifiedAt: time.Now().Format(time.RFC3339),
	}
	_ = alice.Insert(ctx, testDB)
	_ = bob.Insert(ctx, testDB)

	room := &models.Room{
		ID:        "roo_general1234",
		Name:      "general",
		RoomType:  "channel",
		IsPrivate: 0,
		IsDefault: 1,
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	_ = room.Insert(ctx, testDB)

	_, _ = testDB.ExecContext(ctx, "INSERT INTO rooms_members (user_id, room_id) VALUES ($1, $2)", alice.ID, room.ID)
	_, _ = testDB.ExecContext(ctx, "INSERT INTO rooms_members (user_id, room_id) VALUES ($1, $2)", bob.ID, room.ID)

	// Create messages from both users
	aliceMsg := &models.Message{
		ID:         "msg_alice123456",
		RoomID:     room.ID,
		UserID:     alice.ID,
		Body:       "Hello world from alice",
		CreatedAt:  time.Now().Format(time.RFC3339Nano),
		ModifiedAt: time.Now().Format(time.RFC3339Nano),
	}
	bobMsg := &models.Message{
		ID:         "msg_bob12345678",
		RoomID:     room.ID,
		UserID:     bob.ID,
		Body:       "Hello world from bob",
		CreatedAt:  time.Now().Format(time.RFC3339Nano),
		ModifiedAt: time.Now().Format(time.RFC3339Nano),
	}
	_ = aliceMsg.Insert(ctx, testDB)
	_ = bobMsg.Insert(ctx, testDB)

	// Search with user filter
	req := protocol.SearchRequest{
		Query:  "Hello",
		UserID: alice.ID,
	}
	reqData, _ := json.Marshal(req)

	resp, _ := api.Search(alice, reqData)
	searchResp := resp.Data.(protocol.SearchResponse)

	if len(searchResp.Results) != 1 {
		t.Errorf("expected 1 result with user filter, got %d", len(searchResp.Results))
	}
	if len(searchResp.Results) > 0 && searchResp.Results[0].UserID != alice.ID {
		t.Errorf("result should be from filtered user")
	}
}

func TestSearch_ExcludesDeletedMessages(t *testing.T) {
	testDB := setupSearchTestDB(t)
	defer testDB.Close()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(testDB, logger)

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

	// Create one normal and one deleted message
	normalMsg := &models.Message{
		ID:         "msg_normal12345",
		RoomID:     room.ID,
		UserID:     user.ID,
		Body:       "This is a unique searchterm",
		CreatedAt:  time.Now().Format(time.RFC3339Nano),
		ModifiedAt: time.Now().Format(time.RFC3339Nano),
	}
	_ = normalMsg.Insert(ctx, testDB)

	deletedMsg := &models.Message{
		ID:         "msg_deleted1234",
		RoomID:     room.ID,
		UserID:     user.ID,
		Body:       "This has the same unique searchterm but is deleted",
		CreatedAt:  time.Now().Format(time.RFC3339Nano),
		ModifiedAt: time.Now().Format(time.RFC3339Nano),
	}
	_ = deletedMsg.Insert(ctx, testDB)

	// Soft delete the second message
	_, _ = testDB.ExecContext(ctx, "UPDATE messages SET deleted_at = $1 WHERE id = $2",
		time.Now().Format(time.RFC3339), deletedMsg.ID)

	// Search for the unique term
	req := protocol.SearchRequest{Query: "searchterm"}
	reqData, _ := json.Marshal(req)

	resp, _ := api.Search(user, reqData)
	searchResp := resp.Data.(protocol.SearchResponse)

	// Should only find the non-deleted message
	if len(searchResp.Results) != 1 {
		t.Errorf("expected 1 result (deleted excluded), got %d", len(searchResp.Results))
	}
	if len(searchResp.Results) > 0 && searchResp.Results[0].MessageID != normalMsg.ID {
		t.Errorf("should find only non-deleted message")
	}
}

func TestSearch_Pagination(t *testing.T) {
	testDB := setupSearchTestDB(t)
	defer testDB.Close()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewApi(testDB, logger)

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

	// Create 5 messages
	for i := 0; i < 5; i++ {
		msg := &models.Message{
			ID:         "msg_pag" + string(rune('0'+i)) + "12345678",
			RoomID:     room.ID,
			UserID:     user.ID,
			Body:       "Pagination test message",
			CreatedAt:  time.Now().Add(time.Duration(i) * time.Second).Format(time.RFC3339Nano),
			ModifiedAt: time.Now().Format(time.RFC3339Nano),
		}
		_ = msg.Insert(ctx, testDB)
	}

	// First page with limit 2
	req := protocol.SearchRequest{Query: "Pagination", Limit: 2}
	reqData, _ := json.Marshal(req)

	resp, _ := api.Search(user, reqData)
	searchResp := resp.Data.(protocol.SearchResponse)

	if len(searchResp.Results) != 2 {
		t.Errorf("expected 2 results on first page, got %d", len(searchResp.Results))
	}
	if searchResp.NextCursor == "" {
		t.Errorf("expected next cursor for more results")
	}

	// Second page
	req.Cursor = searchResp.NextCursor
	reqData, _ = json.Marshal(req)

	resp, _ = api.Search(user, reqData)
	searchResp = resp.Data.(protocol.SearchResponse)

	if len(searchResp.Results) != 2 {
		t.Errorf("expected 2 results on second page, got %d", len(searchResp.Results))
	}

	// Third page (should have 1 remaining)
	req.Cursor = searchResp.NextCursor
	reqData, _ = json.Marshal(req)

	resp, _ = api.Search(user, reqData)
	searchResp = resp.Data.(protocol.SearchResponse)

	if len(searchResp.Results) != 1 {
		t.Errorf("expected 1 result on last page, got %d", len(searchResp.Results))
	}
	if searchResp.NextCursor != "" {
		t.Errorf("expected no next cursor on last page")
	}
}
