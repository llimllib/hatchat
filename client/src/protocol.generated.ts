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
  id: z.string().regex(/^usr_[a-f0-9]{16}$/),
  username: z.string(),
});

export const RoomSchema = z.object({
  id: z.string().regex(/^roo_[a-f0-9]{12}$/),
  is_private: z.boolean(),
  name: z.string(),
});

export const MessageSchema = z.object({
  body: z.string(),
  created_at: z.string(),
  id: z.string().regex(/^msg_[a-f0-9]{12}$/),
  modified_at: z.string(),
  room_id: z.string(),
  user_id: z.string(),
  username: z.string(),
});

export const RoomMemberSchema = z.object({
  avatar: z.string().optional(),
  id: z.string(),
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

export const ListRoomsRequestSchema = z.object({
  query: z.string().optional(),
});

export const LeaveRoomRequestSchema = z.object({
  room_id: z.string(),
});

export const RoomInfoRequestSchema = z.object({
  room_id: z.string(),
});

export const InitResponseSchema = z.object({
  Rooms: z.array(RoomSchema),
  User: UserSchema,
  current_room: z.string(),
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

export const ListRoomsResponseSchema = z.object({
  is_member: z.array(z.boolean()),
  rooms: z.array(RoomSchema),
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

export const ErrorResponseSchema = z.object({
  message: z.string(),
});

export const EnvelopeSchema = z.object({
  data: z.unknown(),
  type: z.string(),
});

// =============================================================================
// Inferred TypeScript types
// =============================================================================

export type User = z.infer<typeof UserSchema>;
export type Room = z.infer<typeof RoomSchema>;
export type Message = z.infer<typeof MessageSchema>;
export type RoomMember = z.infer<typeof RoomMemberSchema>;
export type InitRequest = z.infer<typeof InitRequestSchema>;
export type SendMessageRequest = z.infer<typeof SendMessageRequestSchema>;
export type HistoryRequest = z.infer<typeof HistoryRequestSchema>;
export type JoinRoomRequest = z.infer<typeof JoinRoomRequestSchema>;
export type CreateRoomRequest = z.infer<typeof CreateRoomRequestSchema>;
export type ListRoomsRequest = z.infer<typeof ListRoomsRequestSchema>;
export type LeaveRoomRequest = z.infer<typeof LeaveRoomRequestSchema>;
export type RoomInfoRequest = z.infer<typeof RoomInfoRequestSchema>;
export type InitResponse = z.infer<typeof InitResponseSchema>;
export type HistoryResponse = z.infer<typeof HistoryResponseSchema>;
export type JoinRoomResponse = z.infer<typeof JoinRoomResponseSchema>;
export type CreateRoomResponse = z.infer<typeof CreateRoomResponseSchema>;
export type ListRoomsResponse = z.infer<typeof ListRoomsResponseSchema>;
export type LeaveRoomResponse = z.infer<typeof LeaveRoomResponseSchema>;
export type RoomInfoResponse = z.infer<typeof RoomInfoResponseSchema>;
export type ErrorResponse = z.infer<typeof ErrorResponseSchema>;
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
  | "list_rooms"
  | "leave_room"
  | "room_info"
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
  | { type: "list_rooms"; data: ListRoomsRequest }
  | { type: "leave_room"; data: LeaveRoomRequest }
  | { type: "room_info"; data: RoomInfoRequest };

/**
 * Type-safe envelope for server → client messages
 */
export type ServerEnvelope =
  | { type: "init"; data: InitResponse }
  | { type: "message"; data: Message }
  | { type: "history"; data: HistoryResponse }
  | { type: "join_room"; data: JoinRoomResponse }
  | { type: "create_room"; data: CreateRoomResponse }
  | { type: "list_rooms"; data: ListRoomsResponse }
  | { type: "leave_room"; data: LeaveRoomResponse }
  | { type: "room_info"; data: RoomInfoResponse }
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

export const ListRoomsEnvelopeSchema = z.object({
  type: z.literal("list_rooms"),
  data: ListRoomsResponseSchema,
});

export const LeaveRoomEnvelopeSchema = z.object({
  type: z.literal("leave_room"),
  data: LeaveRoomResponseSchema,
});

export const RoomInfoEnvelopeSchema = z.object({
  type: z.literal("room_info"),
  data: RoomInfoResponseSchema,
});

export const ErrorEnvelopeSchema = z.object({
  type: z.literal("error"),
  data: ErrorResponseSchema,
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
  ListRoomsEnvelopeSchema,
  LeaveRoomEnvelopeSchema,
  RoomInfoEnvelopeSchema,
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
