package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/llimllib/hatchat/server/db"
	"github.com/llimllib/hatchat/server/middleware"
	"github.com/llimllib/hatchat/server/models"
)

// setupTestDB creates an in-memory database with the schema loaded
func setupTestDB(t *testing.T) *db.DB {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	testDB, err := db.NewDB("file::memory:?cache=shared", logger)
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}

	// Drop existing tables to ensure a clean slate (shared in-memory db)
	dropSchema := `
		DROP TABLE IF EXISTS messages;
		DROP TABLE IF EXISTS rooms_members;
		DROP TABLE IF EXISTS sessions;
		DROP TABLE IF EXISTS rooms;
		DROP TABLE IF EXISTS users;
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

// createTestUser creates a user and returns it
func createTestUser(t *testing.T, testDB *db.DB, username string) *models.User {
	t.Helper()
	user := &models.User{
		ID:         models.GenerateUserID(),
		Username:   username,
		Password:   "hashed_password",
		LastRoom:   "",
		CreatedAt:  time.Now().Format(time.RFC3339),
		ModifiedAt: time.Now().Format(time.RFC3339),
	}
	if err := user.Insert(context.Background(), testDB); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	return user
}

// createTestRoom creates a room and returns it
func createTestRoom(t *testing.T, testDB *db.DB, name string, isPrivate bool) *models.Room {
	t.Helper()
	priv := models.FALSE
	if isPrivate {
		priv = models.TRUE
	}
	room := &models.Room{
		ID:        models.GenerateRoomID(),
		Name:      name,
		IsPrivate: priv,
		IsDefault: models.FALSE,
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	if err := room.Insert(context.Background(), testDB); err != nil {
		t.Fatalf("failed to create room: %v", err)
	}
	return room
}

// addUserToRoom adds a user as a member of a room
func addUserToRoom(t *testing.T, testDB *db.DB, userID, roomID string) {
	t.Helper()
	member := &models.RoomsMember{
		UserID: userID,
		RoomID: roomID,
	}
	if err := member.Insert(context.Background(), testDB); err != nil {
		t.Fatalf("failed to add user to room: %v", err)
	}
}

// makeRequest creates a request with the user ID in context
func makeRequest(t *testing.T, method, path string, body any, userID string) *http.Request {
	t.Helper()
	var req *http.Request
	var err error

	if body != nil {
		bodyBytes, _ := json.Marshal(body)
		req, err = http.NewRequest(method, path, bytes.NewBuffer(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, err = http.NewRequest(method, path, nil)
	}
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	// Add user ID to context
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID)
	return req.WithContext(ctx)
}

func TestGetMe(t *testing.T) {
	testDB := setupTestDB(t)
	api := NewAPI(testDB, nil)
	user := createTestUser(t, testDB, "alice")

	req := makeRequest(t, http.MethodGet, "/api/v1/me", nil, user.ID)
	rr := httptest.NewRecorder()

	api.GetMe(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var response UserResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response.ID != user.ID {
		t.Errorf("expected user ID %s, got %s", user.ID, response.ID)
	}
	if response.Username != "alice" {
		t.Errorf("expected username alice, got %s", response.Username)
	}
}

func TestGetMyRooms(t *testing.T) {
	testDB := setupTestDB(t)
	api := NewAPI(testDB, nil)
	user := createTestUser(t, testDB, "alice")
	room := createTestRoom(t, testDB, "general", false)
	addUserToRoom(t, testDB, user.ID, room.ID)

	req := makeRequest(t, http.MethodGet, "/api/v1/me/rooms", nil, user.ID)
	rr := httptest.NewRecorder()

	api.GetMyRooms(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var response RoomListResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(response.Rooms) != 1 {
		t.Errorf("expected 1 room, got %d", len(response.Rooms))
	}
	if response.Rooms[0].Name != "general" {
		t.Errorf("expected room name general, got %s", response.Rooms[0].Name)
	}
}

func TestGetRooms(t *testing.T) {
	testDB := setupTestDB(t)
	api := NewAPI(testDB, nil)
	user := createTestUser(t, testDB, "alice")
	createTestRoom(t, testDB, "public-room", false)
	createTestRoom(t, testDB, "private-room", true)

	req := makeRequest(t, http.MethodGet, "/api/v1/rooms", nil, user.ID)
	rr := httptest.NewRecorder()

	api.GetRooms(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var response RoomListResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	// Should only return public rooms
	if len(response.Rooms) != 1 {
		t.Errorf("expected 1 public room, got %d", len(response.Rooms))
	}
	if len(response.Rooms) > 0 && response.Rooms[0].Name != "public-room" {
		t.Errorf("expected room name public-room, got %s", response.Rooms[0].Name)
	}
}

func TestCreateRoom(t *testing.T) {
	testDB := setupTestDB(t)
	api := NewAPI(testDB, nil)
	user := createTestUser(t, testDB, "alice")

	body := CreateRoomRequest{
		Name:      "new-room",
		IsPrivate: false,
	}

	req := makeRequest(t, http.MethodPost, "/api/v1/rooms", body, user.ID)
	rr := httptest.NewRecorder()

	api.CreateRoom(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var response RoomResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response.Name != "new-room" {
		t.Errorf("expected room name new-room, got %s", response.Name)
	}
	if response.IsPrivate {
		t.Error("expected room to be public")
	}

	// Verify user is a member
	isMember, err := db.IsRoomMember(context.Background(), testDB, user.ID, response.ID)
	if err != nil {
		t.Fatalf("failed to check membership: %v", err)
	}
	if !isMember {
		t.Error("expected user to be a member of the new room")
	}
}

func TestCreateRoomValidation(t *testing.T) {
	testDB := setupTestDB(t)
	api := NewAPI(testDB, nil)
	user := createTestUser(t, testDB, "alice")

	tests := []struct {
		name     string
		body     CreateRoomRequest
		wantCode int
	}{
		{
			name:     "empty name",
			body:     CreateRoomRequest{Name: ""},
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "whitespace name",
			body:     CreateRoomRequest{Name: "   "},
			wantCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := makeRequest(t, http.MethodPost, "/api/v1/rooms", tt.body, user.ID)
			rr := httptest.NewRecorder()

			api.CreateRoom(rr, req)

			if rr.Code != tt.wantCode {
				t.Errorf("expected status %d, got %d: %s", tt.wantCode, rr.Code, rr.Body.String())
			}
		})
	}
}

func TestGetRoom(t *testing.T) {
	testDB := setupTestDB(t)
	api := NewAPI(testDB, nil)
	user := createTestUser(t, testDB, "alice")
	room := createTestRoom(t, testDB, "general", false)
	addUserToRoom(t, testDB, user.ID, room.ID)

	req := makeRequest(t, http.MethodGet, "/api/v1/rooms/"+room.ID, nil, user.ID)
	rr := httptest.NewRecorder()

	api.GetRoom(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var response RoomDetailResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response.Name != "general" {
		t.Errorf("expected room name general, got %s", response.Name)
	}
	if response.MemberCount != 1 {
		t.Errorf("expected 1 member, got %d", response.MemberCount)
	}
}

func TestGetRoomPrivate(t *testing.T) {
	testDB := setupTestDB(t)
	api := NewAPI(testDB, nil)
	user := createTestUser(t, testDB, "alice")
	otherUser := createTestUser(t, testDB, "bob")
	room := createTestRoom(t, testDB, "secret", true)
	addUserToRoom(t, testDB, otherUser.ID, room.ID)

	// User is not a member, should get forbidden
	req := makeRequest(t, http.MethodGet, "/api/v1/rooms/"+room.ID, nil, user.ID)
	rr := httptest.NewRecorder()

	api.GetRoom(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestJoinRoom(t *testing.T) {
	testDB := setupTestDB(t)
	api := NewAPI(testDB, nil)
	user := createTestUser(t, testDB, "alice")
	room := createTestRoom(t, testDB, "general", false)

	req := makeRequest(t, http.MethodPost, "/api/v1/rooms/"+room.ID+"/join", nil, user.ID)
	rr := httptest.NewRecorder()

	api.JoinRoom(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify user is a member
	isMember, err := db.IsRoomMember(context.Background(), testDB, user.ID, room.ID)
	if err != nil {
		t.Fatalf("failed to check membership: %v", err)
	}
	if !isMember {
		t.Error("expected user to be a member after joining")
	}
}

func TestJoinRoomPrivate(t *testing.T) {
	testDB := setupTestDB(t)
	api := NewAPI(testDB, nil)
	user := createTestUser(t, testDB, "alice")
	room := createTestRoom(t, testDB, "secret", true)

	req := makeRequest(t, http.MethodPost, "/api/v1/rooms/"+room.ID+"/join", nil, user.ID)
	rr := httptest.NewRecorder()

	api.JoinRoom(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected status 403 for private room, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestLeaveRoom(t *testing.T) {
	testDB := setupTestDB(t)
	api := NewAPI(testDB, nil)
	user := createTestUser(t, testDB, "alice")
	room := createTestRoom(t, testDB, "general", false)
	addUserToRoom(t, testDB, user.ID, room.ID)

	req := makeRequest(t, http.MethodPost, "/api/v1/rooms/"+room.ID+"/leave", nil, user.ID)
	rr := httptest.NewRecorder()

	api.LeaveRoom(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify user is no longer a member
	isMember, err := db.IsRoomMember(context.Background(), testDB, user.ID, room.ID)
	if err != nil {
		t.Fatalf("failed to check membership: %v", err)
	}
	if isMember {
		t.Error("expected user to not be a member after leaving")
	}
}

func TestLeaveDefaultRoom(t *testing.T) {
	testDB := setupTestDB(t)
	api := NewAPI(testDB, nil)
	user := createTestUser(t, testDB, "alice")

	// Create a default room
	room := &models.Room{
		ID:        models.GenerateRoomID(),
		Name:      "main",
		IsPrivate: models.FALSE,
		IsDefault: models.TRUE,
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	if err := room.Insert(context.Background(), testDB); err != nil {
		t.Fatalf("failed to create default room: %v", err)
	}
	addUserToRoom(t, testDB, user.ID, room.ID)

	req := makeRequest(t, http.MethodPost, "/api/v1/rooms/"+room.ID+"/leave", nil, user.ID)
	rr := httptest.NewRecorder()

	api.LeaveRoom(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for default room, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestGetUser(t *testing.T) {
	testDB := setupTestDB(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewAPI(testDB, logger)
	alice := createTestUser(t, testDB, "alice")
	bob := createTestUser(t, testDB, "bob")

	req := makeRequest(t, http.MethodGet, "/api/v1/users/"+bob.ID, nil, alice.ID)
	rr := httptest.NewRecorder()

	api.GetUser(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var response UserResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response.ID != bob.ID {
		t.Errorf("expected user ID %s, got %s", bob.ID, response.ID)
	}
	if response.Username != "bob" {
		t.Errorf("expected username bob, got %s", response.Username)
	}
}

func TestGetUserNotFound(t *testing.T) {
	testDB := setupTestDB(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	api := NewAPI(testDB, logger)
	alice := createTestUser(t, testDB, "alice")

	req := makeRequest(t, http.MethodGet, "/api/v1/users/usr_nonexistent1234", nil, alice.ID)
	rr := httptest.NewRecorder()

	api.GetUser(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d: %s", rr.Code, rr.Body.String())
	}
}
