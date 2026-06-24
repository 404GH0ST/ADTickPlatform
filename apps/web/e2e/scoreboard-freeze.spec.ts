import { expect } from "@playwright/test";
import { adminTest as test, mockApiBaseUrl } from "./test-utils";

test("organizer can freeze and clear the participant scoreboard", async ({
  page,
}) => {
  await page.goto("/admin/scoreboard");

  const card = page.getByTestId("scoreboard-freeze-card");
  await expect(card).toBeVisible();

  const stateBadge = page.getByTestId("scoreboard-freeze-state");
  await expect(stateBadge).toHaveText("Live");

  // A freeze_at in the past activates immediately.
  await page.getByTestId("scoreboard-freeze-at").fill("2020-01-01T00:00");
  await page.getByTestId("scoreboard-freeze-apply").click();
  await expect(stateBadge).toHaveText("Frozen");

  await page.getByTestId("scoreboard-freeze-clear").click();
  await expect(stateBadge).toHaveText("Live");
});

test("frozen scoreboard shows a banner on the participant view", async ({
  page,
  request,
}) => {
  // Arm the freeze directly on the gateway with a past start so it is active.
  const res = await request.post(
    `${mockApiBaseUrl}/api/v2/admin/game/scoreboard/freeze`,
    { data: { freeze_at: "2020-01-01T00:00:00Z" } },
  );
  expect(res.ok()).toBeTruthy();

  await page.goto("/scoreboard");
  await expect(page.getByTestId("scoreboard-frozen-banner")).toBeVisible();
  await expect(page.getByTestId("scoreboard-frozen-banner")).toContainText(
    "Scoreboard frozen",
  );
});
