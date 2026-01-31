/* eslint-disable */
/**
 * This file was automatically generated from schema/protocol.json
 * DO NOT EDIT MANUALLY - run `just client-types` to regenerate
 */

export interface User {
  /**
   * Avatar URL (may be empty)
   */
  avatar?: string;
  /**
   * Unique user identifier (usr_ prefix)
   */
  id: string;
  /**
   * Display name
   */
  username: string;
}

export interface Room {
  /**
   * Unique room identifier (roo_ prefix)
   */
  id: string;
  /**
   * Whether the room is private
   */
  is_private: boolean;
  /**
   * Room display name
   */
  name: string;
}

export interface Message {
  /**
   * Message content
   */
  body: string;
  /**
   * RFC3339Nano timestamp of creation
   */
  created_at: string;
  /**
   * Unique message identifier (msg_ prefix)
   */
  id: string;
  /**
   * RFC3339Nano timestamp of last modification
   */
  modified_at: string;
  /**
   * Room this message belongs to
   */
  room_id: string;
  /**
   * User who sent the message
   */
  user_id: string;
  /**
   * Username of sender (denormalized for convenience)
   */
  username: string;
}

export type InitRequest = Record<string, never>;

export interface SendMessageRequest {
  /**
   * Message content
   */
  body: string;
  /**
   * Target room ID
   */
  room_id: string;
}

export interface HistoryRequest {
  /**
   * Pagination cursor (created_at of oldest message seen)
   */
  cursor?: string;
  /**
   * Maximum messages to return (default 50; max 100)
   */
  limit?: number;
  /**
   * Room to fetch history for
   */
  room_id: string;
}

export interface JoinRoomRequest {
  /**
   * Room ID to switch to
   */
  room_id: string;
}

export interface InitResponse {
  /**
   * Rooms the user is a member of
   */
  Rooms: {
    /**
     * Unique room identifier (roo_ prefix)
     */
    id: string;
    /**
     * Whether the room is private
     */
    is_private: boolean;
    /**
     * Room display name
     */
    name: string;
  }[];
  /**
   * The authenticated user
   */
  User: {
    /**
     * Avatar URL (may be empty)
     */
    avatar?: string;
    /**
     * Unique user identifier (usr_ prefix)
     */
    id: string;
    /**
     * Display name
     */
    username: string;
  };
  /**
   * Room ID to display initially
   */
  current_room: string;
}

export interface HistoryResponse {
  /**
   * Whether older messages exist
   */
  has_more: boolean;
  /**
   * Messages in chronological order (newest first)
   */
  messages: {
    /**
     * Message content
     */
    body: string;
    /**
     * RFC3339Nano timestamp of creation
     */
    created_at: string;
    /**
     * Unique message identifier (msg_ prefix)
     */
    id: string;
    /**
     * RFC3339Nano timestamp of last modification
     */
    modified_at: string;
    /**
     * Room this message belongs to
     */
    room_id: string;
    /**
     * User who sent the message
     */
    user_id: string;
    /**
     * Username of sender (denormalized for convenience)
     */
    username: string;
  }[];
  /**
   * Pass as cursor to fetch older messages
   */
  next_cursor: string;
}

export interface JoinRoomResponse {
  /**
   * The room that was joined
   */
  room: {
    /**
     * Unique room identifier (roo_ prefix)
     */
    id: string;
    /**
     * Whether the room is private
     */
    is_private: boolean;
    /**
     * Room display name
     */
    name: string;
  };
}

export interface ErrorResponse {
  /**
   * Human-readable error message
   */
  message: string;
}

export interface Envelope {
  /**
   * Type-specific payload
   */
  data: {
    [k: string]: unknown | undefined;
  };
  /**
   * Message type identifier
   */
  type: string;
}

// =============================================================================
// Helper types for working with the protocol
// =============================================================================

/**
 * All valid message type strings
 */
export type MessageType =
  | "init"
  | "message"
  | "history"
  | "join_room"
  | "error";

/**
 * Type-safe envelope for client → server messages
 */
export type ClientEnvelope =
  | { type: "init"; data: InitRequest }
  | { type: "message"; data: SendMessageRequest }
  | { type: "history"; data: HistoryRequest }
  | { type: "join_room"; data: JoinRoomRequest };

/**
 * Type-safe envelope for server → client messages
 */
export type ServerEnvelope =
  | { type: "init"; data: InitResponse }
  | { type: "message"; data: Message }
  | { type: "history"; data: HistoryResponse }
  | { type: "join_room"; data: JoinRoomResponse }
  | { type: "error"; data: ErrorResponse };

/**
 * Type guard for checking message type
 */
export function isMessageType<T extends MessageType>(
  envelope: ServerEnvelope,
  type: T,
): envelope is Extract<ServerEnvelope, { type: T }> {
  return envelope.type === type;
}
