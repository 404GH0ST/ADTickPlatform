import { expect, test, type APIRequestContext } from "@playwright/test";

import { mockApiBaseUrl, participantStorageState, resetMockApi } from "./test-utils";

/*
 * The neubrutalism theme replaces the shell, home, scoreboard, services, flag
 * form and login views. These tests drive the real participant features
 * through the redesigned views to prove they still work end to end.
 */

async function useTheme(request: APIRequestContext, themeID: string) {
  const response = await request.put(`${mockApiBaseUrl}/api/v2/admin/platform/appearance`, {
    data: { theme_id: themeID },
  });
  expect(response.ok()).toBeTruthy();
}

test.use({ viewport: { width: 1280, height: 900 } });

test.beforeEach(async ({ request }) => {
  await resetMockApi(request);
  await request.post(`${mockApiBaseUrl}/api/v2/admin/game/match/start`);
  await useTheme(request, "neubrutalism");
});

test.describe("as a participant", () => {
  test.use({ storageState: participantStorageState });

  test("home and scoreboard render the theme's views", async ({ page }) => {
    await page.goto("/");
    await expect(page.getByTestId("nb-overview")).toBeVisible();
    await page.getByTestId("nb-overview").getByRole("link", { name: /Scoreboard/ }).click();

    await expect(page).toHaveURL(/\/scoreboard$/);
    await expect(page.getByTestId("nb-scoreboard")).toBeVisible();
    await expect(page.getByTestId("participant-scoreboard-card")).toHaveCount(0);
    await expect(page.getByTestId("nb-score-row").first()).toBeVisible();
    await expect(page.locator('[data-current-team="true"]')).toHaveCount(1);
  });

  test("services features work through the redesigned view", async ({ page }) => {
    await page.goto("/services");
    const card = page.getByTestId("nb-service-svc-1");
    await expect(card).toBeVisible();
    await expect(page.getByTestId("service-card-svc-1")).toHaveCount(0);
    await expect(card.getByRole("link", { name: "Download Source" })).toHaveAttribute(
      "href",
      "/api/platform/challenges/1/source",
    );
    await expect(card.getByRole("button", { name: "SSH Locked" })).toBeDisabled();

    // Flag submission (themed form) unlocks SSH.
    await page.getByRole("textbox", { name: "Flags", exact: true }).fill("PLAYIT{enemy-challenge-1}");
    await page.getByRole("button", { name: "Submit", exact: true }).click();
    await expect(page.locator('[data-testid="flag-verdict"][data-verdict="accepted"]')).toBeVisible();
    await expect(card.getByRole("button", { name: "SSH Access" })).toBeEnabled();

    // SSH session dialog (stock, restyled): credentials are requested on open by the shared logic.
    await card.getByRole("button", { name: "SSH Access" }).click();
    const sshDialog = page.getByRole("dialog", { name: "SSH Access" });
    await expect(sshDialog.getByText("ssh root@10.80.50.11 -p 22").first()).toBeVisible();
    await sshDialog.getByRole("button", { name: "Close" }).first().click();

    // Restart and its cooldown.
    const notice = card.getByTestId(/^service-action-notice-/);
    await card.getByRole("button", { name: /^Restart/ }).click();
    await expect(notice).toContainText("Restart complete");

    // Factory reset through the confirmation dialog, then cooldown.
    await card.getByRole("button", { name: /^Factory Reset/ }).click();
    const resetDialog = page.getByRole("dialog", { name: "Factory Reset Service" });
    await resetDialog.getByRole("button", { name: "Confirm Reset" }).click();
    await expect(notice).toContainText("Factory reset complete");
    await expect(card.getByText(/factory reset cooldown: \d+s/)).toBeVisible();
  });

  test("theme default mode applies until the participant picks one", async ({ page }) => {
    await page.emulateMedia({ colorScheme: "dark" });
    await page.goto("/scoreboard");
    // Neubrutalism defaults to light even on a dark OS.
    await expect(page.locator("html")).toHaveAttribute("data-theme", "light");

    await page.locator(".mode-picker summary").click();
    await page.locator('[data-mode-option="dark"]').click();
    await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
    await page.reload();
    await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  });
});

test("organizer tabs keep stock content inside the theme's shell", async ({ page }) => {
  // Default storage state is the organizer session.
  await page.goto("/admin/game?view=settings");
  await expect(page.getByTestId("nb-shell")).toBeVisible();
  await expect(page.getByTestId("theme-settings-card")).toBeVisible();

  await page.goto("/admin");
  await expect(page.getByTestId("trusted-state-panel")).toBeVisible();
  const announcements = page.getByTestId("announcements-panel");
  await announcements.locator("textarea").fill("Theme test: warmup in 5 minutes.");
  await announcements.getByRole("button", { name: "Publish" }).click();
  await expect(announcements.getByText("Announcement published to participants.")).toBeVisible();
  await expect(announcements.getByText("Theme test: warmup in 5 minutes.")).toBeVisible();
});

test("login works through the redesigned form", async ({ browser }) => {
  const context = await browser.newContext({ storageState: { cookies: [], origins: [] } });
  const page = await context.newPage();
  await page.goto("/login");
  await expect(page.getByTestId("nb-login")).toBeVisible();
  await page.getByLabel("Email").fill("alpha.captain@college.local");
  await page.getByLabel("Password").fill("alpha-password");
  await page.locator('form button[type="submit"]').click();
  await expect(page).toHaveURL(/\/services$/);
  await expect(page.getByTestId("nb-services")).toBeVisible();
  await context.close();
});
