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
  await expect(panel.getByText("Team Award Candidates")).toBeVisible();

  const attackerCard = panel.locator('[aria-label="Best Attacker"]');
  const defenderCard = panel.locator('[aria-label="Best Defender"]');
  const availabilityCard = panel.locator(
    '[aria-label="Best Availability"]',
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
  const empty = panel.getByText(
    "No team has a score yet. Rankings will appear after the first scored tick; verify the match and scheduler are running if this remains empty.",
  );
  await expect(empty).toHaveCount(3);
});
