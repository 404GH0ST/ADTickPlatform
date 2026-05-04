import { expect } from "@playwright/test";
import { realtimeTest as test } from "./test-utils";

test("participant attacks table applies realtime attack updates to the live slice count", async ({
  page,
}) => {
  await page.goto("/attacks/table");

  await expect(page.getByText("Showing 1-12 of 12")).toBeVisible();

  await expect(page.getByText("Showing 1-12 of 13")).toBeVisible();
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

  await expect(page.getByText("Tick #12", { exact: true })).toBeVisible();
  await expect(page.getByText("tick #12 completed")).toBeVisible();

  await expect(page.getByText("Tick #13", { exact: true })).toBeVisible();
  await expect(page.getByText("tick #13 completed")).toBeVisible();
});

test("organizer attacks route applies realtime updates to the loaded attack count", async ({
  page,
}) => {
  await page.goto("/admin/attacks");

  await expect(page.getByText("Loaded 12 of 12 attack(s)")).toBeVisible();

  await expect(page.getByText("Loaded 13 of 13 attack(s)")).toBeVisible();
});
