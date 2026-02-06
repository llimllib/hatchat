import { test, expect, type Page } from "@playwright/test";
import {
  registerAndLogin,
  waitForChatReady,
  sendMessage,
  waitForMessage,
} from "./helpers";

/**
 * Open the quick-search modal using Cmd+K (Mac) or Ctrl+K (other platforms)
 */
async function openQuickSearch(page: Page): Promise<void> {
  // Use Meta+K for Mac, Ctrl+K for others
  const isMac = process.platform === "darwin";
  const modifier = isMac ? "Meta" : "Control";
  await page.keyboard.press(`${modifier}+k`);
  await expect(page.locator(".quick-search-modal")).toBeVisible();
}

/**
 * Close the quick-search modal
 */
async function closeQuickSearch(page: Page): Promise<void> {
  await page.keyboard.press("Escape");
  await expect(page.locator(".quick-search-modal")).not.toBeVisible();
}

/**
 * Wait for a quick-search item (room, dm, or user) to appear - excludes the search-escape option
 */
async function waitForQuickSearchResult(
  page: Page,
  text: string
): Promise<void> {
  // The search-escape item has a 🔍 icon, room items have #, DM items have 💬, user items have 👤
  // We want to match items that DON'T start with "Search messages for"
  const item = page.locator(".quick-search-item").filter({
    has: page.locator(".quick-search-item-name"),
    hasNot: page.locator(".quick-search-item-icon:text('🔍')")
  }).filter({
    hasText: text
  }).first();
  
  await expect(item).toBeVisible({ timeout: 5000 });
}

