// Package protocol defines all WebSocket message types exchanged between
// client and server. This is the source of truth for the API contract.
//
// Run `just schema` to generate JSON Schema from these types.
// Run `just client-types` to regenerate TypeScript types from the schema.
package protocol

// Direction indicates whether a message is sent by client, server, or both
type Direction string

const (
	ClientToServer Direction = "client_to_server"
	ServerToClient Direction = "server_to_client"
	Bidirectional  Direction = "bidirectional"
)

// MessageMeta provides metadata about a message type for documentation
type MessageMeta struct {
	Type        string    // The "type" field value in the envelope
	Direction   Direction // Who sends this message
	Description string    // Human-readable description
}

// Envelope is the wrapper for all WebSocket messages
type Envelope struct {
	Type string `json:"type" jsonschema:"required,description=Message type identifier"`
	Data any    `json:"data" jsonschema:"required,description=Type-specific payload"`
}

// =============================================================================
// Shared Types (used in multiple messages)
// =============================================================================

// User represents a user in the system
type User struct {
	ID       string `json:"id" jsonschema:"required,description=Unique user identifier (usr_ prefix),pattern=^usr_[a-f0-9]{16}$"`
	Username string `json:"username" jsonschema:"required,description=Display name"`
	Avatar   string `json:"avatar" jsonschema:"description=Avatar URL (may be empty)"`
}

// Room represents a chat room
type Room struct {
	ID        string `json:"id" jsonschema:"required,description=Unique room identifier (roo_ prefix),pattern=^roo_[a-f0-9]{12}$"`
	Name      string `json:"name" jsonschema:"required,description=Room display name"`
	IsPrivate bool   `json:"is_private" jsonschema:"required,description=Whether the room is private"`
}

// Message represents a chat message
type Message struct {
	ID         string `json:"id" jsonschema:"required,description=Unique message identifier (msg_ prefix),pattern=^msg_[a-f0-9]{12}$"`
	RoomID     string `json:"room_id" jsonschema:"required,description=Room this message belongs to"`
	UserID     string `json:"user_id" jsonschema:"required,description=User who sent the message"`
	Username   string `json:"username" jsonschema:"required,description=Username of sender (denormalized for convenience)"`
	Body       string `json:"body" jsonschema:"required,description=Message content"`
	CreatedAt  string `json:"created_at" jsonschema:"required,description=RFC3339Nano timestamp of creation"`
	ModifiedAt string `json:"modified_at" jsonschema:"required,description=RFC3339Nano timestamp of last modification"`
}

// =============================================================================
// Client → Server Messages
// =============================================================================

// InitRequest is sent by the client to initialize the connection
// Direction: client → server
// Response: InitResponse
type InitRequest struct {
	// Currently empty, but reserved for future use (e.g., resume token)
}

// SendMessageRequest is sent by the client to post a new chat message
// Direction: client → server
// Response: Message (broadcast to room)
type SendMessageRequest struct {
	Body   string `json:"body" jsonschema:"required,description=Message content,minLength=1"`
	RoomID string `json:"room_id" jsonschema:"required,description=Target room ID,minLength=1"`
}

// HistoryRequest is sent by the client to fetch message history
// Direction: client → server
// Response: HistoryResponse
type HistoryRequest struct {
	RoomID string `json:"room_id" jsonschema:"required,description=Room to fetch history for"`
	Cursor string `json:"cursor" jsonschema:"description=Pagination cursor (created_at of oldest message seen)"`
	Limit  int    `json:"limit" jsonschema:"description=Maximum messages to return (default 50; max 100),minimum=1,maximum=100"`
}

// JoinRoomRequest is sent by the client to switch to a different room.
// If the user is not a member of a public room, they will be added as a member.
// Direction: client → server
// Response: JoinRoomResponse
type JoinRoomRequest struct {
	RoomID string `json:"room_id" jsonschema:"required,description=Room ID to switch to"`
}

// CreateRoomRequest is sent by the client to create a new room
// Direction: client → server
// Response: CreateRoomResponse
type CreateRoomRequest struct {
	Name      string `json:"name" jsonschema:"required,description=Room display name,minLength=1,maxLength=80"`
	IsPrivate bool   `json:"is_private" jsonschema:"description=Whether the room is private (invite-only)"`
}

// ListRoomsRequest is sent by the client to get a list of public rooms
// Direction: client → server
// Response: ListRoomsResponse
type ListRoomsRequest struct {
	Query string `json:"query" jsonschema:"description=Optional search query to filter rooms by name"`
}

// LeaveRoomRequest is sent by the client to leave a room
// Direction: client → server
// Response: LeaveRoomResponse
type LeaveRoomRequest struct {
	RoomID string `json:"room_id" jsonschema:"required,description=Room ID to leave"`
}

// RoomInfoRequest is sent by the client to get details about a room
// Direction: client → server
// Response: RoomInfoResponse
type RoomInfoRequest struct {
	RoomID string `json:"room_id" jsonschema:"required,description=Room ID to get info for"`
}

// =============================================================================
// Server → Client Messages
// =============================================================================

