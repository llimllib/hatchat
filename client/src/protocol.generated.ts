/* eslint-disable */
/**
 * This file was automatically generated from schema/protocol.json
 * DO NOT EDIT MANUALLY - run `just client-types` to regenerate
 */

import { z } from "zod/v4";

// =============================================================================
// Zod Schemas - validated against JSON Schema definitions
// =============================================================================

export const UserSchema = z.object({
  avatar: z.string().optional(),
  display_name: z.string().optional(),
  id: z.string().regex(/^usr_[a-f0-9]{16}$/),
  status: z.string().optional(),
  username: z.string(),
});

export const RoomMemberSchema = z.object({
  avatar: z.string().optional(),
  display_name: z.string().optional(),
  id: z.string(),
  username: z.string(),
});

export const RoomSchema = z.object({
  id: z.string().regex(/^roo_[a-f0-9]{12}$/),
  is_private: z.boolean(),
  members: z.array(RoomMemberSchema).optional(),
  name: z.string(),
  room_type: z.string(),
});

export const ReactionSchema = z.object({
  count: z.int(),
  emoji: z.string(),
  user_ids: z.array(z.string()),
});

export const MessageSchema = z.object({
  body: z.string(),
  created_at: z.string(),
  deleted_at: z.string().optional(),
  id: z.string().regex(/^msg_[a-f0-9]{12}$/),
  modified_at: z.string(),
  reactions: z.array(ReactionSchema).optional(),
  room_id: z.string(),
  user_id: z.string(),
  username: z.string(),
});

export const InitRequestSchema = z.object({});

export const SendMessageRequestSchema = z.object({
  body: z.string().min(1),
  room_id: z.string().min(1),
});

export const HistoryRequestSchema = z.object({
  cursor: z.string().optional(),
  limit: z.int().min(1).max(100).optional(),
  room_id: z.string(),
});

export const JoinRoomRequestSchema = z.object({
  room_id: z.string(),
});

export const CreateRoomRequestSchema = z.object({
  is_private: z.boolean().optional(),
  name: z.string().min(1).max(80),
});

export const CreateDMRequestSchema = z.object({
  user_ids: z.array(z.string()),
});

export const ListRoomsRequestSchema = z.object({
  query: z.string().optional(),
});

export const ListUsersRequestSchema = z.object({
  query: z.string().optional(),
});

export const LeaveRoomRequestSchema = z.object({
  room_id: z.string(),
});

export const RoomInfoRequestSchema = z.object({
  room_id: z.string(),
});

export const GetProfileRequestSchema = z.object({
  user_id: z.string(),
});

export const UpdateProfileRequestSchema = z.object({
  display_name: z.string().optional(),
  status: z.string().optional(),
});

export const EditMessageRequestSchema = z.object({
  body: z.string().min(1),
  message_id: z.string(),
});

export const DeleteMessageRequestSchema = z.object({
  message_id: z.string(),
});

export const AddReactionRequestSchema = z.object({
  emoji: z.string(),
  message_id: z.string(),
});

export const RemoveReactionRequestSchema = z.object({
  emoji: z.string(),
  message_id: z.string(),
});

export const SearchRequestSchema = z.object({
  cursor: z.string().optional(),
  limit: z.int().min(1).max(100).optional(),
  query: z.string().min(1),
  room_id: z.string().optional(),
  user_id: z.string().optional(),
});

export const GetMessageContextRequestSchema = z.object({
  message_id: z.string(),
});

export const InitResponseSchema = z.object({
  current_room: z.string(),
  dms: z.array(RoomSchema),
  rooms: z.array(RoomSchema),
  user: UserSchema,
});

export const HistoryResponseSchema = z.object({
  has_more: z.boolean(),
  messages: z.array(MessageSchema),
  next_cursor: z.string(),
});

export const JoinRoomResponseSchema = z.object({
  joined: z.boolean(),
  room: RoomSchema,
});

export const CreateRoomResponseSchema = z.object({
  room: RoomSchema,
});

export const CreateDMResponseSchema = z.object({
  created: z.boolean(),
  room: RoomSchema,
});

