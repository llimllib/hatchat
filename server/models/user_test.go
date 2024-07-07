package models

import (
	"context"
	"testing"
)

func TestUser_Insert(t *testing.T) {
	db := setupDB()
	defer teardownDB(db)

	// Test case: Insert a new user
	user := &User{
		ID:       "user1",
		Username: "john",
		Password: "password123",
	}
	if err := user.Insert(db); err != nil {
		t.Errorf("User.Insert failed: %v", err)
	}

	// Check if the user was inserted correctly
	rows, err := db.QueryContext(context.Background(), "SELECT * FROM users WHERE id = ?", user.ID)
	if err != nil {
		t.Errorf("Failed to select user: %v", err)
	}
	defer rows.Close()

	if !rows.Next() {
		t.Error("Expected at least one row in the result set")
	}

	var retrievedUser User
	err = rows.Scan(&retrievedUser.ID, &retrievedUser.Username, &retrievedUser.Password, &retrievedUser.CreatedAt, &retrievedUser.ModifiedAt)
	if err != nil {
		t.Errorf("Failed to scan user: %v", err)
	}

	if retrievedUser.ID != user.ID || retrievedUser.Username != user.Username || retrievedUser.Password != user.Password {
		t.Errorf("Retrieved user data does not match inserted data: %+v", retrievedUser)
	}

	if retrievedUser.CreatedAt.IsZero() || retrievedUser.ModifiedAt.IsZero() {
		t.Error("CreatedAt and ModifiedAt should not be zero values")
	}
}

func TestGetUserByUsername(t *testing.T) {
	db := setupDB()
	defer teardownDB(db)

	// Insert a sample user
	iuser := &User{
		ID:       "user1",
		Username: "john",
		Password: "password123",
	}
	if err := iuser.Insert(db); err != nil {
		t.Errorf("User.Insert failed: %v", err)
	}

	// Test case: Retrieve an existing user
	user, err := GetUserByUsername(db, "john")
	if err != nil {
		t.Errorf("GetUserByUsername failed: %v", err)
	}

	if user.Username != "john" || user.Password != "password123" {
		t.Errorf("Retrieved user data is incorrect: %+v", user)
	}

	// Test case: Retrieve a non-existent user
	_, err = GetUserByUsername(db, "nonexistent")
	if err == nil {
		t.Error("Expected an error for non-existent user, but got nil")
	}
}