// InitResponse is sent by the server in response to InitRequest
// Direction: server → client
type InitResponse struct {
	User        User    `json:"User" jsonschema:"required,description=The authenticated user"`
	Rooms       []*Room `json:"Rooms" jsonschema:"required,description=Rooms the user is a member of"`
	CurrentRoom string  `json:"current_room" jsonschema:"required,description=Room ID to display initially"`
}

// HistoryResponse is sent by the server in response to HistoryRequest
// Direction: server → client
type HistoryResponse struct {
	Messages   []*Message `json:"messages" jsonschema:"required,description=Messages in chronological order (newest first)"`
	HasMore    bool       `json:"has_more" jsonschema:"required,description=Whether older messages exist"`
	NextCursor string     `json:"next_cursor" jsonschema:"required,description=Pass as cursor to fetch older messages"`
}

// ErrorResponse is sent by the server when an error occurs
// Direction: server → client
type ErrorResponse struct {
	Message string `json:"message" jsonschema:"required,description=Human-readable error message"`
}

// JoinRoomResponse is sent by the server in response to JoinRoomRequest
// Direction: server → client
type JoinRoomResponse struct {
	Room   Room `json:"room" jsonschema:"required,description=The room that was joined"`
	Joined bool `json:"joined" jsonschema:"required,description=True if user was added as a new member (vs already being a member)"`
}

// CreateRoomResponse is sent by the server in response to CreateRoomRequest
// Direction: server → client
type CreateRoomResponse struct {
	Room Room `json:"room" jsonschema:"required,description=The newly created room"`
}

// ListRoomsResponse is sent by the server in response to ListRoomsRequest
// Direction: server → client
type ListRoomsResponse struct {
	Rooms    []*Room `json:"rooms" jsonschema:"required,description=List of public rooms"`
	IsMember []bool  `json:"is_member" jsonschema:"required,description=Whether the user is a member of each room (parallel array)"`
}

// LeaveRoomResponse is sent by the server in response to LeaveRoomRequest
// Direction: server → client
type LeaveRoomResponse struct {
	RoomID string `json:"room_id" jsonschema:"required,description=Room ID that was left"`
}

// RoomMember represents a member of a room
type RoomMember struct {
	ID       string `json:"id" jsonschema:"required,description=User ID"`
	Username string `json:"username" jsonschema:"required,description=Username"`
	Avatar   string `json:"avatar" jsonschema:"description=Avatar URL (may be empty)"`
}

// RoomInfoResponse is sent by the server in response to RoomInfoRequest
// Direction: server → client
type RoomInfoResponse struct {
	Room        Room         `json:"room" jsonschema:"required,description=Room details"`
	MemberCount int          `json:"member_count" jsonschema:"required,description=Number of members in the room"`
	Members     []RoomMember `json:"members" jsonschema:"required,description=List of room members"`
	CreatedAt   string       `json:"created_at" jsonschema:"required,description=RFC3339 timestamp of when the room was created"`
}

// =============================================================================
// Message Registry - defines all message types and their metadata
// =============================================================================

// MessageTypes documents all supported message types
var MessageTypes = []MessageMeta{
	{
		Type:        "init",
		Direction:   ClientToServer,
		Description: "Initialize connection and get user/room data",
	},
	{
		Type:        "init",
		Direction:   ServerToClient,
		Description: "Response with user info, rooms, and current room",
	},
	{
		Type:        "message",
		Direction:   ClientToServer,
		Description: "Send a chat message to a room",
	},
	{
		Type:        "message",
		Direction:   ServerToClient,
		Description: "Broadcast a new message to room members",
	},
	{
		Type:        "history",
		Direction:   ClientToServer,
		Description: "Request message history for a room",
	},
	{
		Type:        "history",
		Direction:   ServerToClient,
		Description: "Response with paginated message history",
	},
	{
		Type:        "error",
		Direction:   ServerToClient,
		Description: "Error response when a request fails",
	},
	{
		Type:        "join_room",
		Direction:   ClientToServer,
		Description: "Switch to a different room (updates last_room). Joins public rooms if not a member.",
	},
	{
		Type:        "join_room",
		Direction:   ServerToClient,
		Description: "Response confirming room switch",
	},
	{
		Type:        "create_room",
		Direction:   ClientToServer,
		Description: "Create a new room",
	},
	{
		Type:        "create_room",
		Direction:   ServerToClient,
		Description: "Response with the newly created room",
	},
	{
		Type:        "list_rooms",
		Direction:   ClientToServer,
		Description: "Request list of public rooms",
	},
	{
		Type:        "list_rooms",
		Direction:   ServerToClient,
		Description: "Response with list of public rooms",
	},
	{
		Type:        "leave_room",
		Direction:   ClientToServer,
		Description: "Leave a room",
	},
	{
		Type:        "leave_room",
		Direction:   ServerToClient,
		Description: "Response confirming room leave",
	},
	{
		Type:        "room_info",
		Direction:   ClientToServer,
		Description: "Request information about a room",
	},
	{
		Type:        "room_info",
		Direction:   ServerToClient,
		Description: "Response with room details and members",
	},
}
