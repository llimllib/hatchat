/**
 * Markdown rendering pipeline for chat messages.
 * Message body → mention transform → marked.parse() → DOMPurify.sanitize() → safe HTML
 */

import DOMPurify from "dompurify";
import hljs from "highlight.js/lib/core";
// Import specific languages to minimize bundle size
import bash from "highlight.js/lib/languages/bash";
import css from "highlight.js/lib/languages/css";
import go from "highlight.js/lib/languages/go";
import javascript from "highlight.js/lib/languages/javascript";
import json from "highlight.js/lib/languages/json";
import python from "highlight.js/lib/languages/python";
import rust from "highlight.js/lib/languages/rust";
import sql from "highlight.js/lib/languages/sql";
import typescript from "highlight.js/lib/languages/typescript";
import xml from "highlight.js/lib/languages/xml";
import yaml from "highlight.js/lib/languages/yaml";
import { Marked } from "marked";

// Register languages
hljs.registerLanguage("bash", bash);
hljs.registerLanguage("shell", bash);
hljs.registerLanguage("css", css);
hljs.registerLanguage("go", go);
hljs.registerLanguage("javascript", javascript);
hljs.registerLanguage("js", javascript);
hljs.registerLanguage("json", json);
hljs.registerLanguage("python", python);
hljs.registerLanguage("py", python);
hljs.registerLanguage("rust", rust);
hljs.registerLanguage("sql", sql);
hljs.registerLanguage("typescript", typescript);
hljs.registerLanguage("ts", typescript);
hljs.registerLanguage("html", xml);
hljs.registerLanguage("xml", xml);
hljs.registerLanguage("yaml", yaml);
hljs.registerLanguage("yml", yaml);

// Configure marked
const marked = new Marked({
  breaks: true, // Convert \n to <br> (chat-friendly)
  gfm: true, // GitHub Flavored Markdown
  renderer: {
    // Open links in new tab
    link({ href, title, text }) {
      const titleAttr = title ? ` title="${title}"` : "";
      return `<a href="${href}"${titleAttr} target="_blank" rel="noopener noreferrer">${text}</a>`;
    },
    // Syntax highlighting for code blocks
    code({ text, lang }) {
      if (lang && hljs.getLanguage(lang)) {
        const highlighted = hljs.highlight(text, { language: lang }).value;
        return `<pre><code class="hljs language-${lang}">${highlighted}</code></pre>`;
      }
      // Auto-detect language
      const result = hljs.highlightAuto(text);
      const langClass = result.language ? ` language-${result.language}` : "";
      return `<pre><code class="hljs${langClass}">${result.value}</code></pre>`;
    },
  },
});

// Configure DOMPurify to allow safe HTML from Markdown
const purifyConfig = {
  ALLOWED_TAGS: [
    "a",
    "b",
    "blockquote",
    "br",
    "code",
    "del",
    "em",
    "h1",
    "h2",
    "h3",
    "h4",
    "h5",
    "h6",
    "hr",
    "i",
    "li",
    "ol",
    "p",
    "pre",
    "s",
    "span",
    "strong",
    "sub",
    "sup",
    "table",
    "tbody",
    "td",
    "th",
    "thead",
    "tr",
    "ul",
  ],
  ALLOWED_ATTR: [
    "class",
    "href",
    "rel",
    "target",
    "title",
    "data-mention-type",
    "data-mention-name",
  ],
};

/**
 * Transform @username and #channel-name mentions into styled spans.
 * Must be called BEFORE markdown parsing to avoid interfering with code blocks.
 *
 * We use placeholder tokens that survive markdown parsing, then replace them after.
 */

// Regex for @username mentions: @ followed by word chars (letters, digits, underscores, hyphens)
// Must be at start of string or preceded by whitespace/punctuation (not inside a word)
const MENTION_RE = /(?:^|(?<=[\s(]))@([a-zA-Z0-9_-]+)/g;

// Regex for #channel references: # followed by word chars and hyphens
// Must be at start of string or preceded by whitespace/punctuation
const CHANNEL_RE = /(?:^|(?<=[\s(]))#([a-zA-Z0-9_-]+)/g;

/**
 * Convert @username and #channel in text to HTML spans with data attributes.
 * These survive the markdown + DOMPurify pipeline and are used for click handlers.
 */
function transformMentions(text: string): string {
  // Don't transform inside code blocks - split on backtick-fenced and inline code
  const parts = text.split(/(```[\s\S]*?```|`[^`]+`)/g);
  for (let i = 0; i < parts.length; i++) {
    // Odd indices are code blocks/inline code - skip them
    if (i % 2 === 1) continue;

    parts[i] = parts[i]
      .replace(
        MENTION_RE,
        '<span class="mention mention-user" data-mention-type="user" data-mention-name="$1">@$1</span>',
      )
      .replace(
        CHANNEL_RE,
        '<span class="mention mention-channel" data-mention-type="channel" data-mention-name="$1">#$1</span>',
      );
  }
  return parts.join("");
}

/**
 * Check if a message body contains an @mention of the given username.
 */
export function containsMention(body: string, username: string): boolean {
  // Check for @username or @room
  const re = new RegExp(`(?:^|[\\s(])@(${username}|room)(?=[\\s,.)!?]|$)`, "i");
  return re.test(body);
}

/**
 * Render a Markdown string to sanitized HTML.
 * Returns safe HTML that can be set as innerHTML.
 */
export function renderMarkdown(text: string): string {
  // Transform mentions before markdown parsing
  const withMentions = transformMentions(text);
  const html = marked.parse(withMentions) as string;
  return DOMPurify.sanitize(html, purifyConfig);
}
