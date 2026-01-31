/**
 * DOM helper function for creating elements with attributes and children.
 *
 * @example
 * $("div", { class: "message" },
 *   $("span", { text: "Hello" }),
 *   text(" world")
 * )
 */
export function $(
  tagName: string,
  attributes?: Record<string, string>,
  ...children: (HTMLElement | Text)[]
): HTMLElement {
  const elt = document.createElement(tagName);
  if (attributes) {
    for (const [key, val] of Object.entries(attributes)) {
      switch (key) {
        case "text":
          elt.innerText = val;
          break;
        default:
          elt.setAttribute(key, val || "");
      }
    }
  }
  for (const child of children) {
    elt.appendChild(child);
  }

  return elt;
}

/**
 * Create a text node.
 */
export function text(s: string): Text {
  return document.createTextNode(s);
}
