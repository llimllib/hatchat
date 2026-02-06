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
	ID          string `json:"id" jsonschema:"required,description=Unique user identifier (usr_ prefix),pattern=^usr_[a-f0-9]{16}$"`
	Username    string `json:"username" jsonschema:"required,description=Login username"`
	DisplayName string `json:"display_name" jsonschema:"description=Display name (shown instead of username if set)"`
	Status      string `json:"status" jsonschema:"description=Custom status message"`
	Avatar      string `json:"avatar" jsonschema:"description=Avatar URL (may be empty)"`
}

// Room represents a chat room or DM
type Room struct {
	ID        string       `json:"id" jsonschema:"required,description=Unique room identifier (roo_ prefix),pattern=^roo_[a-f0-9]{12}$"`
	Name      string       `json:"name" jsonschema:"required,description=Room display name (empty for DMs)"`
	RoomType  string       `json:"room_type" jsonschema:"required,description=Type of room: 'channel' or 'dm',enum=channel,enum=dm"`
	IsPrivate bool         `json:"is_private" jsonschema:"required,description=Whether the room is private"`
	Members   []RoomMember `json:"members,omitempty" jsonschema:"description=Room members (only populated for DMs)"`
}

// RoomMember represents a member of a room
type RoomMember struct {
	ID          string `json:"id" jsonschema:"required,description=User ID"`
	Username    string `json:"username" jsonschema:"required,description=Username"`
	DisplayName string `json:"display_name" jsonschema:"description=Display name (may be empty)"`
	Avatar      string `json:"avatar" jsonschema:"description=Avatar URL (may be empty)"`
}

// Message represents a chat message
type Message struct {
	ID         string     `json:"id" jsonschema:"required,description=Unique message identifier (msg_ prefix),pattern=^msg_[a-f0-9]{12}$"`
	RoomID     string     `json:"room_id" jsonschema:"required,description=Room this message belongs to"`
	UserID     string     `json:"user_id" jsonschema:"required,description=User who sent the message"`
	Username   string     `json:"username" jsonschema:"required,description=Username of sender (denormalized for convenience)"`
	Body       string     `json:"body" jsonschema:"required,description=Message content"`
	CreatedAt  string     `json:"created_at" jsonschema:"required,description=RFC3339Nano timestamp of creation"`
	ModifiedAt string     `json:"modified_at" jsonschema:"required,description=RFC3339Nano timestamp of last modification"`
	DeletedAt  string     `json:"deleted_at,omitempty" jsonschema:"description=RFC3339Nano timestamp of deletion (empty if not deleted)"`
	Reactions  []Reaction `json:"reactions,omitempty" jsonschema:"description=Aggregated emoji reactions on this message"`
}

