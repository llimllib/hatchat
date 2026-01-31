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
	Type string `json:"type" jsonschema:"description=Message type identifier"`
	Data any    `json:"data" jsonschema:"description=Type-specific payload"`
}

// =============================================================================
// Shared Types (used in multiple messages)
// =============================================================================

// User represents a user in the system
type User struct {
	ID       string `json:"id" jsonschema:"description=Unique user identifier (usr_ prefix),pattern=^usr_[a-f0-9]{16}$"`
	Username string `json:"username" jsonschema:"description=Display name"`
	Avatar   string `json:"avatar" jsonschema:"description=Avatar URL (may be empty)"`
}

// Room represents a chat room
type Room struct {
	ID        string `json:"id" jsonschema:"description=Unique room identifier (roo_ prefix),pattern=^roo_[a-f0-9]{12}$"`
	Name      string `json:"name" jsonschema:"description=Room display name"`
	IsPrivate bool   `json:"is_private" jsonschema:"description=Whether the room is private"`
}

// Message represents a chat message
type Message struct {
	ID         string `json:"id" jsonschema:"description=Unique message identifier (msg_ prefix),pattern=^msg_[a-f0-9]{12}$"`
	RoomID     string `json:"room_id" jsonschema:"description=Room this message belongs to"`
	UserID     string `json:"user_id" jsonschema:"description=User who sent the message"`
	Username   string `json:"username" jsonschema:"description=Username of sender (denormalized for convenience)"`
	Body       string `json:"body" jsonschema:"description=Message content"`
	CreatedAt  string `json:"created_at" jsonschema:"description=RFC3339Nano timestamp of creation"`
	ModifiedAt string `json:"modified_at" jsonschema:"description=RFC3339Nano timestamp of last modification"`
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
	Body   string `json:"body" jsonschema:"description=Message content,minLength=1"`
	RoomID string `json:"room_id" jsonschema:"description=Target room ID,minLength=1"`
}

// HistoryRequest is sent by the client to fetch message history
// Direction: client → server
// Response: HistoryResponse
type HistoryRequest struct {
	RoomID string `json:"room_id" jsonschema:"description=Room to fetch history for,required"`
	Cursor string `json:"cursor" jsonschema:"description=Pagination cursor (created_at of oldest message seen)"`
	Limit  int    `json:"limit" jsonschema:"description=Maximum messages to return (default 50; max 100),minimum=1,maximum=100"`
}

// =============================================================================
// Server → Client Messages
// =============================================================================

// InitResponse is sent by the server in response to InitRequest
// Direction: server → client
type InitResponse struct {
	User        User    `json:"User" jsonschema:"description=The authenticated user"`
	Rooms       []*Room `json:"Rooms" jsonschema:"description=Rooms the user is a member of"`
	CurrentRoom string  `json:"current_room" jsonschema:"description=Room ID to display initially"`
}

// HistoryResponse is sent by the server in response to HistoryRequest
// Direction: server → client
type HistoryResponse struct {
	Messages   []*Message `json:"messages" jsonschema:"description=Messages in chronological order (newest first)"`
	HasMore    bool       `json:"has_more" jsonschema:"description=Whether older messages exist"`
	NextCursor string     `json:"next_cursor" jsonschema:"description=Pass as cursor to fetch older messages"`
}

// ErrorResponse is sent by the server when an error occurs
// Direction: server → client
type ErrorResponse struct {
	Message string `json:"Message" jsonschema:"description=Human-readable error message"`
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
}
