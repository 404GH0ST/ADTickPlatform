import { expect, test } from "@playwright/test";

const mockApiBaseUrl = "http://127.0.0.1:4010";

test.use({ viewport: { width: 1280, height: 900 } });

test.beforeEach(async ({ request }) => {
  await request.post(`${mockApiBaseUrl}/__reset`);
});

test("participant scoreboard page shows the finished-match banner and ranking rows", async ({
  page,
}) => {
  await page.goto("/scoreboard");

  await expect(page.locator("h1", { hasText: "Scoreboard" })).toBeVisible();
  await expect(
    page.getByText(
      "The match has finished at tick #12. Participant submissions are closed.",
    ),
  ).toBeVisible();
  await expect(page.getByText("Attack, defense, SLA, and total score.")).toBeVisible();
  await expect(page.getByText("College Alpha")).toBeVisible();
  await expect(page.getByText("440")).toBeVisible();
});

test("participant scoreboard page shows an explicit empty state when no scores exist", async ({
  page,
  request,
}) => {
  await request.post(`${mockApiBaseUrl}/__reset`, {
    data: { scenario: "empty-scoreboard" },
  });

  await page.goto("/scoreboard");

  await expect(page.locator("h1", { hasText: "Scoreboard" })).toBeVisible();
  await expect(page.getByText("No score rows are available yet.")).toBeVisible();
});

test("organizer scoreboard page filters and sorts authoritative rankings", async ({
  page,
}) => {
  await page.goto("/admin/scoreboard");

  await expect(page.locator("h1", { hasText: "Scoreboard" })).toBeVisible();
  await expect(page.getByText("Authoritative Scoreboard")).toBeVisible();

  await page.getByLabel("Team Filter").fill("beta");
  await expect(page.getByText("Showing 1 of 3 team row(s).")).toBeVisible();
  await expect(page.getByText("College Beta")).toBeVisible();
  await expect(page.getByText("College Alpha")).toHaveCount(0);

  await page.getByLabel("Team Filter").fill("");
  await page.getByLabel("Sort By").selectOption("total");
  await page.getByLabel("Direction").selectOption("asc");

  const scoreboardTable = page.locator("table").filter({
    has: page.getByText("College Alpha"),
  }).first();
  const firstDataRow = scoreboardTable.locator("tbody tr").first();
  await expect(firstDataRow).toContainText("College Gamma");
  await expect(firstDataRow).toContainText("295");
});

test("organizer scoreboard page shows degraded warning and empty state when the authoritative feed fails", async ({
  page,
  request,
}) => {
  await request.post(`${mockApiBaseUrl}/__reset`, {
    data: { scenario: "degraded-admin" },
  });

  await page.goto("/admin/scoreboard");

  await expect(page.locator("h1", { hasText: "Scoreboard" })).toBeVisible();
  await expect(
    page.getByText(
      "Organizer data is partially unavailable. Only live responses that succeeded are shown. No sample data is injected.",
    ),
  ).toBeVisible();
  await expect(page.getByText("Showing 0 of 0 team row(s).")).toBeVisible();
  await expect(page.getByText("No score rows persisted yet.")).toBeVisible();
});
