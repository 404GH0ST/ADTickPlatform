import { expect } from "@playwright/test";
import { adminTest as test, resetMockApi } from "./test-utils";


test("participant scoreboard page shows the finished-match banner and ranking rows", async ({
  page,
  request,
}) => {
  await resetMockApi(request);
  await page.setViewportSize({ width: 2048, height: 1152 });

  await page.goto("/scoreboard");

  await expect(page.locator("h1", { hasText: "Scoreboard" })).toBeVisible();
  await expect(
    page.getByText(
      "The match has finished at tick #12. Participant submissions are closed.",
    ),
  ).toBeVisible();
  await expect(
    page.getByText("Team ranking with per-service attack, defense, SLA, and total scores."),
  ).toBeVisible();
  const scoreboardTable = page.getByRole("table");
  await expect(scoreboardTable.locator("caption")).toContainText(
    "Right-side columns are team totals across all services",
  );
  await expect(page.locator('[aria-label*="current team"]')).toHaveCount(1);
  await expect(scoreboardTable.getByText("Floppcraft", { exact: true }).first()).toBeVisible();
  await expect(scoreboardTable.getByText("College Alpha")).toBeVisible();
  await expect(scoreboardTable.getByText("440.00")).toBeVisible();

  const currentTeamRow = scoreboardTable.locator("tbody tr", {
    has: page.locator('[aria-label*="current team"]'),
  });
  const currentTeamCell = currentTeamRow.locator("td").nth(1);
  const firstServiceCell = currentTeamRow.locator("td").nth(2);
  const [teamBox, serviceBox] = await Promise.all([
    currentTeamCell.boundingBox(),
    firstServiceCell.boundingBox(),
  ]);
  expect(teamBox).not.toBeNull();
  expect(serviceBox).not.toBeNull();
  expect(teamBox!.x + teamBox!.width).toBeLessThanOrEqual(serviceBox!.x + 1);
});

test("participant scoreboard page shows an explicit empty state when no scores exist", async ({
  page,
  request,
}) => {
  await resetMockApi(request, "empty-scoreboard");

  await page.goto("/scoreboard");

  await expect(page.locator("h1", { hasText: "Scoreboard" })).toBeVisible();
  await expect(
    page.getByRole("status").filter({ hasText: "No score rows are available yet." }),
  ).toBeVisible();
});

test("organizer scoreboard page filters and sorts authoritative rankings", async ({
  page,
  request,
}) => {
  await resetMockApi(request);

  await page.goto("/admin/scoreboard");

  await expect(page.locator("h1", { hasText: "Scoreboard" })).toBeVisible();
  await expect(page.getByText("Authoritative Scoreboard")).toBeVisible();
  await expect(page.getByText("Showing 3 of 3 teams.")).toBeVisible();

  await page.getByLabel("Team Filter").fill("beta");
  await expect(page.getByText("Showing 1 of 3 teams.")).toBeVisible();
  let scoreboardTable = page.getByRole("table");
  await expect(scoreboardTable.getByText("College Beta")).toBeVisible();
  await expect(scoreboardTable.getByText("College Alpha")).toHaveCount(0);

  await page.getByLabel("Team Filter").fill("");
  // Click on the column header button "Total" to sort by total score ascending
  await page.getByRole("button", { name: "Total", exact: true }).click();

  scoreboardTable = page.locator("table").filter({
    has: page.getByText("College Alpha"),
  }).first();
  const firstDataRow = scoreboardTable.locator("tbody tr").first();
  await expect(firstDataRow).toContainText("College Gamma");
  await expect(firstDataRow).toContainText("295.00");
});

test("organizer scoreboard page shows degraded warning and empty state when the authoritative feed fails", async ({
  page,
  request,
}) => {
  await resetMockApi(request, "degraded-admin");

  await page.goto("/admin/scoreboard");

  await expect(page.locator("h1", { hasText: "Scoreboard" })).toBeVisible();
  await expect(
    page.getByText(
      "Organizer data is partially unavailable. Only live responses that succeeded are shown. No sample data is injected.",
    ),
  ).toBeVisible();
  await expect(page.getByText("Showing 0 of 0 teams.")).toBeVisible();
  await expect(
    page.getByRole("status").filter({ hasText: "No score rows persisted yet." }),
  ).toBeVisible();
});