export const ListRoomsResponseSchema = z.object({
  is_member: z.array(z.boolean()),
  rooms: z.array(RoomSchema),
});

export const ListUsersResponseSchema = z.object({
  users: z.array(UserSchema),
});

export const LeaveRoomResponseSchema = z.object({
  room_id: z.string(),
});

export const RoomInfoResponseSchema = z.object({
  created_at: z.string(),
  member_count: z.int(),
  members: z.array(RoomMemberSchema),
  room: RoomSchema,
});

export const GetProfileResponseSchema = z.object({
  user: UserSchema,
});

export const UpdateProfileResponseSchema = z.object({
  user: UserSchema,
});

export const ErrorResponseSchema = z.object({
  message: z.string(),
});

export const MessageEditedSchema = z.object({
  body: z.string(),
  message_id: z.string(),
  modified_at: z.string(),
  room_id: z.string(),
});

export const MessageDeletedSchema = z.object({
  message_id: z.string(),
  room_id: z.string(),
});

export const ReactionUpdatedSchema = z.object({
  action: z.string(),
  emoji: z.string(),
  message_id: z.string(),
  room_id: z.string(),
  user_id: z.string(),
});

export const SearchResultSchema = z.object({
  created_at: z.string(),
  message_id: z.string(),
  room_id: z.string(),
  room_name: z.string(),
  snippet: z.string(),
  user_id: z.string(),
  username: z.string(),
});

export const SearchResponseSchema = z.object({
  next_cursor: z.string().optional(),
  results: z.array(SearchResultSchema),
  total: z.int().optional(),
});

export const GetMessageContextResponseSchema = z.object({
  message: MessageSchema,
  room_id: z.string(),
});

export const EnvelopeSchema = z.object({
  data: z.unknown(),
  type: z.string(),
});

// =============================================================================
// Inferred TypeScript types
// =============================================================================

export type User = z.infer<typeof UserSchema>;
export type RoomMember = z.infer<typeof RoomMemberSchema>;
export type Room = z.infer<typeof RoomSchema>;
export type Reaction = z.infer<typeof ReactionSchema>;
export type Message = z.infer<typeof MessageSchema>;
export type InitRequest = z.infer<typeof InitRequestSchema>;
export type SendMessageRequest = z.infer<typeof SendMessageRequestSchema>;
export type HistoryRequest = z.infer<typeof HistoryRequestSchema>;
export type JoinRoomRequest = z.infer<typeof JoinRoomRequestSchema>;
export type CreateRoomRequest = z.infer<typeof CreateRoomRequestSchema>;
export type CreateDMRequest = z.infer<typeof CreateDMRequestSchema>;
export type ListRoomsRequest = z.infer<typeof ListRoomsRequestSchema>;
export type ListUsersRequest = z.infer<typeof ListUsersRequestSchema>;
export type LeaveRoomRequest = z.infer<typeof LeaveRoomRequestSchema>;
export type RoomInfoRequest = z.infer<typeof RoomInfoRequestSchema>;
export type GetProfileRequest = z.infer<typeof GetProfileRequestSchema>;
export type UpdateProfileRequest = z.infer<typeof UpdateProfileRequestSchema>;
export type EditMessageRequest = z.infer<typeof EditMessageRequestSchema>;
export type DeleteMessageRequest = z.infer<typeof DeleteMessageRequestSchema>;
export type AddReactionRequest = z.infer<typeof AddReactionRequestSchema>;
export type RemoveReactionRequest = z.infer<typeof RemoveReactionRequestSchema>;
export type SearchRequest = z.infer<typeof SearchRequestSchema>;
export type GetMessageContextRequest = z.infer<
  typeof GetMessageContextRequestSchema
