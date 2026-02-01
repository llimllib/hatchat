// Package rest provides REST API handlers for the chat application.
// These endpoints complement the WebSocket API for scenarios where
// REST is more appropriate (e.g., external integrations, simple queries).
package rest

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/llimllib/hatchat/server/db"
	"github.com/llimllib/hatchat/server/middleware"
	"github.com/llimllib/hatchat/server/models"
)

// API provides REST API handlers
type API struct {
	db     *db.DB
	logger *slog.Logger
}

// NewAPI creates a new REST API handler
func NewAPI(db *db.DB, logger *slog.Logger) *API {
	return &API{
		db:     db,
		logger: logger,
	}
}

// Response types for REST API

// UserResponse represents a user in API responses (excludes sensitive fields)
type UserResponse struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	Avatar    string `json:"avatar,omitempty"`
	CreatedAt string `json:"created_at"`
}

// RoomResponse represents a room in API responses
type RoomResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	IsPrivate bool   `json:"is_private"`
	IsDefault bool   `json:"is_default"`
	CreatedAt string `json:"created_at"`
}

// RoomListResponse is the response for listing rooms
type RoomListResponse struct {
	Rooms []RoomResponse `json:"rooms"`
}

// RoomDetailResponse includes room info and member details
type RoomDetailResponse struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	IsPrivate   bool             `json:"is_private"`
	IsDefault   bool             `json:"is_default"`
	CreatedAt   string           `json:"created_at"`
	MemberCount int              `json:"member_count"`
	Members     []MemberResponse `json:"members"`
}

// MemberResponse represents a room member
type MemberResponse struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Avatar   string `json:"avatar,omitempty"`
}

// CreateRoomRequest is the request body for creating a room
type CreateRoomRequest struct {
	Name      string `json:"name"`
	IsPrivate bool   `json:"is_private"`
}

// ErrorResponse is returned when an error occurs
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

// Helper functions

func (a *API) writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		a.logger.Error("failed to encode JSON response", "error", err)
	}
}

func (a *API) writeError(w http.ResponseWriter, status int, errType, message string) {
	a.writeJSON(w, status, ErrorResponse{
		Error:   errType,
		Message: message,
	})
}

func (a *API) getUser(r *http.Request) (*models.User, error) {
	userID := middleware.GetUserID(r.Context())
	return models.UserByID(r.Context(), a.db, userID)
}

// Handlers

// GetMe returns the current user's profile
// GET /api/v1/me
func (a *API) GetMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		a.writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET required")
		return
	}

	user, err := a.getUser(r)
	if err != nil {
		a.logger.Error("failed to get user", "error", err)
		a.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get user")
		return
	}

	a.writeJSON(w, http.StatusOK, UserResponse{
		ID:        user.ID,
		Username:  user.Username,
		Avatar:    user.Avatar.String,
		CreatedAt: user.CreatedAt,
	})
}

// GetMyRooms returns the rooms the current user is a member of
// GET /api/v1/me/rooms
func (a *API) GetMyRooms(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		a.writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET required")
		return
	}

	userID := middleware.GetUserID(r.Context())
	ctx := r.Context()

	rooms, err := models.UserRoomDetailsByUserID(ctx, a.db, userID)
	if err != nil {
		a.logger.Error("failed to get user rooms", "error", err)
		a.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get rooms")
		return
	}

	response := RoomListResponse{
		Rooms: make([]RoomResponse, len(rooms)),
	}
	for i, room := range rooms {
		response.Rooms[i] = RoomResponse{
			ID:        room.ID,
			Name:      room.Name,
			IsPrivate: room.IsPrivate != 0,
			// UserRoomDetails doesn't include IsDefault and CreatedAt
			// These are lightweight room listings, not full details
		}
	}

	a.writeJSON(w, http.StatusOK, response)
}

// GetRooms returns all public rooms
// GET /api/v1/rooms
func (a *API) GetRooms(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		a.writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET required")
		return
	}

	ctx := r.Context()

	rooms, err := db.ListPublicRooms(ctx, a.db)
	if err != nil {
		a.logger.Error("failed to list rooms", "error", err)
		a.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list rooms")
		return
	}

	response := RoomListResponse{
		Rooms: make([]RoomResponse, len(rooms)),
	}
	for i, room := range rooms {
		response.Rooms[i] = RoomResponse{
			ID:        room.ID,
			Name:      room.Name,
			IsPrivate: room.IsPrivate != 0,
			IsDefault: room.IsDefault != 0,
			CreatedAt: room.CreatedAt,
		}
	}

	a.writeJSON(w, http.StatusOK, response)
}

