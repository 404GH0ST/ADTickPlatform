import { expect } from "@playwright/test";
import { adminTest as test, mockApiBaseUrl } from "./test-utils";


test("organizer attacks page stays map-first and renders the broad accepted-attack feed", async ({
  page,
}) => {
  await page.goto("/admin/attacks");

  await expect(page.locator("h1", { hasText: "Attacks" })).toBeVisible();
  await expect(page.getByText("Accepted Attacks")).toBeVisible();
  await expect(page.getByText("Organizer attack map")).toBeVisible();
  await expect(page.getByText("Loaded 12 of 12 attack(s)")).toBeVisible();
  await expect(page.getByText("Broad feed")).toBeVisible();
  await expect(page.getByText("College Alpha").first()).toBeVisible();
  await expect(page.getByText("College Beta").first()).toBeVisible();
  await expect(page.getByText("first valid submission accepted").first()).toBeVisible();
});

test("organizer attacks page can focus the current tick and step through tick playback controls", async ({
  page,
}) => {
  await page.goto("/admin/attacks");

  const panel = page.getByTestId("attack-map-panel");

  await page.getByRole("button", { name: "Current Tick" }).click();

  await expect(panel.getByTestId("attack-map-visible-attacks")).toHaveText("4 attacks");
  await expect(panel.getByTestId("attack-map-selected-tick")).toHaveText("Tick #12");
  await expect(panel.getByTestId("attack-map-highlight-current-tick")).toBeVisible();

  await page.getByRole("button", { name: "Previous Tick" }).click();

  await expect(panel.getByTestId("attack-map-visible-attacks")).toHaveText("2 attacks");
  await expect(panel.getByTestId("attack-map-selected-tick")).toHaveText("Tick #11");
  await expect(panel.getByTestId("attack-map-highlight-current-tick")).toBeVisible();

  await page.getByRole("button", { name: "Next Tick" }).click();

  await expect(panel.getByTestId("attack-map-visible-attacks")).toHaveText("4 attacks");
  await expect(panel.getByTestId("attack-map-selected-tick")).toHaveText("Tick #12");
});

test("organizer attacks page shows empty-state messaging when no accepted attacks exist", async ({
  page,
  request,
}) => {
  await request.post(`${mockApiBaseUrl}/__reset`, {
    data: { scenario: "empty-attacks" },
  });

  await page.goto("/admin/attacks");
  await expect(
    page.getByText("No accepted attacks in the current slice to plot."),
  ).toBeVisible();
  await expect(
    page.getByText("No accepted attacks match the current organizer slice."),
  ).toBeVisible();
});

test("organizer attacks page shows degraded warning and empty-state messaging when the attack feed fails", async ({
  page,
  request,
}) => {
  await request.post(`${mockApiBaseUrl}/__reset`, {
    data: { scenario: "degraded-admin-attacks" },
  });

  await page.goto("/admin/attacks");

  await expect(
    page.getByText(
      "Organizer data is partially unavailable. Only live responses that succeeded are shown. No sample data is injected.",
    ),
  ).toBeVisible();
  await expect(
    page.getByText("No accepted attacks in the current slice to plot."),
  ).toBeVisible();
  await expect(
    page.getByText("No accepted attacks match the current organizer slice."),
  ).toBeVisible();
});
