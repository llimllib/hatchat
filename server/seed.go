package server

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os"
	"regexp"
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

// extractDialogFromFile extracts dialog lines from a text file that uses
// Unicode curly quotes (U+201C " and U+201D ").
func extractDialogFromFile(filepath string) ([]string, error) {
	f, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer f.Close() //nolint: errcheck

	// Pattern matches text between curly quotes, 5-300 chars
	pattern := regexp.MustCompile(`\x{201c}([^\x{201d}]{5,300})\x{201d}`)

	var dialogs []string
	scanner := bufio.NewScanner(f)
	// Increase buffer size for long lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		matches := pattern.FindAllStringSubmatch(line, -1)
		for _, match := range matches {
			if len(match) > 1 {
				dialogs = append(dialogs, match[1])
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return dialogs, nil
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

const bronteePath = "docs/brontë/janeeyre.txt"

// seedDevMessages creates 1000 test messages spread over the past month
// using dialog from Charlotte Brontë's "Jane Eyre".
func seedDevMessages(database *db.DB, logger *slog.Logger) error {
	ctx := context.Background()

	// Get the default room
	room, err := models.GetDefaultRoom(ctx, database)
	if err != nil {
		return fmt.Errorf("get default room: %w", err)
	}

	// Get alice and bob users
	alice, err := models.UserByUsername(ctx, database, "alice")
	if err != nil {
		return fmt.Errorf("get alice: %w", err)
	}
	bob, err := models.UserByUsername(ctx, database, "bob")
	if err != nil {
		return fmt.Errorf("get bob: %w", err)
	}
	users := []*models.User{alice, bob}

	// Check if we already have messages (don't re-seed)
	existingMsgs, err := db.GetRoomMessages(ctx, database, room.ID, "", 1)
	if err != nil {
		return fmt.Errorf("check existing messages: %w", err)
	}
	if len(existingMsgs) > 0 {
		logger.Debug("messages already exist, skipping seed")
		return nil
	}

	// Extract dialog from Jane Eyre
	dialogs, err := extractDialogFromFile(bronteePath)
	if err != nil {
		return fmt.Errorf("extract dialog: %w", err)
	}

	if len(dialogs) < 1000 {
		return fmt.Errorf("not enough dialog lines: got %d, need 1000", len(dialogs))
	}

	// Create 1000 messages spread over the past month
	const numMessages = 1000
	now := time.Now()
	monthAgo := now.AddDate(0, -1, 0)
	duration := now.Sub(monthAgo)
	interval := duration / numMessages

	for i := 0; i < numMessages; i++ {
		// Alternate between alice and bob, with some variation
		user := users[i%2]

		// Calculate timestamp - spread evenly over the month
		msgTime := monthAgo.Add(time.Duration(i) * interval)
		timestamp := msgTime.Format(time.RFC3339Nano)

		msg := &models.Message{
			ID:         models.GenerateMessageID(),
			RoomID:     room.ID,
			UserID:     user.ID,
			Body:       dialogs[i],
			CreatedAt:  timestamp,
			ModifiedAt: timestamp,
		}

		if err := msg.Insert(ctx, database); err != nil {
			return fmt.Errorf("insert message %d: %w", i, err)
		}
	}

	logger.Info("seeded dev messages", "count", numMessages)
	return nil
}