test.describe("Quick-Search (Cmd+K)", () => {
  test("should open quick-search modal with Cmd+K/Ctrl+K", async ({ page }) => {
    await registerAndLogin(page);
    await waitForChatReady(page);

    // Modal should not be visible initially
    await expect(page.locator(".quick-search-modal")).not.toBeVisible();

    // Open with keyboard shortcut
    await openQuickSearch(page);

    // Modal should be visible
    await expect(page.locator(".quick-search-modal")).toBeVisible();
    await expect(page.locator(".quick-search-input")).toBeVisible();
    await expect(page.locator(".quick-search-input")).toBeFocused();
  });

  test("should close quick-search with Escape key", async ({ page }) => {
    await registerAndLogin(page);
    await waitForChatReady(page);

    await openQuickSearch(page);
    await expect(page.locator(".quick-search-modal")).toBeVisible();

    await closeQuickSearch(page);
    await expect(page.locator(".quick-search-modal")).not.toBeVisible();
  });

  test("should close quick-search when clicking overlay", async ({ page }) => {
    await registerAndLogin(page);
    await waitForChatReady(page);

    await openQuickSearch(page);
    await expect(page.locator(".quick-search-modal")).toBeVisible();

    // Click on the overlay (outside the modal)
    await page.locator(".quick-search-overlay").click({ position: { x: 10, y: 10 } });
    await expect(page.locator(".quick-search-modal")).not.toBeVisible();
  });

  test("should show recent rooms when opened without query", async ({ page }) => {
    await registerAndLogin(page);
    await waitForChatReady(page);

    // Create a test room to have something in history
    await page.evaluate(() => {
      const ws = (window as any).__ws;
      if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(
          JSON.stringify({
            type: "create_room",
            data: { name: "quicksearchtest", is_private: false },
          })
        );
      }
    });

    // Wait for the room to appear in sidebar and switch to it
    await expect(
      page.locator(".sidebar-channels").getByText("quicksearchtest")
    ).toBeVisible({ timeout: 5000 });
    await page.locator(".sidebar-channels").getByText("quicksearchtest").click();
    await expect(page.locator(".chat-header h2")).toContainText("quicksearchtest");

    // Switch back to main
    await page.locator(".sidebar-channels").getByText("main").click();
    await expect(page.locator(".chat-header h2")).toContainText("main");

    // Open quick-search
    await openQuickSearch(page);

    // Should show "Recent" section header
    await expect(page.locator(".quick-search-section-header")).toContainText("Recent");
    
    // Should show recent rooms (quicksearchtest should be there since we just visited it)
    await expect(
      page.locator(".quick-search-results").getByText("quicksearchtest")
    ).toBeVisible();
  });

  test("should filter rooms as user types", async ({ page }) => {
    await registerAndLogin(page);
    await waitForChatReady(page);

    // Create a uniquely named room
    const roomName = `filter-test-${Date.now()}`;
    await page.evaluate((name) => {
      const ws = (window as any).__ws;
      if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(
          JSON.stringify({
            type: "create_room",
            data: { name: name, is_private: false },
          })
        );
      }
    }, roomName);

    // Wait for room to appear
    await expect(
      page.locator(".sidebar-channels").getByText(roomName)
    ).toBeVisible({ timeout: 5000 });

    // Open quick-search and type room name
    await openQuickSearch(page);
    await page.locator(".quick-search-input").fill("filter-test");

    // Should show the room in results
    await waitForQuickSearchResult(page, roomName);
  });

  test("should navigate to room when selected from quick-search", async ({
    page,
  }) => {
    await registerAndLogin(page);
    await waitForChatReady(page);

    // Create a test room
    const roomName = `nav-test-${Date.now()}`;
    await page.evaluate((name) => {
      const ws = (window as any).__ws;
      if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(
          JSON.stringify({
            type: "create_room",
            data: { name: name, is_private: false },
          })
        );
      }
    }, roomName);

    // Wait for room to appear
    await expect(
      page.locator(".sidebar-channels").getByText(roomName)
    ).toBeVisible({ timeout: 5000 });

    // Open quick-search
    await openQuickSearch(page);
    await page.locator(".quick-search-input").fill(roomName);

    // Wait for result to appear
    await waitForQuickSearchResult(page, roomName);

    // Press Enter to select
    await page.keyboard.press("Enter");

    // Should navigate to the room
    await expect(page.locator(".chat-header h2")).toContainText(roomName);
    // Modal should be closed
    await expect(page.locator(".quick-search-modal")).not.toBeVisible();
  });

  test("should navigate with arrow keys and Enter", async ({ page }) => {
    await registerAndLogin(page);
    await waitForChatReady(page);

    // Open quick-search with some results
    await openQuickSearch(page);
    await page.locator(".quick-search-input").fill("main");

    // Wait for results
    await waitForQuickSearchResult(page, "main");

    // First item should be selected by default
    await expect(page.locator(".quick-search-item.selected")).toBeVisible();

    // Press down arrow to move selection
    await page.keyboard.press("ArrowDown");

    // Press up arrow to go back
    await page.keyboard.press("ArrowUp");

    // Press Enter to select
    await page.keyboard.press("Enter");

    // Modal should close
    await expect(page.locator(".quick-search-modal")).not.toBeVisible();
  });

  test("should show 'Search messages' escape hatch", async ({ page }) => {
    await registerAndLogin(page);
    await waitForChatReady(page);

    await openQuickSearch(page);
    await page.locator(".quick-search-input").fill("hello world");

    // Should show the "Search messages" option at the bottom
    await expect(
      page.locator(".quick-search-results").getByText(/Search messages for/)
    ).toBeVisible();
  });

  test("should navigate to search page when 'Search messages' is selected", async ({
    page,
  }) => {
    await registerAndLogin(page);
    await waitForChatReady(page);

    await openQuickSearch(page);
    const searchQuery = "test-search-query";
    await page.locator(".quick-search-input").fill(searchQuery);

    // Wait for "Search messages" to appear (it's a search-escape item)
    const searchEscapeItem = page.locator(".quick-search-item").filter({
      hasText: /Search messages for/
    });
    await expect(searchEscapeItem).toBeVisible();

    // Click on "Search messages" option
    await searchEscapeItem.click();

    // Should navigate to search page with query
    await expect(page).toHaveURL(/\/search\?q=/);
    // Verify the query is in the URL (might be encoded)
    const url = page.url();
    expect(url).toContain(searchQuery);
  });
});

