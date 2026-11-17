import { expect, test } from "@playwright/test";

import { mockApiBaseUrl, participantStorageState, resetMockApi } from "./test-utils";

/*
 * Uses the test-only "e2e-crash" theme (themes/_fixtures, registered only in
 * e2e builds), whose Scoreboard, Services and OrganizerAudit views throw.
 * A broken theme must degrade to the stock views, on the server render and
 * in the browser, without breaking the page or its features.
 */

test.use({ viewport: { width: 1280, height: 900 } });

test.beforeEach(async ({ request }) => {
  await resetMockApi(request);
  await request.post(`${mockApiBaseUrl}/api/v2/admin/game/match/start`);
  const response = await request.put(`${mockApiBaseUrl}/api/v2/admin/platform/appearance`, {
    data: { theme_id: "e2e-crash" },
  });
  expect(response.ok()).toBeTruthy();
});

test.describe("as a participant", () => {
  test.use({ storageState: participantStorageState });

  test("a view that throws during server rendering does not fail the page", async ({ page }) => {
    // React defers the failed slot to the browser, which then shows the stock view.
    const response = await page.request.get("/scoreboard");
    expect(response.status()).toBe(200);
    const html = await response.text();
    expect(html).toContain('data-scheme="e2e-crash"');
    expect(html).toContain('<h1');
  });

  test("browser render falls back, logs, and keeps features working", async ({ page }) => {
    const errors: string[] = [];
    page.on("console", (message) => {
      if (message.type() === "error") errors.push(message.text());
    });

    await page.goto("/scoreboard");
    await expect(page.locator("html")).toHaveAttribute("data-scheme", "e2e-crash");
    await expect(page.getByTestId("participant-scoreboard-card")).toBeVisible();
    await expect
      .poll(() => errors.some((text) => text.includes("[theme:e2e-crash] Scoreboard override failed")))
      .toBe(true);

    // Services also crashes in this theme; the stock page must stay usable.
    await page.goto("/services");
    const card = page.getByTestId("service-card-svc-1");
    await expect(card).toBeVisible();
    await page.getByRole("textbox", { name: "Flags", exact: true }).fill("PLAYIT{enemy-challenge-1}");
    await page.getByRole("button", { name: "Submit", exact: true }).click();
    await expect(card.getByRole("button", { name: "SSH Access" })).toBeEnabled();
  });
});

test("organizer views fall back too", async ({ page }) => {
  await page.goto("/admin");
  await expect(page.getByTestId("trusted-state-panel")).toBeVisible();
  await expect(page.getByTestId("run-trusted-reconcile")).toBeEnabled();

  await page.goto("/admin/audit");
  await expect(page.getByText("Audit Log", { exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: "Apply Filters" })).toBeVisible();
});
