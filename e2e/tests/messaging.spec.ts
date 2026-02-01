import { test, expect, type Page, type BrowserContext } from "@playwright/test";
import {
  registerAndLogin,
  waitForChatReady,
  sendMessage,
  waitForMessage,
  generateUsername,
  login,
  registerUser,
} from "./helpers";

test.describe("Messaging", () => {
  test("should send a message and see it appear", async ({ page }) => {
    await registerAndLogin(page);
    await waitForChatReady(page);

    const message = `Hello, world! ${Date.now()}`;
    await sendMessage(page, message);

    // Message should appear in the chat
    await waitForMessage(page, message);
  });

  test("should display sender username with message", async ({ page }) => {
    const { username } = await registerAndLogin(page);
    await waitForChatReady(page);

    const message = `Test message ${Date.now()}`;
    await sendMessage(page, message);

    // Both the message and username should be visible
    await waitForMessage(page, message);
    // The username should appear somewhere near the message
    await expect(
      page.locator(".chat-messages").getByText(username)
    ).toBeVisible();
  });

  test("should receive messages from other users in real-time", async ({
    browser,
  }) => {
    // Create two browser contexts (like two different browsers)
    const context1 = await browser.newContext();
    const context2 = await browser.newContext();
    const page1 = await context1.newPage();
    const page2 = await context2.newPage();

    try {
      // Register and login two different users
      const user1 = await registerAndLogin(page1);
      await waitForChatReady(page1);

      const user2 = await registerAndLogin(page2);
      await waitForChatReady(page2);

      // Both should be in the default "main" room
      await expect(page1.locator(".chat-header h2")).toContainText("main");
      await expect(page2.locator(".chat-header h2")).toContainText("main");

      // User 1 sends a message
      const message = `Hello from user1! ${Date.now()}`;
      await sendMessage(page1, message);

      // User 2 should receive it in real-time
      await waitForMessage(page2, message);
      // And should see user1's username
      await expect(
        page2.locator(".chat-messages").getByText(user1.username)
      ).toBeVisible();

      // User 2 sends a reply
      const reply = `Reply from user2! ${Date.now()}`;
      await sendMessage(page2, reply);

      // User 1 should receive it
      await waitForMessage(page1, reply);
      await expect(
        page1.locator(".chat-messages").getByText(user2.username)
      ).toBeVisible();
    } finally {
      await context1.close();
      await context2.close();
    }
  });

  test("should show message immediately after sending (optimistic update)", async ({
    page,
  }) => {
    await registerAndLogin(page);
    await waitForChatReady(page);

    const message = `Optimistic message ${Date.now()}`;

    // Type the message
    const input = page.locator("#message");
    await input.fill(message);

    // Before pressing enter, message should not be visible
    await expect(
      page.locator(".chat-messages").getByText(message)
    ).not.toBeVisible();

    // Press enter
    await input.press("Enter");

    // Message should appear immediately (optimistic update)
    // Using a short timeout to verify it's truly optimistic
    await expect(
      page.locator(".chat-messages").getByText(message)
    ).toBeVisible({ timeout: 1000 });
  });

  test("should clear input after sending message", async ({ page }) => {
    await registerAndLogin(page);
    await waitForChatReady(page);

    const message = `Test message ${Date.now()}`;
    const input = page.locator("#message");

    await input.fill(message);
    await input.press("Enter");

    // Input should be cleared
    await expect(input).toHaveValue("");
  });

  test("should persist messages after page reload (message history)", async ({
    page,
  }) => {
    await registerAndLogin(page);
    await waitForChatReady(page);

    // Send a unique message
    const message = `Persistent message ${Date.now()}`;
    await sendMessage(page, message);
    await waitForMessage(page, message);

    // Reload the page
    await page.reload();
    await waitForChatReady(page);

    // Message should still be there (loaded from history)
    await waitForMessage(page, message);
  });

  test("should handle sending multiple messages in quick succession", async ({
    page,
  }) => {
    await registerAndLogin(page);
    await waitForChatReady(page);

    const messages = [
      `First message ${Date.now()}`,
      `Second message ${Date.now()}`,
      `Third message ${Date.now()}`,
    ];

    // Send all messages quickly
    for (const msg of messages) {
      await sendMessage(page, msg);
    }

    // All messages should appear in order
    for (const msg of messages) {
      await waitForMessage(page, msg);
    }
  });
});
