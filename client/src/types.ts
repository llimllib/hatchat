// Re-export protocol types and schemas from generated file
export type {
  ClientEnvelope,
  CreateRoomRequest,
  CreateRoomResponse,
  Envelope,
  ErrorResponse,
  HistoryRequest,
  HistoryResponse,
  InitRequest,
  InitResponse,
  JoinRoomRequest,
  JoinRoomResponse,
  ListRoomsRequest,
  ListRoomsResponse,
  Message,
  MessageType,
  Room,
  SendMessageRequest,
  ServerEnvelope,
  User,
} from "./protocol.generated";

export {
  CreateRoomResponseSchema,
  ErrorResponseSchema,
  HistoryResponseSchema,
  InitResponseSchema,
  isMessageType,
  JoinRoomResponseSchema,
  ListRoomsResponseSchema,
  MessageSchema,
  parseServerEnvelope,
  RoomSchema,
  // Zod schemas for runtime validation
  ServerEnvelopeSchema,
  safeParseServerEnvelope,
  UserSchema,
} from "./protocol.generated";

// =============================================================================
// Client-specific types (not part of the protocol)
// =============================================================================

/**
 * InitialData is an alias for InitResponse for backward compatibility.
 * Use InitResponse directly in new code.
 */
export type { InitResponse as InitialData } from "./protocol.generated";

/**
 * Pending message waiting for server confirmation.
 * This is a client-side concept for optimistic UI updates.
 */
export interface PendingMessage {
  tempId: string;
  body: string;
  roomId: string;
  element: HTMLElement;
}

/**
 * Create a unique key for matching pending messages with server confirmations.
 * We match by user_id + room_id + body since we don't have the server ID yet.
 */
export function makePendingKey(
  body: string,
  roomId: string,
  userId: string,
): string {
  return `${userId}:${roomId}:${body}`;
}
