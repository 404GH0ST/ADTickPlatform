import { expect } from "@playwright/test";
import { adminTest as test, resetMockApi } from "./test-utils";

test("organizer scoreboard page surfaces cumulative attack, defense, and SLA per category", async ({
  page,
  request,
}) => {
  await resetMockApi(request);

  await page.goto("/admin/scoreboard");

  await expect(page.locator("h1", { hasText: "Scoreboard" })).toBeVisible();

  const panel = page.getByTestId("scoreboard-category-leaders-panel");
  await expect(panel).toBeVisible();
  await expect(panel.getByText("Kandidat Penghargaan Tim")).toBeVisible();

  const attackerCard = panel.locator('[aria-label="Tim Penyerang Terbaik"]');
  const defenderCard = panel.locator('[aria-label="Tim Bertahan Terbaik"]');
  const availabilityCard = panel.locator(
    '[aria-label="Tim dengan Ketersediaan Terbaik"]',
  );

  for (const card of [attackerCard, defenderCard, availabilityCard]) {
    await expect(card).toBeVisible();
    const items = card.locator("li");
    await expect(items).toHaveCount(3);
    const firstEntry = items.first();
    await expect(firstEntry).toContainText("#1");
    await expect(firstEntry).toContainText("College Alpha");
  }

  await expect(panel).toContainText("180.00");
  await expect(panel).toContainText("140.00");
  await expect(panel).toContainText("120.00");
});

test("organizer scoreboard category leaders panel renders the explicit empty state when no scores are persisted", async ({
  page,
  request,
}) => {
  await resetMockApi(request, "empty-scoreboard");

  await page.goto("/admin/scoreboard");

  const panel = page.getByTestId("scoreboard-category-leaders-panel");
  await expect(panel).toBeVisible();
  const empty = panel.getByText("Belum ada tim yang bermain.");
  await expect(empty).toHaveCount(3);
});
