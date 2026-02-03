package db

import (
	"context"
	"testing"
	"time"

	"github.com/llimllib/hatchat/server/models"
)

func TestGetReactionsForMessages_Empty(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	result, err := GetReactionsForMessages(context.Background(), database, []string{})
	if err != nil {
		t.Fatalf("GetReactionsForMessages failed: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty map, got %d entries", len(result))
	}
}

func TestGetReactionsForMessages_Basic(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	ctx := context.Background()

	// Create test data
	user1 := createTestUser(t, database, "usr_rxn_basic_usr1", "alice")
	user2 := createTestUser(t, database, "usr_rxn_basic_usr2", "bob")
	room := createTestRoom(t, database, "roo_rxn_basic_01", "general", false)

	msg := createTestMessageForReactions(t, database, "msg_rxn_basic01", room.ID, user1.ID, "hello")

	// Add reactions
	now := time.Now().Format(time.RFC3339Nano)
	r1 := models.Reaction{MessageID: msg.ID, UserID: user1.ID, Emoji: "👍", CreatedAt: now}
	r2 := models.Reaction{MessageID: msg.ID, UserID: user2.ID, Emoji: "👍", CreatedAt: now}
	r3 := models.Reaction{MessageID: msg.ID, UserID: user1.ID, Emoji: "❤️", CreatedAt: now}

	if err := r1.Insert(ctx, database); err != nil {
		t.Fatalf("Failed to insert reaction: %v", err)
	}
	if err := r2.Insert(ctx, database); err != nil {
		t.Fatalf("Failed to insert reaction: %v", err)
	}
	if err := r3.Insert(ctx, database); err != nil {
		t.Fatalf("Failed to insert reaction: %v", err)
	}

	result, err := GetReactionsForMessages(ctx, database, []string{msg.ID})
	if err != nil {
		t.Fatalf("GetReactionsForMessages failed: %v", err)
	}

	reactions, ok := result[msg.ID]
	if !ok {
		t.Fatal("expected reactions for message, got none")
	}

	if len(reactions) != 2 {
		t.Fatalf("expected 2 reaction groups, got %d", len(reactions))
	}

	// Check thumbs up: should have count 2 with both users
	var thumbsUp, heart bool
	for _, r := range reactions {
		if r.Emoji == "👍" {
			thumbsUp = true
			if r.Count != 2 {
				t.Errorf("expected 👍 count 2, got %d", r.Count)
			}
			if len(r.UserIDs) != 2 {
				t.Errorf("expected 2 user IDs for 👍, got %d", len(r.UserIDs))
			}
		}
		if r.Emoji == "❤️" {
			heart = true
			if r.Count != 1 {
				t.Errorf("expected ❤️ count 1, got %d", r.Count)
			}
		}
	}
	if !thumbsUp {
		t.Error("expected 👍 reaction")
	}
	if !heart {
		t.Error("expected ❤️ reaction")
	}
}

func TestGetReactionsForMessages_MultipleMessages(t *testing.T) {
	database := testDB(t)
	defer func() { _ = database.Close() }()

	ctx := context.Background()

	user := createTestUser(t, database, "usr_rxn_multi_usr", "alice")
	room := createTestRoom(t, database, "roo_rxn_multi_01", "general", false)

	msg1 := createTestMessageForReactions(t, database, "msg_rxn_multi01", room.ID, user.ID, "first")
	msg2 := createTestMessageForReactions(t, database, "msg_rxn_multi02", room.ID, user.ID, "second")
	msg3 := createTestMessageForReactions(t, database, "msg_rxn_multi03", room.ID, user.ID, "third (no reactions)")

	now := time.Now().Format(time.RFC3339Nano)
	r1 := models.Reaction{MessageID: msg1.ID, UserID: user.ID, Emoji: "👍", CreatedAt: now}
	r2 := models.Reaction{MessageID: msg2.ID, UserID: user.ID, Emoji: "🎉", CreatedAt: now}
	if err := r1.Insert(ctx, database); err != nil {
		t.Fatalf("Failed to insert reaction: %v", err)
	}
	if err := r2.Insert(ctx, database); err != nil {
		t.Fatalf("Failed to insert reaction: %v", err)
	}

	result, err := GetReactionsForMessages(ctx, database, []string{msg1.ID, msg2.ID, msg3.ID})
	if err != nil {
		t.Fatalf("GetReactionsForMessages failed: %v", err)
	}

	if len(result[msg1.ID]) != 1 {
		t.Errorf("expected 1 reaction for msg1, got %d", len(result[msg1.ID]))
	}
	if len(result[msg2.ID]) != 1 {
		t.Errorf("expected 1 reaction for msg2, got %d", len(result[msg2.ID]))
	}
	if len(result[msg3.ID]) != 0 {
		t.Errorf("expected 0 reactions for msg3, got %d", len(result[msg3.ID]))
	}
}

// Note: createTestUser, createTestRoom helpers are in isroommember_test.go
// createTestMessageForReactions creates a test message (separate name to avoid conflict)
func createTestMessageForReactions(t *testing.T, database *DB, id, roomID, userID, body string) *models.Message {
	t.Helper()
	now := time.Now().Format(time.RFC3339Nano)
	msg := &models.Message{
		ID:         id,
		RoomID:     roomID,
		UserID:     userID,
		Body:       body,
		CreatedAt:  now,
		ModifiedAt: now,
	}
	err := msg.Insert(context.Background(), database)
	if err != nil {
		t.Fatalf("Failed to create test message: %v", err)
	}
	return msg
}
