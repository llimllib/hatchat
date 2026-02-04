/**
 * Autocomplete popup for @mentions and #channel references.
 *
 * Monitors a textarea for trigger characters (@, #) and shows a popup
 * with matching suggestions. Selecting a suggestion inserts the text.
 */

import { $ } from "./dom";

export interface AutocompleteItem {
  /** Display label shown in the popup */
  label: string;
  /** Secondary text (e.g., display name for users) */
  secondary?: string;
  /** Text to insert when selected (without the trigger character) */
  value: string;
  /** Type of suggestion */
  type: "user" | "channel";
}

export interface AutocompleteOptions {
  /** The textarea element to attach to */
  input: HTMLTextAreaElement;
  /** Callback when we need user suggestions for @mention */
  onQueryUsers: (query: string) => void;
  /** Callback when user selects a @mention (e.g., for analytics) */
  onSelectUser?: (username: string) => void;
  /** Callback when user selects a #channel */
  onSelectChannel?: (channelName: string) => void;
}

type TriggerType = "user" | "channel";

export class Autocomplete {
  private input: HTMLTextAreaElement;
  private popup: HTMLElement | null = null;
  private items: AutocompleteItem[] = [];
  private selectedIndex = 0;
  private triggerType: TriggerType | null = null;
  private triggerStart = -1; // position of the trigger char in the input
  private onQueryUsers: (query: string) => void;
  private onSelectUser?: (username: string) => void;
  private onSelectChannel?: (channelName: string) => void;

  // Channel data is set externally since we have it in state
  private channelItems: AutocompleteItem[] = [];

  constructor(options: AutocompleteOptions) {
    this.input = options.input;
    this.onQueryUsers = options.onQueryUsers;
    this.onSelectUser = options.onSelectUser;
    this.onSelectChannel = options.onSelectChannel;

    this.input.addEventListener("input", this.onInput.bind(this));
    this.input.addEventListener("keydown", this.onKeydown.bind(this));
    this.input.addEventListener("blur", this.onBlur.bind(this));
  }

  /**
   * Set available channels for #channel autocomplete
   */
  setChannels(channels: { name: string }[]) {
    this.channelItems = channels.map((ch) => ({
      label: `#${ch.name}`,
      value: ch.name,
      type: "channel" as const,
    }));
  }

  /**
   * Update user suggestions from server response
   */
  updateUserSuggestions(users: { username: string; display_name?: string }[]) {
    if (this.triggerType !== "user") return;

    this.items = users.map((u) => ({
      label: `@${u.username}`,
      secondary: u.display_name || undefined,
      value: u.username,
      type: "user" as const,
    }));

    this.selectedIndex = 0;
    this.renderPopup();
  }

  /**
   * Check if autocomplete is currently active
   */
  get isActive(): boolean {
    return this.popup !== null;
  }

  private onInput() {
    const pos = this.input.selectionStart;
    const text = this.input.value;

    // Find if we're in a trigger context
    const trigger = this.findTrigger(text, pos);

    if (!trigger) {
      this.close();
      return;
    }

    this.triggerType = trigger.type;
    this.triggerStart = trigger.start;
    const query = trigger.query;

    if (trigger.type === "user") {
      // Query server for user suggestions
      this.onQueryUsers(query);
      // If query is empty, show a "type to search" state
      if (query.length === 0) {
        this.items = [];
        this.renderPopup();
      }
    } else if (trigger.type === "channel") {
      // Filter channels locally
      const lowerQuery = query.toLowerCase();
      this.items = this.channelItems.filter(
        (item) =>
          query.length === 0 || item.value.toLowerCase().includes(lowerQuery),
      );
      this.selectedIndex = 0;
      this.renderPopup();
    }
  }

  private onKeydown(e: KeyboardEvent) {
    if (!this.isActive || this.items.length === 0) return;

    switch (e.key) {
      case "ArrowDown":
        e.preventDefault();
        this.selectedIndex = (this.selectedIndex + 1) % this.items.length;
        this.updateSelection();
        break;
      case "ArrowUp":
        e.preventDefault();
        this.selectedIndex =
          (this.selectedIndex - 1 + this.items.length) % this.items.length;
        this.updateSelection();
        break;
      case "Enter":
      case "Tab":
        if (this.items.length > 0) {
          e.preventDefault();
          this.selectItem(this.selectedIndex);
        }
        break;
      case "Escape":
        e.preventDefault();
        this.close();
        break;
    }
  }

  private onBlur() {
    // Delay close to allow click on popup items
    setTimeout(() => this.close(), 200);
  }

