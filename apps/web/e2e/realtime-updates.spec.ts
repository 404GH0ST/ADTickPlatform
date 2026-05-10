import { expect } from "@playwright/test";
import { mockApiBaseUrl, realtimeTest as test } from "./test-utils";

test("participant attacks table applies realtime attack updates to the live slice count", async ({
  page,
  request,
}) => {
  await page.goto("/attacks");
  await expect(page.getByText("Loaded 12 of 12 attack(s)")).toBeVisible();

  await request.post(`${mockApiBaseUrl}/__reset`, {
    data: { scenario: "realtime-updates" },
  });

  await expect(page.getByText("Loaded 13 of 13 attack(s)")).toBeVisible();
  await page.getByRole("link", { name: "Table View" }).click();
  await expect(page.getByRole("cell", { name: "#13" })).toBeVisible();
});

test("participant attack map route applies realtime updates to the loaded attack count", async ({
  page,
}) => {
  await page.goto("/attacks");

  await expect(page.getByText("Loaded 12 of 12 attack(s)")).toBeVisible();

  await expect(page.getByText("Loaded 13 of 13 attack(s)")).toBeVisible();
});

test("organizer game page applies realtime game-status updates to the current tick card", async ({
  page,
}) => {
  await page.goto("/admin/game");
  const currentTickCard = page.getByTestId("current-tick-card").first();

  await expect(currentTickCard).toContainText("Tick #12");

  await expect(currentTickCard).toContainText("Tick #13");
  await expect(currentTickCard).toContainText("tick #13 completed");
});

test("organizer attacks route applies realtime updates to the loaded attack count", async ({
  page,
}) => {
  await page.goto("/admin/attacks");

  await expect(page.getByText("Loaded 12 of 12 attack(s)")).toBeVisible();

  await expect(page.getByText("Loaded 13 of 13 attack(s)")).toBeVisible();
});
