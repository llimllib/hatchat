import { test, expect, type Page } from "@playwright/test";
import { registerAndLogin, waitForChatReady, sendMessage, waitForMessage } from "./helpers";

test.describe("WebSocket Reconnection", () => {
  test("shows reconnecting status when connection is lost", async ({ page }) => {
    await registerAndLogin(page);
    await waitForChatReady(page);

    // Close the WebSocket to simulate a disconnection
    await page.evaluate(() => {
      // biome-ignore lint/suspicious/noExplicitAny: e2e test
      const ws = (window as any).__ws as WebSocket;
      // Use code 1006 to simulate an abnormal closure (not 1000 which is clean)
      ws.close();
    });

    // Should see the reconnecting indicator
    await expect(page.locator("#connection-status")).toBeVisible({ timeout: 2000 });
    await expect(page.locator("#connection-status")).toContainText("Reconnecting");
  });

  test("reconnects automatically after disconnection", async ({ page }) => {
    await registerAndLogin(page);
    await waitForChatReady(page);

    // Send a message before disconnection
    await sendMessage(page, "Message before disconnect");
    await waitForMessage(page, "Message before disconnect");

    // Close the WebSocket to simulate a disconnection
    await page.evaluate(() => {
      // biome-ignore lint/suspicious/noExplicitAny: e2e test
      const ws = (window as any).__ws as WebSocket;
      ws.close();
    });

    // Wait for reconnection (should happen within a few seconds)
    // The reconnecting indicator should appear then disappear
    await expect(page.locator("#connection-status")).toBeVisible({ timeout: 2000 });

    // Wait for reconnection to complete (indicator should disappear)
    await expect(page.locator("#connection-status")).not.toBeVisible({ timeout: 10000 });

    // After reconnection, we should still be able to send messages
    await sendMessage(page, "Message after reconnect");
    await waitForMessage(page, "Message after reconnect");
  });

  test("does not show reconnecting status on page refresh", async ({ page }) => {
    await registerAndLogin(page);
    await waitForChatReady(page);

    // Get the current URL
    const currentUrl = page.url();

    // Refresh the page (this should be a clean close via page unload)
    await page.reload();

    // Wait for chat to be ready again
    await waitForChatReady(page);

    // Should not see any reconnecting indicator (page refresh = clean reload, not reconnection)
    await expect(page.locator("#connection-status")).not.toBeVisible();
  });
});
