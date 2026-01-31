import { describe, expect, it } from "vitest";
import { makePendingKey } from "./types";

describe("makePendingKey", () => {
  it("creates a unique key from body, roomId, and userId", () => {
    const key = makePendingKey("hello", "roo_123", "usr_456");
    expect(key).toBe("usr_456:roo_123:hello");
  });

  it("creates different keys for different messages", () => {
    const key1 = makePendingKey("hello", "roo_123", "usr_456");
    const key2 = makePendingKey("world", "roo_123", "usr_456");
    expect(key1).not.toBe(key2);
  });

  it("creates different keys for different rooms", () => {
    const key1 = makePendingKey("hello", "roo_123", "usr_456");
    const key2 = makePendingKey("hello", "roo_789", "usr_456");
    expect(key1).not.toBe(key2);
  });

  it("creates different keys for different users", () => {
    const key1 = makePendingKey("hello", "roo_123", "usr_456");
    const key2 = makePendingKey("hello", "roo_123", "usr_789");
    expect(key1).not.toBe(key2);
  });

  it("handles empty body", () => {
    const key = makePendingKey("", "roo_123", "usr_456");
    expect(key).toBe("usr_456:roo_123:");
  });

  it("handles special characters in body", () => {
    const key = makePendingKey("hello:world", "roo_123", "usr_456");
    expect(key).toBe("usr_456:roo_123:hello:world");
  });
});