>;
export type InitResponse = z.infer<typeof InitResponseSchema>;
export type HistoryResponse = z.infer<typeof HistoryResponseSchema>;
export type JoinRoomResponse = z.infer<typeof JoinRoomResponseSchema>;
export type CreateRoomResponse = z.infer<typeof CreateRoomResponseSchema>;
export type CreateDMResponse = z.infer<typeof CreateDMResponseSchema>;
export type ListRoomsResponse = z.infer<typeof ListRoomsResponseSchema>;
export type ListUsersResponse = z.infer<typeof ListUsersResponseSchema>;
export type LeaveRoomResponse = z.infer<typeof LeaveRoomResponseSchema>;
export type RoomInfoResponse = z.infer<typeof RoomInfoResponseSchema>;
export type GetProfileResponse = z.infer<typeof GetProfileResponseSchema>;
export type UpdateProfileResponse = z.infer<typeof UpdateProfileResponseSchema>;
export type ErrorResponse = z.infer<typeof ErrorResponseSchema>;
export type MessageEdited = z.infer<typeof MessageEditedSchema>;
export type MessageDeleted = z.infer<typeof MessageDeletedSchema>;
export type ReactionUpdated = z.infer<typeof ReactionUpdatedSchema>;
export type SearchResult = z.infer<typeof SearchResultSchema>;
export type SearchResponse = z.infer<typeof SearchResponseSchema>;
export type GetMessageContextResponse = z.infer<
  typeof GetMessageContextResponseSchema
>;
export type Envelope = z.infer<typeof EnvelopeSchema>;

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
  | "create_room"
  | "create_dm"
  | "list_rooms"
  | "list_users"
  | "leave_room"
  | "room_info"
  | "get_profile"
  | "update_profile"
  | "edit_message"
  | "delete_message"
  | "add_reaction"
  | "remove_reaction"
  | "search"
  | "get_message_context"
  | "message_edited"
  | "message_deleted"
  | "reaction_updated"
  | "error";

/**
 * Type-safe envelope for client → server messages
 */
export type ClientEnvelope =
  | { type: "init"; data: InitRequest }
  | { type: "message"; data: SendMessageRequest }
  | { type: "history"; data: HistoryRequest }
  | { type: "join_room"; data: JoinRoomRequest }
  | { type: "create_room"; data: CreateRoomRequest }
  | { type: "create_dm"; data: CreateDMRequest }
  | { type: "list_rooms"; data: ListRoomsRequest }
  | { type: "list_users"; data: ListUsersRequest }
  | { type: "leave_room"; data: LeaveRoomRequest }
  | { type: "room_info"; data: RoomInfoRequest }
  | { type: "get_profile"; data: GetProfileRequest }
  | { type: "update_profile"; data: UpdateProfileRequest }
  | { type: "edit_message"; data: EditMessageRequest }
  | { type: "delete_message"; data: DeleteMessageRequest }
  | { type: "add_reaction"; data: AddReactionRequest }
  | { type: "remove_reaction"; data: RemoveReactionRequest }
  | { type: "search"; data: SearchRequest }
  | { type: "get_message_context"; data: GetMessageContextRequest };

/**
 * Type-safe envelope for server → client messages
 */
export type ServerEnvelope =
  | { type: "init"; data: InitResponse }
  | { type: "message"; data: Message }
  | { type: "history"; data: HistoryResponse }
  | { type: "join_room"; data: JoinRoomResponse }
  | { type: "create_room"; data: CreateRoomResponse }
  | { type: "create_dm"; data: CreateDMResponse }
  | { type: "list_rooms"; data: ListRoomsResponse }
  | { type: "list_users"; data: ListUsersResponse }
  | { type: "leave_room"; data: LeaveRoomResponse }
  | { type: "room_info"; data: RoomInfoResponse }
  | { type: "get_profile"; data: GetProfileResponse }
  | { type: "update_profile"; data: UpdateProfileResponse }
  | { type: "search"; data: SearchResponse }
  | { type: "get_message_context"; data: GetMessageContextResponse }
  | { type: "message_edited"; data: MessageEdited }
  | { type: "message_deleted"; data: MessageDeleted }
  | { type: "reaction_updated"; data: ReactionUpdated }
  | { type: "error"; data: ErrorResponse };

// =============================================================================
// Runtime validation schemas for server envelopes
// =============================================================================

export const InitEnvelopeSchema = z.object({
  type: z.literal("init"),
  data: InitResponseSchema,
});