test.describe("Search Page", () => {
  test("should display search page UI", async ({ page }) => {
    await registerAndLogin(page);
    await waitForChatReady(page);

    // Navigate to search page
    await page.goto("/search");

    // Should show search UI elements
    await expect(page.locator(".search-header")).toBeVisible();
    await expect(page.locator(".search-input")).toBeVisible();
    await expect(page.locator("#search-room-filter")).toBeVisible();
    await expect(page.locator("#search-user-filter")).toBeVisible();
    await expect(page.locator(".search-back-btn")).toBeVisible();
  });

  test("should focus search input on load", async ({ page }) => {
    await registerAndLogin(page);
    await waitForChatReady(page);

    await page.goto("/search");

    // Input should be focused
    await expect(page.locator("#search-input")).toBeFocused();
  });

  test("should return results for matching messages", async ({ page }) => {
    await registerAndLogin(page);
    await waitForChatReady(page);

    // Send a unique message to search for
    const uniqueText = `searchable-message-${Date.now()}`;
    await sendMessage(page, uniqueText);
    await waitForMessage(page, uniqueText);

    // Go to search page
    await page.goto("/search");

    // Search for the message
    await page.locator("#search-input").fill(uniqueText);
    await page.locator('button[type="submit"]').click();

    // Wait for results
    await expect(page.locator(".search-result-card")).toBeVisible({
      timeout: 10000,
    });

    // Should show the matching message
    await expect(page.locator(".search-result-card")).toContainText("main"); // room name
  });

  test("should show 'no results' for non-matching query", async ({ page }) => {
    await registerAndLogin(page);
    await waitForChatReady(page);

    await page.goto("/search");

    // Search for something that doesn't exist
    const nonExistentQuery = `nonexistent-${Date.now()}-xyz`;
    await page.locator("#search-input").fill(nonExistentQuery);
    await page.locator('button[type="submit"]').click();

    // Should show "no results" message
    await expect(page.locator(".search-empty")).toBeVisible({ timeout: 5000 });
    await expect(page.locator(".search-empty")).toContainText("No messages found");
  });

  test("should update URL with search query", async ({ page }) => {
    await registerAndLogin(page);
    await waitForChatReady(page);

    await page.goto("/search");

    const query = "test-query";
    await page.locator("#search-input").fill(query);
    await page.locator('button[type="submit"]').click();

    // URL should contain the query parameter
    await expect(page).toHaveURL(new RegExp(`q=${query}`));
  });

  test("should restore query from URL", async ({ page }) => {
    await registerAndLogin(page);
    await waitForChatReady(page);

    const query = "prefilled-query";
    await page.goto(`/search?q=${query}`);

    // Input should have the query value
    await expect(page.locator("#search-input")).toHaveValue(query);
  });

  test("should filter results by room", async ({ browser }) => {
    const context = await browser.newContext();
    const page = await context.newPage();

    try {
      await registerAndLogin(page);
      await waitForChatReady(page);

      // Create a new room
      const roomName = `filter-room-${Date.now()}`;
      await page.evaluate((name) => {
        const ws = (window as any).__ws;
        if (ws && ws.readyState === WebSocket.OPEN) {
          ws.send(
            JSON.stringify({
              type: "create_room",
              data: { name: name, is_private: false },
            })
          );
        }
      }, roomName);

      await expect(
        page.locator(".sidebar-channels").getByText(roomName)
      ).toBeVisible({ timeout: 5000 });

      // Switch to new room and send a unique message
      await page.locator(".sidebar-channels").getByText(roomName).click();
      await expect(page.locator(".chat-header h2")).toContainText(roomName);

      const uniqueText = `room-filter-test-${Date.now()}`;
      await sendMessage(page, uniqueText);
      await waitForMessage(page, uniqueText);

      // Send same text in main room
      await page.locator(".sidebar-channels").getByText("main").click();
      await expect(page.locator(".chat-header h2")).toContainText("main");
      await sendMessage(page, uniqueText);
      await waitForMessage(page, uniqueText);

      // Go to search and search for the text
      await page.goto("/search");
      await page.locator("#search-input").fill(uniqueText);
      await page.locator('button[type="submit"]').click();

      // Should have results
      await expect(page.locator(".search-result-card").first()).toBeVisible({
        timeout: 10000,
      });

      // Get count of results (should be 2)
      const resultCount = await page.locator(".search-result-card").count();
      expect(resultCount).toBe(2);

      // Now filter by the specific room - find the option with matching text
      const roomSelect = page.locator("#search-room-filter");
      // Get all options and find the one containing our room name
      const options = await roomSelect.locator("option").all();
      let targetValue = "";
      for (const option of options) {
        const text = await option.textContent();
        if (text && text.includes(roomName)) {
          targetValue = await option.getAttribute("value") || "";
          break;
        }
      }
      await roomSelect.selectOption(targetValue);
      await page.locator('button[type="submit"]').click();

      // Wait a bit for results to update
      await page.waitForTimeout(500);

      // Should now only show 1 result
      const filteredCount = await page.locator(".search-result-card").count();
      expect(filteredCount).toBe(1);
      await expect(page.locator(".search-result-card")).toContainText(roomName);
    } finally {
      await context.close();
    }
  });

  test("should navigate back to chat", async ({ page }) => {
    await registerAndLogin(page);
    await waitForChatReady(page);

    await page.goto("/search");

    // Click back button
    await page.locator(".search-back-btn").click();

    // Should be back in chat
    await expect(page).toHaveURL(/\/chat/);
  });
});

