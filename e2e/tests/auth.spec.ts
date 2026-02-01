import { test, expect } from "@playwright/test";
import {
  generateUsername,
  registerUser,
  login,
  registerAndLogin,
  waitForChatReady,
} from "./helpers";

test.describe("Authentication", () => {
  test("should display login and registration forms on home page", async ({
    page,
  }) => {
    await page.goto("/");

    // Check login form exists
    const loginForm = page.locator('form[action="/login"]');
    await expect(loginForm).toBeVisible();
    await expect(
      loginForm.locator('input[name="username"]')
    ).toBeVisible();
    await expect(
      loginForm.locator('input[name="password"]')
    ).toBeVisible();

    // Check registration form exists
    const registerForm = page.locator('form[action="/register"]');
    await expect(registerForm).toBeVisible();
    await expect(
      registerForm.locator('input[name="username"]')
    ).toBeVisible();
    await expect(
      registerForm.locator('input[name="password"]')
    ).toBeVisible();
  });

  test("should register a new user successfully", async ({ page }) => {
    const username = generateUsername();
    const password = "testpassword123";

    await page.goto("/");

    const registerForm = page.locator('form[action="/register"]');
    await registerForm.locator('input[name="username"]').fill(username);
    await registerForm.locator('input[name="password"]').fill(password);
    await registerForm.locator('button[type="submit"]').click();

    // After registration, user is redirected back to home page
    await page.waitForURL("/");

    // Now login should work
    const loginForm = page.locator('form[action="/login"]');
    await loginForm.locator('input[name="username"]').fill(username);
    await loginForm.locator('input[name="password"]').fill(password);
    await loginForm.locator('button[type="submit"]').click();

    // Should be redirected to chat
    await page.waitForURL(/\/chat\//);
  });

  test("should login with valid credentials", async ({ page }) => {
    // First register
    const { username, password } = await registerUser(page);

    // Then login
    await login(page, username, password);

    // Verify we're in the chat
    await expect(page).toHaveURL(/\/chat\//);
    await waitForChatReady(page);
  });

  test("should redirect to home on invalid login", async ({ page }) => {
    await page.goto("/");

    const loginForm = page.locator('form[action="/login"]');
    await loginForm.locator('input[name="username"]').fill("nonexistent");
    await loginForm.locator('input[name="password"]').fill("wrongpassword");
    await loginForm.locator('button[type="submit"]').click();

    // Should stay on home page (redirected back)
    await page.waitForURL("/");
    await expect(page.locator('form[action="/login"]')).toBeVisible();
  });

  test("should deny unauthenticated users access to chat", async ({
    page,
  }) => {
    // Try to access chat directly without being logged in
    const response = await page.goto("/chat/roo_somefakeroom");

    // Should receive 401 Unauthorized
    expect(response?.status()).toBe(401);
  });

  test("should persist session across page reloads", async ({ page }) => {
    // Register and login
    await registerAndLogin(page);
    await waitForChatReady(page);

    // Remember the current URL
    const chatUrl = page.url();

    // Reload the page
    await page.reload();

    // Should still be in the chat (session persisted)
    await expect(page).toHaveURL(chatUrl);
    await waitForChatReady(page);
  });
});