// CreateRoom creates a new room
// POST /api/v1/rooms
func (a *API) CreateRoom(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		a.writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST required")
		return
	}

	var req CreateRoomRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		a.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
		return
	}

	// Validate room name
	name := strings.TrimSpace(req.Name)
	if name == "" {
		a.writeError(w, http.StatusBadRequest, "validation_error", "Room name is required")
		return
	}
	if len(name) > 80 {
		a.writeError(w, http.StatusBadRequest, "validation_error", "Room name must be 80 characters or less")
		return
	}

	userID := middleware.GetUserID(r.Context())
	ctx := r.Context()

	// Create the room
	isPrivate := models.FALSE
	if req.IsPrivate {
		isPrivate = models.TRUE
	}

	room := &models.Room{
		ID:        models.GenerateRoomID(),
		Name:      name,
		IsPrivate: isPrivate,
		IsDefault: models.FALSE,
		CreatedAt: time.Now().Format(time.RFC3339),
	}

	if err := room.Insert(ctx, a.db); err != nil {
		a.logger.Error("failed to create room", "error", err)
		a.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to create room")
		return
	}

	// Add the creator as a member
	membership := &models.RoomsMember{
		RoomID: room.ID,
		UserID: userID,
	}
	if err := membership.Insert(ctx, a.db); err != nil {
		a.logger.Error("failed to add creator to room", "error", err)
		// Try to clean up the room
		_ = room.Delete(ctx, a.db)
		a.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to create room")
		return
	}

	a.writeJSON(w, http.StatusCreated, RoomResponse{
		ID:        room.ID,
		Name:      room.Name,
		IsPrivate: room.IsPrivate != 0,
		IsDefault: room.IsDefault != 0,
		CreatedAt: room.CreatedAt,
	})
}

// GetRoom returns details about a specific room
// GET /api/v1/rooms/{id}
func (a *API) GetRoom(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		a.writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET required")
		return
	}

	// Extract room ID from path
	roomID := extractRoomID(r.URL.Path)
	if roomID == "" {
		a.writeError(w, http.StatusBadRequest, "invalid_request", "Room ID is required")
		return
	}

	userID := middleware.GetUserID(r.Context())
	ctx := r.Context()

	// Check if the room exists
	info, err := db.GetRoomInfo(ctx, a.db, roomID)
	if err != nil {
		a.logger.Debug("room not found", "room_id", roomID, "error", err)
		a.writeError(w, http.StatusNotFound, "not_found", "Room not found")
		return
	}

	// If the room is private, check if the user is a member
	if info.Room.IsPrivate != 0 {
		isMember, err := db.IsRoomMember(ctx, a.db, userID, roomID)
		if err != nil || !isMember {
			a.writeError(w, http.StatusForbidden, "forbidden", "You are not a member of this room")
			return
		}
	}

	members := make([]MemberResponse, len(info.Members))
	for i, m := range info.Members {
		members[i] = MemberResponse{
			ID:       m.ID,
			Username: m.Username,
			Avatar:   m.Avatar,
		}
	}

	a.writeJSON(w, http.StatusOK, RoomDetailResponse{
		ID:          info.Room.ID,
		Name:        info.Room.Name,
		IsPrivate:   info.Room.IsPrivate != 0,
		IsDefault:   info.Room.IsDefault != 0,
		CreatedAt:   info.Room.CreatedAt,
		MemberCount: info.MemberCount,
		Members:     members,
	})
}

// JoinRoom adds the current user to a room
// POST /api/v1/rooms/{id}/join
func (a *API) JoinRoom(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		a.writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST required")
		return
	}

	// Extract room ID from path
	roomID := extractRoomIDWithSuffix(r.URL.Path, "/join")
	if roomID == "" {
		a.writeError(w, http.StatusBadRequest, "invalid_request", "Room ID is required")
		return
	}

	userID := middleware.GetUserID(r.Context())
	ctx := r.Context()

	// Check if the room exists
	room, err := models.RoomByID(ctx, a.db, roomID)
	if err != nil {
		a.logger.Debug("room not found", "room_id", roomID, "error", err)
		a.writeError(w, http.StatusNotFound, "not_found", "Room not found")
		return
	}

	// Can't join private rooms via this endpoint
	if room.IsPrivate != 0 {
		a.writeError(w, http.StatusForbidden, "forbidden", "Cannot join private rooms")
		return
	}

	// Check if already a member
	isMember, err := db.IsRoomMember(ctx, a.db, userID, roomID)
	if err != nil {
		a.logger.Error("failed to check membership", "error", err)
		a.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to join room")
		return
	}

	if isMember {
		// Already a member, just return success
		a.writeJSON(w, http.StatusOK, RoomResponse{
			ID:        room.ID,
			Name:      room.Name,
			IsPrivate: room.IsPrivate != 0,
			IsDefault: room.IsDefault != 0,
			CreatedAt: room.CreatedAt,
		})
		return
	}

	// Add user to room
	if _, err := db.AddRoomMember(ctx, a.db, userID, roomID); err != nil {
		a.logger.Error("failed to join room", "error", err)
		a.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to join room")
		return
	}

	a.writeJSON(w, http.StatusOK, RoomResponse{
		ID:        room.ID,
		Name:      room.Name,
		IsPrivate: room.IsPrivate != 0,
		IsDefault: room.IsDefault != 0,
		CreatedAt: room.CreatedAt,
	})
}