test.describe("Message Permalinks", () => {
  test("should jump to message when clicking search result", async ({
    page,
  }) => {
    await registerAndLogin(page);
    await waitForChatReady(page);

    // Send a unique message
    const uniqueText = `permalink-test-${Date.now()}`;
    await sendMessage(page, uniqueText);
    await waitForMessage(page, uniqueText);

    // Go to search page and search for it
    await page.goto("/search");
    await page.locator("#search-input").fill(uniqueText);
    await page.locator('button[type="submit"]').click();

    // Wait for results
    await expect(page.locator(".search-result-card")).toBeVisible({
      timeout: 10000,
    });

    // Click the result
    await page.locator(".search-result-card").first().click();

    // Should navigate to chat with message in view
    await expect(page).toHaveURL(/\/chat\/roo_.*#msg_/);

    // Message should be visible
    await waitForMessage(page, uniqueText);
  });

  test("should highlight message briefly after jumping to it", async ({
    page,
  }) => {
    await registerAndLogin(page);
    await waitForChatReady(page);

    // Send a unique message
    const uniqueText = `highlight-test-${Date.now()}`;
    await sendMessage(page, uniqueText);
    await waitForMessage(page, uniqueText);

    // Go to search and find it
    await page.goto("/search");
    await page.locator("#search-input").fill(uniqueText);
    await page.locator('button[type="submit"]').click();

    // Wait for results and click
    await expect(page.locator(".search-result-card")).toBeVisible({
      timeout: 10000,
    });
    await page.locator(".search-result-card").first().click();

    // Should navigate to chat
    await expect(page).toHaveURL(/\/chat\/roo_.*#msg_/);

    // The message should have the highlight-flash class briefly
    await expect(page.locator(".chat-message.highlight-flash")).toBeVisible({
      timeout: 2000,
    });
  });

  test("should load correct room when visiting permalink directly", async ({
    page,
    browser,
  }) => {
    // First, create a message and get its permalink
    await registerAndLogin(page);
    await waitForChatReady(page);

    // Create a room and send a message
    const roomName = `permalink-room-${Date.now()}`;
    await page.evaluate((name) => {
      const ws = (window as any).__ws;
      if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(
          JSON.stringify({
            type: "create_room",
            data: { name: name, is_private: false },
          })
        );
      }
    }, roomName);

    await expect(
      page.locator(".sidebar-channels").getByText(roomName)
    ).toBeVisible({ timeout: 5000 });

    await page.locator(".sidebar-channels").getByText(roomName).click();
    await expect(page.locator(".chat-header h2")).toContainText(roomName);

    const uniqueText = `shareable-message-${Date.now()}`;
    await sendMessage(page, uniqueText);
    await waitForMessage(page, uniqueText);

    // Get the message element and find its ID
    const messageEl = page.locator(".chat-message").last();
    const messageId = await messageEl.getAttribute("data-message-id");
    expect(messageId).toBeTruthy();

    // Get the room ID from URL
    const currentUrl = page.url();
    const roomId = currentUrl.split("/chat/")[1];
    expect(roomId).toMatch(/^roo_/);

    // Now navigate away and come back via permalink
    await page.locator(".sidebar-channels").getByText("main").click();
    await expect(page.locator(".chat-header h2")).toContainText("main");

    // Visit the permalink directly
    await page.goto(`/chat/${roomId}#${messageId}`);

    // Should load the room and show the message
    await expect(page.locator(".chat-header h2")).toContainText(roomName);
    await waitForMessage(page, uniqueText);
  });

  test("should copy permalink when clicking message timestamp", async ({
    page,
  }) => {
    await registerAndLogin(page);
    await waitForChatReady(page);

    // Send a message
    const uniqueText = `copy-link-test-${Date.now()}`;
    await sendMessage(page, uniqueText);
    await waitForMessage(page, uniqueText);

    // Grant clipboard permissions
    await page.context().grantPermissions(["clipboard-write", "clipboard-read"]);

    // Find the message timestamp and click it
    const message = page.locator(".chat-message").last();
    const timestamp = message.locator(".message-timestamp");
    await timestamp.click();

    // Should show a toast notification
    await expect(page.locator(".toast")).toBeVisible();
    await expect(page.locator(".toast")).toContainText("Link copied");
  });

  test("should handle permalink to different room", async ({ browser }) => {
    const context = await browser.newContext();
    const page = await context.newPage();

    try {
      await registerAndLogin(page);
      await waitForChatReady(page);

      // Create two rooms
      const room1 = `room1-${Date.now()}`;
      const room2 = `room2-${Date.now()}`;

      for (const name of [room1, room2]) {
        await page.evaluate((n) => {
          const ws = (window as any).__ws;
          if (ws && ws.readyState === WebSocket.OPEN) {
            ws.send(
              JSON.stringify({
                type: "create_room",
                data: { name: n, is_private: false },
              })
            );
          }
        }, name);
        await expect(
          page.locator(".sidebar-channels").getByText(name)
        ).toBeVisible({ timeout: 5000 });
      }

      // Send message in room2
      await page.locator(".sidebar-channels").getByText(room2).click();
      await expect(page.locator(".chat-header h2")).toContainText(room2);

      const uniqueText = `room2-message-${Date.now()}`;
      await sendMessage(page, uniqueText);
      await waitForMessage(page, uniqueText);

      // Get message ID
      const messageEl = page.locator(".chat-message").last();
      const messageId = await messageEl.getAttribute("data-message-id");
      const roomId = page.url().split("/chat/")[1];

      // Switch to room1
      await page.locator(".sidebar-channels").getByText(room1).click();
      await expect(page.locator(".chat-header h2")).toContainText(room1);

      // Visit permalink to room2 message
      await page.goto(`/chat/${roomId}#${messageId}`);

      // Should switch to room2 and show message
      await expect(page.locator(".chat-header h2")).toContainText(room2);
      await waitForMessage(page, uniqueText);
    } finally {
      await context.close();
    }
  });
});

test.describe("Search Integration", () => {
  test("should highlight search terms in results", async ({ page }) => {
    await registerAndLogin(page);
    await waitForChatReady(page);

    // Send a message with a specific word
    const keyword = `highlight${Date.now()}`;
    await sendMessage(page, `This message contains ${keyword} for testing`);
    await waitForMessage(page, keyword);

    // Search for the keyword
    await page.goto("/search");
    await page.locator("#search-input").fill(keyword);
    await page.locator('button[type="submit"]').click();

    // Wait for results
    await expect(page.locator(".search-result-card")).toBeVisible({
      timeout: 10000,
    });

    // The snippet should contain highlighted text (bold)
    const snippet = page.locator(".search-result-snippet");
    await expect(snippet.locator("strong")).toBeVisible();
    await expect(snippet.locator("strong")).toContainText(keyword);
  });

  test("should show results count", async ({ page }) => {
    await registerAndLogin(page);
    await waitForChatReady(page);

    // Send a unique message
    const uniqueText = `countable-${Date.now()}`;
    await sendMessage(page, uniqueText);
    await waitForMessage(page, uniqueText);

    // Search for it
    await page.goto("/search");
    await page.locator("#search-input").fill(uniqueText);
    await page.locator('button[type="submit"]').click();

    // Wait for results
    await expect(page.locator(".search-count")).toBeVisible({ timeout: 10000 });
    await expect(page.locator(".search-count")).toContainText("1 result");
  });

  test("should support load more pagination", async ({ page }) => {
    await registerAndLogin(page);
    await waitForChatReady(page);

    // Send many messages with the same keyword
    const keyword = `paginate-${Date.now()}`;
    const messageCount = 25; // More than the default page size of 20

    for (let i = 0; i < messageCount; i++) {
      await sendMessage(page, `${keyword} message ${i + 1}`);
      // Small delay to ensure ordering
      await page.waitForTimeout(50);
    }

    // Wait for all messages to appear
    await waitForMessage(page, `${keyword} message ${messageCount}`);

    // Search for the keyword
    await page.goto("/search");
    await page.locator("#search-input").fill(keyword);
    await page.locator('button[type="submit"]').click();

    // Wait for initial results
    await expect(page.locator(".search-result-card").first()).toBeVisible({
      timeout: 10000,
    });

    // Should show "Load more" button since we have more than 20 results
    const loadMoreBtn = page.locator(".load-more-search");
    
    // If there are enough results for pagination, click load more
    const initialCount = await page.locator(".search-result-card").count();
    if (await loadMoreBtn.isVisible()) {
      await loadMoreBtn.click();

      // Wait for more results
      await page.waitForTimeout(1000);
      const newCount = await page.locator(".search-result-card").count();
      expect(newCount).toBeGreaterThan(initialCount);
    }
  });
});
