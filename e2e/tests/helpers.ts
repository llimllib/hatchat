import { type Page, expect } from "@playwright/test";

/**
 * Generate a unique username for testing to avoid conflicts between test runs
 */
export function generateUsername(): string {
  return `testuser_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`;
}

/**
 * Register a new user and return their credentials
 */
export async function registerUser(
  page: Page,
  username?: string,
  password?: string
): Promise<{ username: string; password: string }> {
  const user = username || generateUsername();
  const pass = password || "testpassword123";

  await page.goto("/");

  // Find the registration form (second form on the page)
  const registerForm = page.locator('form[action="/register"]');

  await registerForm.locator('input[name="username"]').fill(user);
  await registerForm.locator('input[name="password"]').fill(pass);
  await registerForm.locator('button[type="submit"]').click();

  // After registration, we're redirected back to home page
  await page.waitForURL("/");

  return { username: user, password: pass };
}

/**
 * Log in with existing credentials
 */
export async function login(
  page: Page,
  username: string,
  password: string
): Promise<void> {
  await page.goto("/");

  // Find the login form (first form on the page)
  const loginForm = page.locator('form[action="/login"]');

  await loginForm.locator('input[name="username"]').fill(username);
  await loginForm.locator('input[name="password"]').fill(password);
  await loginForm.locator('button[type="submit"]').click();

  // After login, we should be redirected to the chat
  await page.waitForURL(/\/chat\//);
}

/**
 * Register and immediately log in
 */
export async function registerAndLogin(
  page: Page,
  username?: string,
  password?: string
): Promise<{ username: string; password: string }> {
  const creds = await registerUser(page, username, password);
  await login(page, creds.username, creds.password);
  return creds;
}

/**
 * Wait for WebSocket connection and chat to be ready
 */
export async function waitForChatReady(page: Page): Promise<void> {
  // Wait for the chat area to load and show the room name (not "Loading...")
  await expect(page.locator(".chat-header h2")).not.toHaveText("Loading...", {
    timeout: 10000,
  });
}

/**
 * Send a message in the current chat
 */
export async function sendMessage(page: Page, message: string): Promise<void> {
  const input = page.locator("#message");
  await input.fill(message);
  await input.press("Enter");
}

/**
 * Wait for a specific message to appear in the chat
 */
export async function waitForMessage(
  page: Page,
  messageText: string,
  timeout = 5000
): Promise<void> {
  await expect(
    page.locator(".chat-messages").getByText(messageText)
  ).toBeVisible({ timeout });
}

/**
 * Get the current room name from the chat header
 */
export async function getCurrentRoomName(page: Page): Promise<string> {
  return (await page.locator(".chat-header h2").textContent()) || "";
}

/**
 * Click on a room in the sidebar to switch to it
 */
export async function switchToRoom(page: Page, roomName: string): Promise<void> {
  await page.locator(".sidebar-channels").getByText(roomName).click();
  // Wait for the room to load
  await expect(page.locator(".chat-header h2")).toContainText(roomName);
}

/**
 * Create a new room via the UI
 */
export async function createRoom(
  page: Page,
  roomName: string,
  isPrivate = false
): Promise<void> {
  // Click the create room button (assuming there's a "+" or "Create Channel" button)
  // This depends on the actual UI implementation
  await page.getByRole("button", { name: /create|add/i }).click();

  // Fill in the room name in the modal/dialog
  await page.locator('input[name="room-name"]').fill(roomName);

  if (isPrivate) {
    await page.locator('input[name="is-private"]').check();
  }

  // Submit the form
  await page.getByRole("button", { name: /create/i }).click();

  // Wait for the room to appear in the sidebar
  await expect(
    page.locator(".sidebar-channels").getByText(roomName)
  ).toBeVisible();
}
