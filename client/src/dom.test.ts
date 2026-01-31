import { describe, expect, it } from "vitest";
import { $, text } from "./dom";

describe("$ helper", () => {
  it("creates an element with the given tag name", () => {
    const el = $("div");
    expect(el.tagName).toBe("DIV");
  });

  it("sets attributes on the element", () => {
    const el = $("div", { class: "test-class", id: "test-id" });
    expect(el.className).toBe("test-class");
    expect(el.id).toBe("test-id");
  });

  it("sets innerText when 'text' attribute is provided", () => {
    const el = $("span", { text: "Hello world" });
    expect(el.innerText).toBe("Hello world");
  });

  it("appends child elements", () => {
    const el = $("div", {}, $("span", { text: "child" }));
    expect(el.children.length).toBe(1);
    expect(el.children[0].tagName).toBe("SPAN");
    expect((el.children[0] as HTMLElement).innerText).toBe("child");
  });

  it("appends text nodes", () => {
    const el = $("div", {}, text("hello"));
    expect(el.textContent).toBe("hello");
  });

  it("handles mixed children", () => {
    const el = $(
      "div",
      {},
      $("span", { text: "Hello" }),
      text(" "),
      $("strong", { text: "world" }),
    );
    // Check structure: two element children and a text node between them
    expect(el.children.length).toBe(2);
    expect(el.children[0].tagName).toBe("SPAN");
    expect(el.children[1].tagName).toBe("STRONG");
    // Check that all three nodes are present (span, text, strong)
    expect(el.childNodes.length).toBe(3);
  });

  it("handles empty attributes", () => {
    const el = $("input", { disabled: "" });
    expect(el.hasAttribute("disabled")).toBe(true);
    expect(el.getAttribute("disabled")).toBe("");
  });
});

describe("text helper", () => {
  it("creates a text node", () => {
    const node = text("hello");
    expect(node.nodeType).toBe(Node.TEXT_NODE);
    expect(node.textContent).toBe("hello");
  });

  it("handles empty string", () => {
    const node = text("");
    expect(node.textContent).toBe("");
  });

  it("handles special characters", () => {
    const node = text("<script>alert('xss')</script>");
    expect(node.textContent).toBe("<script>alert('xss')</script>");
  });
});