  /**
   * Find if cursor is in a trigger context (after @ or # with no space between)
   */
  private findTrigger(
    text: string,
    cursorPos: number,
  ): { type: TriggerType; start: number; query: string } | null {
    // Walk backwards from cursor to find trigger character
    let i = cursorPos - 1;
    while (i >= 0) {
      const ch = text[i];

      // Stop at whitespace - no trigger found
      if (/\s/.test(ch)) return null;

      if (ch === "@") {
        // Must be at start or preceded by whitespace
        if (i === 0 || /\s/.test(text[i - 1])) {
          const query = text.slice(i + 1, cursorPos);
          return { type: "user", start: i, query };
        }
        return null;
      }

      if (ch === "#") {
        // Must be at start or preceded by whitespace
        if (i === 0 || /\s/.test(text[i - 1])) {
          const query = text.slice(i + 1, cursorPos);
          return { type: "channel", start: i, query };
        }
        return null;
      }

      i--;
    }

    return null;
  }

  private selectItem(index: number) {
    const item = this.items[index];
    if (!item) return;

    const text = this.input.value;
    const triggerChar = this.triggerType === "user" ? "@" : "#";
    const cursorPos = this.input.selectionStart;

    // Replace the trigger + query with the completed mention
    const before = text.slice(0, this.triggerStart);
    const after = text.slice(cursorPos);
    const insertion = `${triggerChar}${item.value} `;
    this.input.value = before + insertion + after;

    // Move cursor to after the insertion
    const newPos = this.triggerStart + insertion.length;
    this.input.setSelectionRange(newPos, newPos);

    // Fire callbacks
    if (item.type === "user" && this.onSelectUser) {
      this.onSelectUser(item.value);
    } else if (item.type === "channel" && this.onSelectChannel) {
      this.onSelectChannel(item.value);
    }

    // Fire input event so auto-resize works
    this.input.dispatchEvent(new Event("input", { bubbles: true }));

    this.close();
  }

  private renderPopup() {
    if (!this.popup) {
      this.popup = $("div", { class: "autocomplete-popup" });
      document.body.appendChild(this.popup);
    }

    this.popup.innerHTML = "";

    if (this.items.length === 0) {
      const hint = $("div", { class: "autocomplete-hint" });
      hint.textContent =
        this.triggerType === "user"
          ? "Type a username..."
          : "No matching channels";
      this.popup.appendChild(hint);
    } else {
      for (let i = 0; i < this.items.length; i++) {
        const item = this.items[i];
        const el = $("div", {
          class: `autocomplete-item ${i === this.selectedIndex ? "selected" : ""}`,
        });

        const labelSpan = $("span", {
          class: "autocomplete-label",
          text: item.label,
        });
        el.appendChild(labelSpan);

        if (item.secondary) {
          const secondarySpan = $("span", {
            class: "autocomplete-secondary",
            text: item.secondary,
          });
          el.appendChild(secondarySpan);
        }

        el.addEventListener("mousedown", (e) => {
          e.preventDefault(); // Prevent blur
          this.selectItem(i);
        });

        el.addEventListener("mouseenter", () => {
          this.selectedIndex = i;
          this.updateSelection();
        });

        this.popup.appendChild(el);
      }
    }

    // Position the popup above the input
    this.positionPopup();
  }

  private positionPopup() {
    if (!this.popup) return;

    const inputRect = this.input.getBoundingClientRect();

    // Position above the input, aligned to the left
    this.popup.style.position = "fixed";
    this.popup.style.bottom = `${window.innerHeight - inputRect.top + 4}px`;
    this.popup.style.left = `${inputRect.left}px`;
    this.popup.style.width = `${Math.min(inputRect.width, 300)}px`;
  }

  private updateSelection() {
    if (!this.popup) return;

    const items = this.popup.querySelectorAll(".autocomplete-item");
    for (let i = 0; i < items.length; i++) {
      items[i].classList.toggle("selected", i === this.selectedIndex);
    }
  }

  close() {
    if (this.popup) {
      this.popup.remove();
      this.popup = null;
    }
    this.triggerType = null;
    this.triggerStart = -1;
    this.items = [];
    this.selectedIndex = 0;
  }

  /**
   * Clean up event listeners
   */
  destroy() {
    this.close();
    this.input.removeEventListener("input", this.onInput.bind(this));
    this.input.removeEventListener("keydown", this.onKeydown.bind(this));
    this.input.removeEventListener("blur", this.onBlur.bind(this));
  }
}