// LeaveRoom removes the current user from a room
// POST /api/v1/rooms/{id}/leave
func (a *API) LeaveRoom(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		a.writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST required")
		return
	}

	// Extract room ID from path
	roomID := extractRoomIDWithSuffix(r.URL.Path, "/leave")
	if roomID == "" {
		a.writeError(w, http.StatusBadRequest, "invalid_request", "Room ID is required")
		return
	}

	userID := middleware.GetUserID(r.Context())
	ctx := r.Context()

	// Check if this is the default room
	room, err := models.RoomByID(ctx, a.db, roomID)
	if err != nil {
		a.logger.Debug("room not found", "room_id", roomID, "error", err)
		a.writeError(w, http.StatusNotFound, "not_found", "Room not found")
		return
	}

	if room.IsDefault != 0 {
		a.writeError(w, http.StatusBadRequest, "invalid_request", "Cannot leave the default room")
		return
	}

	// Leave the room
	_, err = db.LeaveRoom(ctx, a.db, userID, roomID)
	if err != nil {
		a.logger.Error("failed to leave room", "error", err)
		a.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to leave room")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Helper to extract room ID from paths like /api/v1/rooms/{id}
func extractRoomID(path string) string {
	parts := strings.Split(strings.TrimSuffix(path, "/"), "/")
	if len(parts) < 4 {
		return ""
	}
	// Path is /api/v1/rooms/{id}
	return parts[len(parts)-1]
}

// Helper to extract room ID from paths like /api/v1/rooms/{id}/action
func extractRoomIDWithSuffix(path string, suffix string) string {
	path = strings.TrimSuffix(path, suffix)
	return extractRoomID(path)
}

// RoomsHandler handles all /api/v1/rooms/* requests
func (a *API) RoomsHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// /api/v1/rooms - list or create
	if path == "/api/v1/rooms" || path == "/api/v1/rooms/" {
		switch r.Method {
		case http.MethodGet:
			a.GetRooms(w, r)
		case http.MethodPost:
			a.CreateRoom(w, r)
		default:
			a.writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET or POST required")
		}
		return
	}

	// /api/v1/rooms/{id}/join
	if strings.HasSuffix(path, "/join") {
		a.JoinRoom(w, r)
		return
	}

	// /api/v1/rooms/{id}/leave
	if strings.HasSuffix(path, "/leave") {
		a.LeaveRoom(w, r)
		return
	}

	// /api/v1/rooms/{id}
	a.GetRoom(w, r)
}

// MeHandler handles all /api/v1/me/* requests
func (a *API) MeHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// /api/v1/me/rooms
	if strings.HasSuffix(path, "/rooms") || strings.HasSuffix(path, "/rooms/") {
		a.GetMyRooms(w, r)
		return
	}

	// /api/v1/me
	a.GetMe(w, r)
}

// GetUser returns a specific user's public profile
// GET /api/v1/users/{id}
func (a *API) GetUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		a.writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET required")
		return
	}

	// Extract user ID from path
	parts := strings.Split(strings.TrimSuffix(r.URL.Path, "/"), "/")
	if len(parts) < 4 {
		a.writeError(w, http.StatusBadRequest, "invalid_request", "User ID is required")
		return
	}
	userID := parts[len(parts)-1]

	ctx := context.Background()

	user, err := models.UserByID(ctx, a.db, userID)
	if err != nil {
		a.logger.Debug("user not found", "user_id", userID, "error", err)
		a.writeError(w, http.StatusNotFound, "not_found", "User not found")
		return
	}

	a.writeJSON(w, http.StatusOK, UserResponse{
		ID:        user.ID,
		Username:  user.Username,
		Avatar:    user.Avatar.String,
		CreatedAt: user.CreatedAt,
	})
}
