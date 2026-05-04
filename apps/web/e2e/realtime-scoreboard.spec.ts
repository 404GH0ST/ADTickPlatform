import { expect, type Page } from "@playwright/test";
import { realtimeTest as test, resetMockApi } from "./test-utils";

async function assertScoreboardUpdates(page: Page) {
  const alphaRow = page.locator("tr").filter({
    has: page.getByRole("cell", { name: "College Alpha" }),
  });

  await expect(alphaRow).toBeVisible();
  await expect(alphaRow).toContainText("440");
  await expect(alphaRow).toContainText("455");
}

test("participant scoreboard page applies realtime scoreboard updates", async ({
  page,
  request,
}) => {
  await resetMockApi(request, "realtime-updates");
  await page.goto("/scoreboard");
  await assertScoreboardUpdates(page);
});

test("organizer scoreboard page applies realtime scoreboard updates", async ({
  page,
  request,
}) => {
  await resetMockApi(request, "realtime-updates");
  await page.goto("/admin/scoreboard");
  await assertScoreboardUpdates(page);
});
