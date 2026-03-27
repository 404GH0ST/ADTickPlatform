import { expect, test } from "@playwright/test";

const mockApiBaseUrl = "http://127.0.0.1:4010";

test.use({ viewport: { width: 1280, height: 900 } });

test.beforeEach(async ({ request }) => {
  await request.post(`${mockApiBaseUrl}/__reset`, {
    data: { scenario: "realtime-updates" },
  });
});

test("participant scoreboard page applies realtime scoreboard updates", async ({
  page,
}) => {
  await page.goto("/scoreboard");

  const alphaRow = page.locator("tr").filter({
    has: page.getByRole("cell", { name: "College Alpha" }),
  });

  await expect(alphaRow).toContainText("440");
  await expect(alphaRow).toContainText("455");
});

test("organizer scoreboard page applies realtime scoreboard updates", async ({
  page,
}) => {
  await page.goto("/admin/scoreboard");

  const alphaRow = page.locator("tr").filter({
    has: page.getByRole("cell", { name: "College Alpha" }),
  });

  await expect(alphaRow).toContainText("440");
  await expect(alphaRow).toContainText("455");
});