// Reaction represents an aggregated emoji reaction on a message
type Reaction struct {
	Emoji   string   `json:"emoji" jsonschema:"required,description=The emoji character(s)"`
	Count   int      `json:"count" jsonschema:"required,description=Number of users who reacted with this emoji"`
	UserIDs []string `json:"user_ids" jsonschema:"required,description=IDs of users who reacted (for highlighting own reactions)"`
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

// CreateRoomRequest is sent by the client to create a new channel room
// Direction: client → server
// Response: CreateRoomResponse
type CreateRoomRequest struct {
	Name      string `json:"name" jsonschema:"required,description=Room display name,minLength=1,maxLength=80"`
	IsPrivate bool   `json:"is_private" jsonschema:"description=Whether the room is private (invite-only)"`
}

// CreateDMRequest creates or finds an existing DM with the given users
// Direction: client → server
// Response: CreateDMResponse
type CreateDMRequest struct {
	UserIDs []string `json:"user_ids" jsonschema:"required,description=User IDs to start DM with (not including self),minItems=1"`
}

// ListRoomsRequest is sent by the client to get a list of public rooms
// Direction: client → server
// Response: ListRoomsResponse
type ListRoomsRequest struct {
	Query string `json:"query" jsonschema:"description=Optional search query to filter rooms by name"`
}

// ListUsersRequest searches for users (for user picker in DM creation)
// Direction: client → server
// Response: ListUsersResponse
type ListUsersRequest struct {
	Query string `json:"query" jsonschema:"description=Search query for username (partial match)"`
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

// GetProfileRequest fetches a user's profile
// Direction: client → server
// Response: GetProfileResponse
type GetProfileRequest struct {
	UserID string `json:"user_id" jsonschema:"required,description=User ID to get profile for"`
}

// UpdateProfileRequest updates the current user's profile
// Direction: client → server
// Response: UpdateProfileResponse
type UpdateProfileRequest struct {
	DisplayName *string `json:"display_name,omitempty" jsonschema:"description=New display name (omit to keep current)"`
	Status      *string `json:"status,omitempty" jsonschema:"description=New status message (omit to keep current)"`
}

// EditMessageRequest edits a message's body. Only the message author can edit.
// Direction: client → server
// Broadcast: MessageEdited to room members
type EditMessageRequest struct {
	MessageID string `json:"message_id" jsonschema:"required,description=ID of the message to edit"`
	Body      string `json:"body" jsonschema:"required,description=New message body,minLength=1"`
}

// DeleteMessageRequest soft-deletes a message. Only the message author can delete.
// Direction: client → server
// Broadcast: MessageDeleted to room members
type DeleteMessageRequest struct {
	MessageID string `json:"message_id" jsonschema:"required,description=ID of the message to delete"`
}

// AddReactionRequest adds an emoji reaction to a message. Any room member can react.
// Direction: client → server
// Broadcast: ReactionUpdated to room members
type AddReactionRequest struct {
	MessageID string `json:"message_id" jsonschema:"required,description=ID of the message to react to"`
	Emoji     string `json:"emoji" jsonschema:"required,description=Emoji character(s) to react with"`
}

// RemoveReactionRequest removes the user's emoji reaction from a message.
// Direction: client → server
// Broadcast: ReactionUpdated to room members
type RemoveReactionRequest struct {
	MessageID string `json:"message_id" jsonschema:"required,description=ID of the message to remove reaction from"`
	Emoji     string `json:"emoji" jsonschema:"required,description=Emoji character(s) to remove"`
}

// SearchRequest searches messages across rooms the user has access to
// Direction: client → server
// Response: SearchResponse
type SearchRequest struct {
	Query  string `json:"query" jsonschema:"required,description=Search query text,minLength=1"`
	RoomID string `json:"room_id,omitempty" jsonschema:"description=Filter to specific room"`
	UserID string `json:"user_id,omitempty" jsonschema:"description=Filter to messages from specific user"`
	Cursor string `json:"cursor,omitempty" jsonschema:"description=Pagination cursor for next page"`
	Limit  int    `json:"limit,omitempty" jsonschema:"description=Max results to return (default 20),minimum=1,maximum=100"`
}

// GetMessageContextRequest fetches a message with surrounding context for permalinks
// Direction: client → server
// Response: GetMessageContextResponse
type GetMessageContextRequest struct {
	MessageID string `json:"message_id" jsonschema:"required,description=ID of the message to get context for"`
}

// =============================================================================
// Server → Client Messages
// =============================================================================

// InitResponse is sent by the server in response to InitRequest
// Direction: server → client
type InitResponse struct {
	User        User    `json:"user" jsonschema:"required,description=The authenticated user"`
	Rooms       []*Room `json:"rooms" jsonschema:"required,description=Channel rooms the user is a member of"`
	DMs         []*Room `json:"dms" jsonschema:"required,description=DM rooms the user is a member of (sorted by most recent activity)"`
	CurrentRoom string  `json:"current_room" jsonschema:"required,description=Room ID to display initially"`
}

// HistoryResponse is sent by the server in response to HistoryRequest
// Direction: server → client
type HistoryResponse struct {
	Messages   []*Message `json:"messages" jsonschema:"required,description=Messages in chronological order (newest first)"`
	HasMore    bool       `json:"has_more" jsonschema:"required,description=Whether older messages exist"`
	NextCursor string     `json:"next_cursor" jsonschema:"required,description=Pass as cursor to fetch older messages"`
}

// MessageEdited is broadcast to room members when a message is edited
// Direction: server → client (broadcast)
type MessageEdited struct {
	MessageID  string `json:"message_id" jsonschema:"required,description=ID of the edited message"`
	Body       string `json:"body" jsonschema:"required,description=New message body"`
	RoomID     string `json:"room_id" jsonschema:"required,description=Room the message belongs to"`
	ModifiedAt string `json:"modified_at" jsonschema:"required,description=RFC3339Nano timestamp of the edit"`
}

// MessageDeleted is broadcast to room members when a message is soft-deleted
// Direction: server → client (broadcast)
type MessageDeleted struct {
	MessageID string `json:"message_id" jsonschema:"required,description=ID of the deleted message"`
	RoomID    string `json:"room_id" jsonschema:"required,description=Room the message belongs to"`
}

// ReactionUpdated is broadcast when a reaction is added or removed
// Direction: server → client (broadcast)
type ReactionUpdated struct {
	MessageID string `json:"message_id" jsonschema:"required,description=ID of the message"`
	RoomID    string `json:"room_id" jsonschema:"required,description=Room the message belongs to"`
	UserID    string `json:"user_id" jsonschema:"required,description=User who added/removed the reaction"`
	Emoji     string `json:"emoji" jsonschema:"required,description=The emoji character(s)"`
	Action    string `json:"action" jsonschema:"required,description=Whether the reaction was added or removed,enum=add,enum=remove"`
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

// CreateDMResponse is sent by the server in response to CreateDMRequest
// Direction: server → client
type CreateDMResponse struct {
	Room    Room `json:"room" jsonschema:"required,description=The DM room (existing or newly created)"`
	Created bool `json:"created" jsonschema:"required,description=True if a new DM was created (false if existing DM was found)"`
}

// ListRoomsResponse is sent by the server in response to ListRoomsRequest
// Direction: server → client
type ListRoomsResponse struct {
	Rooms    []*Room `json:"rooms" jsonschema:"required,description=List of public rooms"`
	IsMember []bool  `json:"is_member" jsonschema:"required,description=Whether the user is a member of each room (parallel array)"`
}

// ListUsersResponse is sent by the server in response to ListUsersRequest
// Direction: server → client
type ListUsersResponse struct {
	Users []User `json:"users" jsonschema:"required,description=List of matching users"`
}

// LeaveRoomResponse is sent by the server in response to LeaveRoomRequest
// Direction: server → client
type LeaveRoomResponse struct {
	RoomID string `json:"room_id" jsonschema:"required,description=Room ID that was left"`
}

// RoomInfoResponse is sent by the server in response to RoomInfoRequest
// Direction: server → client
type RoomInfoResponse struct {
	Room        Room         `json:"room" jsonschema:"required,description=Room details"`
	MemberCount int          `json:"member_count" jsonschema:"required,description=Number of members in the room"`
	Members     []RoomMember `json:"members" jsonschema:"required,description=List of room members"`
	CreatedAt   string       `json:"created_at" jsonschema:"required,description=RFC3339 timestamp of when the room was created"`
}

// GetProfileResponse is sent by the server in response to GetProfileRequest
// Direction: server → client
type GetProfileResponse struct {
	User User `json:"user" jsonschema:"required,description=User profile data"`
}

// UpdateProfileResponse is sent by the server in response to UpdateProfileRequest
// Direction: server → client
type UpdateProfileResponse struct {
	User User `json:"user" jsonschema:"required,description=Updated user profile"`
}

// SearchResponse returns matching messages with snippets
// Direction: server → client
type SearchResponse struct {
	Results    []SearchResult `json:"results" jsonschema:"required,description=Matching messages with snippets"`
	NextCursor string         `json:"next_cursor,omitempty" jsonschema:"description=Pagination cursor for next page"`
	Total      int            `json:"total,omitempty" jsonschema:"description=Approximate total matches"`
}

// SearchResult is a single search hit with context snippet
type SearchResult struct {
	MessageID string `json:"message_id" jsonschema:"required,description=ID of the matching message"`
	RoomID    string `json:"room_id" jsonschema:"required,description=Room the message belongs to"`
	RoomName  string `json:"room_name" jsonschema:"required,description=Name of the room (for display)"`
	UserID    string `json:"user_id" jsonschema:"required,description=Author of the message"`
	Username  string `json:"username" jsonschema:"required,description=Username of the author"`
	Snippet   string `json:"snippet" jsonschema:"required,description=Message excerpt with **highlighted** matches"`
	CreatedAt string `json:"created_at" jsonschema:"required,description=RFC3339Nano timestamp of the message"`
}

// GetMessageContextResponse returns a message and its room for permalink navigation
// Direction: server → client
type GetMessageContextResponse struct {
	Message Message `json:"message" jsonschema:"required,description=The requested message"`
	RoomID  string  `json:"room_id" jsonschema:"required,description=Room the message belongs to"`
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
		Description: "Response with user info, rooms, DMs, and current room",
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
		Description: "Create a new channel room",
	},
	{
		Type:        "create_room",
		Direction:   ServerToClient,
		Description: "Response with the newly created room",
	},
	{
		Type:        "create_dm",
		Direction:   ClientToServer,
		Description: "Create or find an existing DM with specified users",
	},
	{
		Type:        "create_dm",
		Direction:   ServerToClient,
		Description: "Response with the DM room (new or existing)",
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
		Type:        "list_users",
		Direction:   ClientToServer,
		Description: "Search for users (for DM user picker)",
	},
	{
		Type:        "list_users",
		Direction:   ServerToClient,
		Description: "Response with matching users",
	},
	{
		Type:        "leave_room",
		Direction:   ClientToServer,
		Description: "Leave a room (not allowed for 1:1 DMs)",
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
	{
		Type:        "get_profile",
		Direction:   ClientToServer,
		Description: "Request a user's profile",
	},
	{
		Type:        "get_profile",
		Direction:   ServerToClient,
		Description: "Response with user profile data",
	},
	{
		Type:        "update_profile",
		Direction:   ClientToServer,
		Description: "Update current user's profile",
	},
	{
		Type:        "update_profile",
		Direction:   ServerToClient,
		Description: "Response with updated profile",
	},
	{
		Type:        "edit_message",
		Direction:   ClientToServer,
		Description: "Edit a message's body (author only)",
	},
	{
		Type:        "message_edited",
		Direction:   ServerToClient,
		Description: "Broadcast when a message is edited",
	},
	{
		Type:        "delete_message",
		Direction:   ClientToServer,
		Description: "Soft-delete a message (author only)",
	},
	{
		Type:        "message_deleted",
		Direction:   ServerToClient,
		Description: "Broadcast when a message is deleted",
	},
	{
		Type:        "add_reaction",
		Direction:   ClientToServer,
		Description: "Add an emoji reaction to a message",
	},
	{
		Type:        "remove_reaction",
		Direction:   ClientToServer,
		Description: "Remove an emoji reaction from a message",
	},
	{
		Type:        "reaction_updated",
		Direction:   ServerToClient,
		Description: "Broadcast when a reaction is added or removed",
	},
	{
		Type:        "search",
		Direction:   ClientToServer,
		Description: "Search messages across accessible rooms",
	},
	{
		Type:        "search",
		Direction:   ServerToClient,
		Description: "Response with matching messages and snippets",
	},
	{
		Type:        "get_message_context",
		Direction:   ClientToServer,
		Description: "Get a message and its room for permalink navigation",
	},
	{
		Type:        "get_message_context",
		Direction:   ServerToClient,
		Description: "Response with message and room ID",
	},
}