export const MessageEnvelopeSchema = z.object({
  type: z.literal("message"),
  data: MessageSchema,
});

export const HistoryEnvelopeSchema = z.object({
  type: z.literal("history"),
  data: HistoryResponseSchema,
});

export const JoinRoomEnvelopeSchema = z.object({
  type: z.literal("join_room"),
  data: JoinRoomResponseSchema,
});

export const CreateRoomEnvelopeSchema = z.object({
  type: z.literal("create_room"),
  data: CreateRoomResponseSchema,
});

export const CreateDMEnvelopeSchema = z.object({
  type: z.literal("create_dm"),
  data: CreateDMResponseSchema,
});

export const ListRoomsEnvelopeSchema = z.object({
  type: z.literal("list_rooms"),
  data: ListRoomsResponseSchema,
});

export const ListUsersEnvelopeSchema = z.object({
  type: z.literal("list_users"),
  data: ListUsersResponseSchema,
});

export const LeaveRoomEnvelopeSchema = z.object({
  type: z.literal("leave_room"),
  data: LeaveRoomResponseSchema,
});

export const RoomInfoEnvelopeSchema = z.object({
  type: z.literal("room_info"),
  data: RoomInfoResponseSchema,
});

export const GetProfileEnvelopeSchema = z.object({
  type: z.literal("get_profile"),
  data: GetProfileResponseSchema,
});

export const UpdateProfileEnvelopeSchema = z.object({
  type: z.literal("update_profile"),
  data: UpdateProfileResponseSchema,
});

export const MessageEditedEnvelopeSchema = z.object({
  type: z.literal("message_edited"),
  data: MessageEditedSchema,
});

export const MessageDeletedEnvelopeSchema = z.object({
  type: z.literal("message_deleted"),
  data: MessageDeletedSchema,
});

export const ReactionUpdatedEnvelopeSchema = z.object({
  type: z.literal("reaction_updated"),
  data: ReactionUpdatedSchema,
});

export const ErrorEnvelopeSchema = z.object({
  type: z.literal("error"),
  data: ErrorResponseSchema,
});

export const SearchEnvelopeSchema = z.object({
  type: z.literal("search"),
  data: SearchResponseSchema,
});

export const GetMessageContextEnvelopeSchema = z.object({
  type: z.literal("get_message_context"),
  data: GetMessageContextResponseSchema,
});

/**
 * Discriminated union schema for all server → client messages
 */
export const ServerEnvelopeSchema = z.discriminatedUnion("type", [
  InitEnvelopeSchema,
  MessageEnvelopeSchema,
  HistoryEnvelopeSchema,
  JoinRoomEnvelopeSchema,
  CreateRoomEnvelopeSchema,
  CreateDMEnvelopeSchema,
  ListRoomsEnvelopeSchema,
  ListUsersEnvelopeSchema,
  LeaveRoomEnvelopeSchema,
  RoomInfoEnvelopeSchema,
  GetProfileEnvelopeSchema,
  UpdateProfileEnvelopeSchema,
  SearchEnvelopeSchema,
  GetMessageContextEnvelopeSchema,
  MessageEditedEnvelopeSchema,
  MessageDeletedEnvelopeSchema,
  ReactionUpdatedEnvelopeSchema,
  ErrorEnvelopeSchema,
]);

/**
 * Type guard for checking message type
 */
export function isMessageType<T extends MessageType>(
  envelope: ServerEnvelope,
  type: T,
): envelope is Extract<ServerEnvelope, { type: T }> {
  return envelope.type === type;
}

/**
 * Parse and validate a server envelope from raw JSON
 * @throws z.ZodError if validation fails
 */
export function parseServerEnvelope(data: unknown): ServerEnvelope {
  return ServerEnvelopeSchema.parse(data);
}

/**
 * Safely parse a server envelope, returning null on failure
 */
export function safeParseServerEnvelope(data: unknown): ServerEnvelope | null {
  const result = ServerEnvelopeSchema.safeParse(data);
  return result.success ? result.data : null;
}
