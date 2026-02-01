package server

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/llimllib/hatchat/server/db"
	"github.com/llimllib/hatchat/server/models"
)

type seedUser struct {
	username string
	password string
}

var devUsers = []seedUser{
	{username: "alice", password: "alice"},
	{username: "bob", password: "bob"},
}

// seedDevUsers creates test users for development. Called when SEED_DEVELOPMENT_DB env var is set.
func seedDevUsers(database *db.DB, logger *slog.Logger) error {
	ctx := context.Background()

	room, err := models.GetDefaultRoom(ctx, database)
	if err != nil {
		return fmt.Errorf("get default room: %w", err)
	}

	now := time.Now().Format(time.RFC3339)

	for _, u := range devUsers {
		// Check if user already exists
		existing, err := models.UserByUsername(ctx, database, u.username)
		if err == nil && existing != nil {
			logger.Debug("dev user already exists, skipping", "username", u.username)
			continue
		}

		// Hash password
		encPass, err := bcrypt.GenerateFromPassword([]byte(u.password), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("hash password for %s: %w", u.username, err)
		}

		// Create user
		user := &models.User{
			ID:         models.GenerateUserID(),
			Username:   u.username,
			Password:   string(encPass),
			LastRoom:   room.ID,
			CreatedAt:  now,
			ModifiedAt: now,
		}

		if err := user.Insert(ctx, database); err != nil {
			return fmt.Errorf("insert user %s: %w", u.username, err)
		}

		// Add user to default room
		roomMember := &models.RoomsMember{
			UserID: user.ID,
			RoomID: room.ID,
		}
		if err := roomMember.Insert(ctx, database); err != nil {
			return fmt.Errorf("add user %s to default room: %w", u.username, err)
		}

		logger.Info("created dev user", "username", u.username, "password", u.password)
	}

	return nil
}
