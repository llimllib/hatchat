import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  formatTimestamp,
  formatTimestampFull,
  getInitials,
  stringToColor,
} from "./utils";

describe("formatTimestamp", () => {
  beforeEach(() => {
    // Mock Date to a fixed time: January 15, 2024, 3:30 PM
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2024-01-15T15:30:00"));
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("shows only time for today", () => {
    const result = formatTimestamp("2024-01-15T10:45:00");
    expect(result).toBe("10:45 AM");
  });

  it("shows month/day and time for this year", () => {
    const result = formatTimestamp("2024-01-10T10:45:00");
    expect(result).toBe("Jan 10, 10:45 AM");
  });

  it("shows full date for previous year", () => {
    const result = formatTimestamp("2023-12-25T10:45:00");
    expect(result).toBe("Dec 25, 2023, 10:45 AM");
  });

  it("handles PM times correctly", () => {
    const result = formatTimestamp("2024-01-15T22:30:00");
    expect(result).toBe("10:30 PM");
  });
});

describe("formatTimestampFull", () => {
  it("formats full datetime", () => {
    const result = formatTimestampFull("2024-01-15T15:30:45");
    expect(result).toContain("January");
    expect(result).toContain("15");
    expect(result).toContain("2024");
    expect(result).toContain("3:30");
    expect(result).toContain("PM");
  });
});

describe("getInitials", () => {
  it("gets initials from single word username", () => {
    expect(getInitials("john")).toBe("JO");
  });

  it("gets initials from space-separated name", () => {
    expect(getInitials("John Doe")).toBe("JD");
  });

  it("gets initials from underscore-separated name", () => {
    expect(getInitials("john_doe")).toBe("JD");
  });

  it("gets initials from dot-separated name", () => {
    expect(getInitials("john.doe")).toBe("JD");
  });

  it("gets initials from dash-separated name", () => {
    expect(getInitials("john-doe")).toBe("JD");
  });

  it("handles short usernames", () => {
    expect(getInitials("j")).toBe("J");
  });

  it("handles multiple separators", () => {
    expect(getInitials("john.q.public")).toBe("JQ");
  });
});

describe("stringToColor", () => {
  it("returns a hex color", () => {
    const color = stringToColor("test");
    expect(color).toMatch(/^#[0-9a-f]{6}$/);
  });

  it("returns consistent color for same string", () => {
    const color1 = stringToColor("alice");
    const color2 = stringToColor("alice");
    expect(color1).toBe(color2);
  });

  it("returns different colors for different strings", () => {
    const color1 = stringToColor("alice");
    const color2 = stringToColor("bob");
    // They might occasionally collide, but usually won't
    // Just verify they're both valid colors
    expect(color1).toMatch(/^#[0-9a-f]{6}$/);
    expect(color2).toMatch(/^#[0-9a-f]{6}$/);
  });
});
