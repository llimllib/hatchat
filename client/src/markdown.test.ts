import { describe, expect, it } from "vitest";
import { containsMention, renderMarkdown } from "./markdown";

describe("renderMarkdown", () => {
  it("renders basic markdown", () => {
    const html = renderMarkdown("**bold** and *italic*");
    expect(html).toContain("<strong>bold</strong>");
    expect(html).toContain("<em>italic</em>");
  });

  it("renders inline code", () => {
    const html = renderMarkdown("use `console.log`");
    expect(html).toContain("<code>console.log</code>");
  });
});

describe("mention rendering", () => {
  it("renders @username as a mention span", () => {
    const html = renderMarkdown("hello @alice how are you?");
    expect(html).toContain('class="mention mention-user"');
    expect(html).toContain('data-mention-type="user"');
    expect(html).toContain('data-mention-name="alice"');
    expect(html).toContain("@alice");
  });

  it("renders #channel as a mention span", () => {
    const html = renderMarkdown("check out #general for updates");
    expect(html).toContain('class="mention mention-channel"');
    expect(html).toContain('data-mention-type="channel"');
    expect(html).toContain('data-mention-name="general"');
    expect(html).toContain("#general");
  });

  it("renders @username at start of message", () => {
    const html = renderMarkdown("@bob look at this");
    expect(html).toContain('data-mention-name="bob"');
  });

  it("renders #channel at start of message", () => {
    const html = renderMarkdown("#random is fun");
    expect(html).toContain('data-mention-name="random"');
  });

  it("does not render mentions inside inline code", () => {
    const html = renderMarkdown("use `@admin` to mention");
    // Inside code, @admin should not be a mention span
    expect(html).not.toContain('data-mention-type="user"');
    expect(html).toContain("@admin");
  });

  it("does not render mentions inside code blocks", () => {
    const html = renderMarkdown("```\n@admin\n```");
    expect(html).not.toContain('data-mention-type="user"');
  });

  it("renders multiple mentions in one message", () => {
    const html = renderMarkdown("@alice and @bob should check #general");
    expect(html).toContain('data-mention-name="alice"');
    expect(html).toContain('data-mention-name="bob"');
    expect(html).toContain('data-mention-name="general"');
  });

  it("handles usernames with hyphens and underscores", () => {
    const html = renderMarkdown("hey @user-name_123");
    expect(html).toContain('data-mention-name="user-name_123"');
  });

  it("handles channel names with hyphens", () => {
    const html = renderMarkdown("see #my-channel");
    expect(html).toContain('data-mention-name="my-channel"');
  });

  it("does not match @ in the middle of a word", () => {
    const html = renderMarkdown("email@example.com");
    expect(html).not.toContain('data-mention-type="user"');
  });
});

describe("containsMention", () => {
  it("detects @username mention", () => {
    expect(containsMention("hello @alice", "alice")).toBe(true);
  });

  it("detects @username at start", () => {
    expect(containsMention("@alice look", "alice")).toBe(true);
  });

  it("detects @username at end", () => {
    expect(containsMention("hey @alice", "alice")).toBe(true);
  });

  it("detects @room mention", () => {
    expect(containsMention("@room please review", "anyuser")).toBe(true);
  });

  it("does not match partial username", () => {
    expect(containsMention("hello @alicewonderland", "alice")).toBe(false);
  });

  it("does not match different username", () => {
    expect(containsMention("hello @bob", "alice")).toBe(false);
  });

  it("is case insensitive", () => {
    expect(containsMention("hello @Alice", "alice")).toBe(true);
  });

  it("handles @username followed by punctuation", () => {
    expect(containsMention("hello @alice!", "alice")).toBe(true);
    expect(containsMention("hello @alice,", "alice")).toBe(true);
    expect(containsMention("(@alice)", "alice")).toBe(true);
  });
});
