import { expect } from "@playwright/test";

import { adminTest as test } from "./test-utils";

test("organizer theme applies platform-wide and keeps participant mode", async ({ page }) => {
  await page.emulateMedia({ colorScheme: "light" });
  await page.goto("/admin/game?view=settings");

  const card = page.getByTestId("theme-settings-card");
  await expect(card).toBeVisible();
  await expect(page.locator("html")).toHaveAttribute("data-scheme", "graphite");
  await expect(card.locator('[data-theme-option="graphite"]')).toHaveAttribute("aria-checked", "true");

  // Participant-side mode choice is independent of the organizer theme.
  await page.locator(".mode-picker summary").click();
  await page.locator('[data-mode-option="dark"]').click();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");

  await card.locator('[data-theme-option="paper"]').click();
  await expect(page.locator("html")).toHaveAttribute("data-scheme", "paper");
  await expect(card.locator('[data-theme-option="paper"]')).toHaveAttribute("aria-checked", "true");
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");

  // Server-rendered on every page, including after a full reload.
  await page.goto("/scoreboard");
  await expect(page.locator("html")).toHaveAttribute("data-scheme", "paper");
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
});

test("a theme can replace the page layout", async ({ page }) => {
  await page.goto("/admin/game?view=settings");
  await expect(page.getByTestId("nb-shell")).toHaveCount(0);

  await page.getByTestId("theme-settings-card").locator('[data-theme-option="neubrutalism"]').click();
  await expect(page.locator("html")).toHaveAttribute("data-scheme", "neubrutalism");
  await expect(page.getByTestId("nb-shell")).toBeVisible();

  await page.goto("/scoreboard");
  await expect(page.getByTestId("nb-shell")).toBeVisible();
  await expect(page.getByRole("navigation", { name: "Primary" })).toBeVisible();
});

test("theme API rejects ids without a theme folder", async ({ page }) => {
  await page.goto("/admin/game?view=settings");
  const response = await page.request.put("/api/admin/platform/appearance", {
    data: { theme_id: "not-a-theme" },
  });
  expect(response.status()).toBe(400);
  await page.reload();
  await expect(page.locator("html")).toHaveAttribute("data-scheme", "graphite");
});
