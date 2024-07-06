package models

import (
	"context"
	"fmt"
	"log/slog"
	"testing"

	"github.com/llimllib/tinychat/server/db"
)

func setupDB() *db.DB {
	dbPath := "file::memory:?cache=shared"
	db, err := db.NewDB(dbPath, slog.Default())
	if err != nil {
		panic(fmt.Sprintf("Failed to create database connection: %v", err))
	}

	err = db.RunSQLFile("../../schema.sql")
	if err != nil {
		panic(fmt.Sprintf("Failed to create database: %v", err))
	}

	return db
}

func teardownDB(db *db.DB) {
	db.Close()
}

func TestSession_Insert(t *testing.T) {
	db := setupDB()
	defer teardownDB(db)

	session := &Session{
		ID:       "session1",
		Username: "john",
	}
	err := session.Insert(db)
	if err != nil {
		t.Fatalf("Session.Insert failed: %v", err)
	}

	// Check if the session was inserted correctly
	rows, err := db.Select(context.Background(), "SELECT * FROM sessions WHERE id = ?", session.ID)
	if err != nil {
		t.Fatalf("Failed to select session: %v", err)
	}
	defer rows.Close()

	if !rows.Next() {
		t.Fatalf("Expected at least one row in the result set")
	}

	var retrievedSession Session
	err = rows.Scan(&retrievedSession.ID, &retrievedSession.Username, &retrievedSession.CreatedAt)
	if err != nil {
		t.Fatalf("Failed to scan session: %v", err)
	}

	if retrievedSession.ID != session.ID || retrievedSession.Username != session.Username {
		t.Fatalf("Retrieved session data does not match inserted data: %+v", retrievedSession)
	}

	if retrievedSession.CreatedAt.IsZero() {
		t.Fatalf("CreatedAt should not be a zero value")
	}
}

func TestGetSessionByUsername(t *testing.T) {
	db := setupDB()
	defer teardownDB(db)

	isession := &Session{
		ID:       "session2",
		Username: "john",
	}
	err := isession.Insert(db)
	if err != nil {
		t.Fatalf("Session.Insert failed: %v", err)
	}

	// Test case: Retrieve an existing session by username
	session, err := GetSessionByUsername(db, "john")
	if err != nil {
		t.Fatalf("GetSessionByUsername failed: %v", err)
	}

	if session.Username != "john" {
		t.Fatalf("Retrieved session data is incorrect: %+v", session)
	}

	// Test case: Retrieve a non-existent session by username
	_, err = GetSessionByUsername(db, "nonexistent")
	if err == nil {
		t.Fatal("Expected an error for non-existent session, but got nil")
	}
}

func TestGetSessionByID(t *testing.T) {
	db := setupDB()
	defer teardownDB(db)

	isession := &Session{
		ID:       "session3",
		Username: "john",
	}
	err := isession.Insert(db)
	if err != nil {
		t.Fatalf("Session.Insert failed: %v", err)
	}

	// Test case: Retrieve an existing session by ID
	session, err := GetSessionByID(db, "session3")
	if err != nil {
		t.Fatalf("GetSessionByID failed: %v", err)
	}

	if session.ID != "session3" || session.Username != "john" {
		t.Errorf("Retrieved session data is incorrect: %+v", session)
	}

	// Test case: Retrieve a non-existent session by ID
	_, err = GetSessionByID(db, "nonexistent")
	if err == nil {
		t.Fatalf("Expected an error for non-existent session, but got nil")
	}
}
