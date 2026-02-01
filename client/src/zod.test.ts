/**
 * Tests for Zod runtime validation of protocol messages.
 * These ensure that the generated Zod schemas correctly validate
 * incoming WebSocket messages at runtime.
 */

import { describe, expect, it } from "vitest";
import {
  MessageSchema,
  parseServerEnvelope,
  RoomSchema,
  ServerEnvelopeSchema,
  safeParseServerEnvelope,
  UserSchema,
} from "./protocol.generated";

describe("Zod Schema Validation", () => {
  describe("UserSchema", () => {
    it("validates a valid user", () => {
      const user = {
        id: "usr_1234567890abcdef",
        username: "testuser",
        avatar: "https://example.com/avatar.png",
      };
      const result = UserSchema.safeParse(user);
      expect(result.success).toBe(true);
    });

    it("validates user without avatar", () => {
      const user = {
        id: "usr_1234567890abcdef",
        username: "testuser",
      };
      const result = UserSchema.safeParse(user);
      expect(result.success).toBe(true);
    });

    it("rejects user with invalid ID pattern", () => {
      const user = {
        id: "invalid_id",
        username: "testuser",
      };
      const result = UserSchema.safeParse(user);
      expect(result.success).toBe(false);
    });

    it("rejects user missing required fields", () => {
      const user = {
        id: "usr_1234567890abcdef",
      };
      const result = UserSchema.safeParse(user);
      expect(result.success).toBe(false);
    });
  });

  describe("RoomSchema", () => {
    it("validates a valid room", () => {
      const room = {
        id: "roo_123456789abc",
        name: "general",
        is_private: false,
      };
      const result = RoomSchema.safeParse(room);
      expect(result.success).toBe(true);
    });

    it("rejects room with invalid ID pattern", () => {
      const room = {
        id: "room_bad",
        name: "general",
        is_private: false,
      };
      const result = RoomSchema.safeParse(room);
      expect(result.success).toBe(false);
    });
  });

  describe("MessageSchema", () => {
    it("validates a valid message", () => {
      const message = {
        id: "msg_123456789abc",
        room_id: "roo_123456789abc",
        user_id: "usr_1234567890abcdef",
        username: "testuser",
        body: "Hello, world!",
        created_at: "2024-01-15T10:30:00.123456789Z",
        modified_at: "2024-01-15T10:30:00.123456789Z",
      };
      const result = MessageSchema.safeParse(message);
      expect(result.success).toBe(true);
    });

    it("rejects message with invalid ID", () => {
      const message = {
        id: "bad_id",
        room_id: "roo_123456789abc",
        user_id: "usr_1234567890abcdef",
        username: "testuser",
        body: "Hello!",
        created_at: "2024-01-15T10:30:00Z",
        modified_at: "2024-01-15T10:30:00Z",
      };
      const result = MessageSchema.safeParse(message);
      expect(result.success).toBe(false);
    });
  });

  describe("ServerEnvelopeSchema", () => {
    it("validates init envelope", () => {
      const envelope = {
        type: "init",
        data: {
          User: {
            id: "usr_1234567890abcdef",
            username: "testuser",
          },
          Rooms: [
            {
              id: "roo_123456789abc",
              name: "general",
              is_private: false,
            },
          ],
          current_room: "roo_123456789abc",
        },
      };
      const result = ServerEnvelopeSchema.safeParse(envelope);
      expect(result.success).toBe(true);
      if (result.success) {
        expect(result.data.type).toBe("init");
      }
    });

    it("validates message envelope", () => {
      const envelope = {
        type: "message",
        data: {
          id: "msg_123456789abc",
          room_id: "roo_123456789abc",
          user_id: "usr_1234567890abcdef",
          username: "testuser",
          body: "Hello!",
          created_at: "2024-01-15T10:30:00Z",
          modified_at: "2024-01-15T10:30:00Z",
        },
      };
      const result = ServerEnvelopeSchema.safeParse(envelope);
      expect(result.success).toBe(true);
      if (result.success) {
        expect(result.data.type).toBe("message");
      }
    });

    it("validates history envelope", () => {
      const envelope = {
        type: "history",
        data: {
          messages: [],
          has_more: false,
          next_cursor: "",
        },
      };
      const result = ServerEnvelopeSchema.safeParse(envelope);
      expect(result.success).toBe(true);
    });

    it("validates join_room envelope", () => {
      const envelope = {
        type: "join_room",
        data: {
          room: {
            id: "roo_123456789abc",
            name: "general",
            is_private: false,
          },
        },
      };
      const result = ServerEnvelopeSchema.safeParse(envelope);
      expect(result.success).toBe(true);
    });

    it("validates error envelope", () => {
      const envelope = {
        type: "error",
        data: {
          message: "Something went wrong",
        },
      };
      const result = ServerEnvelopeSchema.safeParse(envelope);
      expect(result.success).toBe(true);
    });

    it("rejects unknown envelope type", () => {
      const envelope = {
        type: "unknown_type",
        data: {},
      };
      const result = ServerEnvelopeSchema.safeParse(envelope);
      expect(result.success).toBe(false);
    });

    it("rejects envelope with mismatched data", () => {
      const envelope = {
        type: "init",
        data: {
          message: "This is error data, not init data",
        },
      };
      const result = ServerEnvelopeSchema.safeParse(envelope);
      expect(result.success).toBe(false);
    });
  });

  describe("parseServerEnvelope", () => {
    it("parses valid envelope", () => {
      const raw = {
        type: "error",
        data: { message: "test error" },
      };
      const result = parseServerEnvelope(raw);
      expect(result.type).toBe("error");
      expect(result.data.message).toBe("test error");
    });

    it("throws on invalid envelope", () => {
      const raw = {
        type: "invalid",
        data: {},
      };
      expect(() => parseServerEnvelope(raw)).toThrow();
    });
  });

  describe("safeParseServerEnvelope", () => {
    it("returns parsed envelope on success", () => {
      const raw = {
        type: "error",
        data: { message: "test error" },
      };
      const result = safeParseServerEnvelope(raw);
      expect(result).not.toBeNull();
      expect(result?.type).toBe("error");
    });

    it("returns null on invalid envelope", () => {
      const raw = {
        type: "invalid",
        data: {},
      };
      const result = safeParseServerEnvelope(raw);
      expect(result).toBeNull();
    });

    it("returns null on non-object input", () => {
      expect(safeParseServerEnvelope("not an object")).toBeNull();
      expect(safeParseServerEnvelope(null)).toBeNull();
      expect(safeParseServerEnvelope(undefined)).toBeNull();
      expect(safeParseServerEnvelope(123)).toBeNull();
    });
  });
});

describe("Zod Type Inference", () => {
  it("infers correct types from schemas", () => {
    // Type-level test - if this compiles, the types are correct
    const user: ReturnType<typeof UserSchema.parse> = {
      id: "usr_1234567890abcdef",
      username: "test",
    };

    const room: ReturnType<typeof RoomSchema.parse> = {
      id: "roo_123456789abc",
      name: "general",
      is_private: false,
    };

    expect(user.id).toMatch(/^usr_/);
    expect(room.id).toMatch(/^roo_/);
  });
});
