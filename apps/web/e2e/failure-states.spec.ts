import { expect, test, type APIRequestContext } from "@playwright/test";

import {
  loginAsOrganizer,
  loginAsParticipant,
  mockApiBaseUrl,
  resetMockApi,
} from "./test-utils";

test.use({
  viewport: { width: 1280, height: 900 },
  storageState: { cookies: [], origins: [] },
});

test.beforeEach(async ({ request }) => {
  await resetMockApi(request);
});

test.afterEach(async ({ request }) => {
  await resetMockApi(request);
});

async function setSessionOutage(request: APIRequestContext, enabled: boolean) {
  const response = await request.post(`${mockApiBaseUrl}/__session-outage`, {
    data: { enabled },
  });
  expect(response.ok()).toBeTruthy();
}

test("an unknown address gets a titled page with a way back", async ({ page }) => {
  const response = await page.goto("/no-such-page");

  expect(response?.status()).toBe(404);
  await expect(page).toHaveTitle("Page Not Found | AD Platform");
  await expect(page.getByRole("main").getByRole("heading", { level: 1 })).toHaveText("Page not found");

  await page.getByRole("link", { name: "Go to Overview" }).click();
  await expect(page).toHaveURL(/\/$/);
});

test("an outage does not tell a signed-in participant to sign in", async ({ context, page, request }) => {
  await loginAsParticipant(context);
  await setSessionOutage(request, true);

  await page.goto("/services");

  await expect(page.getByText("You have not been signed out.")).toBeVisible();
  await expect(page.getByText("Your services can't be loaded right now.")).toBeVisible();
  await expect(page.getByText(/No services are assigned/)).toHaveCount(0);
  await expect(page.getByRole("link", { name: "Sign In" })).toHaveCount(0);

  await setSessionOutage(request, false);
  await page.getByRole("button", { name: "Try again" }).click();

  await expect(page.getByRole("button", { name: "Try again" })).toHaveCount(0);
  await expect(page.getByText("You have not been signed out.")).toHaveCount(0);
  await expect(page.getByTestId("participant-summary")).not.toContainText("Unavailable");
});

test("an outage keeps organizer routes closed without a login bounce", async ({ context, page, request }) => {
  await loginAsOrganizer(context);
  await setSessionOutage(request, true);

  await page.goto("/admin/teams");

  await expect(page).toHaveURL(/\/admin\/teams$/);
  await expect(page.getByRole("heading", { level: 1, name: "Platform unavailable" })).toBeVisible();
  await expect(page.getByRole("link", { name: "Players" })).toHaveCount(0);

  const api = await context.request.get("/api/admin/teams");
  expect(api.status()).toBe(503);

  await setSessionOutage(request, false);
  await page.getByRole("button", { name: "Try again" }).click();

  await expect(page).toHaveURL(/\/admin\/teams$/);
  await expect(page.getByTestId("system-state")).toHaveCount(0);
});

test("an outage does not open organizer pages to a participant", async ({ context, page, request }) => {
  await loginAsParticipant(context);
  await setSessionOutage(request, true);

  await page.goto("/admin/teams");

  await expect(page.getByRole("heading", { level: 1, name: "Platform unavailable" })).toBeVisible();
  expect((await context.request.get("/api/admin/teams")).status()).toBe(503);

  await setSessionOutage(request, false);
  await page.getByRole("button", { name: "Try again" }).click();
  await expect(page).not.toHaveURL(/\/admin/);
});

test("an outage does not change where a signed-out visitor goes", async ({ page, request }) => {
  await setSessionOutage(request, true);

  await page.goto("/admin/teams");

  await expect(page).toHaveURL(/\/login$/);
  await expect(page.getByTestId("session-unavailable-notice")).toHaveCount(0);
});

test("a failed score request is not shown as an empty board", async ({ page, request }) => {
  // This scenario also fails the live stream, which would otherwise refill the board.
  await resetMockApi(request, "degraded-participant-scoreboard");

  await page.goto("/scoreboard");

  await expect(page.getByTestId("data-unavailable")).toContainText("Scores can't be loaded");
  await expect(page.getByText(/No scores yet|No score rows/)).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Try again" })).toHaveCount(1);
});

for (const [themeId, registerLabel] of [["graphite", "Register"], ["neubrutalism", "New member"]]) {
  test(`${themeId}: text typed before the page is interactive survives hydration`, async ({ browser, request }) => {
    const saved = await request.put(`${mockApiBaseUrl}/api/v2/admin/platform/appearance`, {
      data: { theme_id: themeId },
    });
    expect(saved.ok()).toBeTruthy();
    const context = await browser.newContext({ storageState: { cookies: [], origins: [] } });
    const page = await context.newPage();
    // Holding the scripts back leaves the server-rendered form on screen, unhydrated.
    let release = () => {};
    const held = new Promise<void>((resolve) => { release = resolve; });
    await page.route("**/_next/static/**/*.js", async (route) => {
      await held;
      await route.continue();
    });

    await page.goto("/login", { waitUntil: "commit" });
    await page.getByLabel("Email").fill("early@example.test");
    await page.getByLabel("Password").fill("typed-before-load");
    release();

    // Switching mode forces a controlled re-render, which is what used to erase the text.
    await expect(async () => {
      await page.getByRole("button", { name: registerLabel, exact: true }).first().click({ timeout: 1_000 });
      await expect(page.getByLabel("Display name")).toBeVisible({ timeout: 500 });
    }).toPass();

    await expect(page.getByLabel("Email")).toHaveValue("early@example.test");
    await expect(page.getByLabel("Password")).toHaveValue("typed-before-load");
    await context.close();
  });
}

test("an outage keeps an unsent flag and the last data on screen", async ({ context, page, request }) => {
  await resetMockApi(request);
  await request.post(`${mockApiBaseUrl}/api/v2/admin/game/match/start`);
  await loginAsParticipant(context);
  await page.goto("/services");

  const flags = page.getByRole("textbox", { name: "Flags" });
  await flags.fill("PLAYIT{typed_not_sent}");
  await expect(page.getByText("1 flag ready")).toBeVisible();
  await expect(page.getByTestId("service-card-svc-1")).toBeVisible();

  await setSessionOutage(request, true);
  await page.evaluate(() => window.dispatchEvent(new Event("platform:flag-accepted")));

  await expect(page.getByText("You have not been signed out.")).toBeVisible();
  await expect(page.getByTestId("stale-data-notice")).toBeVisible();
  await expect(flags).toHaveValue("PLAYIT{typed_not_sent}");
  await expect(page.getByTestId("service-card-svc-1")).toBeVisible();

  await setSessionOutage(request, false);
  await page.getByRole("button", { name: "Try again" }).focus();
  await page.keyboard.press("Enter");

  await expect(page.getByTestId("stale-data-notice")).toHaveCount(0);
  await expect(flags).toHaveValue("PLAYIT{typed_not_sent}");
  await expect(page.getByRole("heading", { level: 1 })).toBeFocused();
});

test("a live scoreboard is not called stale when only the first load failed", async ({ page, request }) => {
  await resetMockApi(request, "degraded-participant");

  await page.goto("/scoreboard");

  await expect(page.getByRole("table")).toContainText("College Alpha");
  await expect(page.getByTestId("stale-data-notice")).toHaveCount(0);
});

test("a failed attack filter says so and does not relabel the rows", async ({ page, request }) => {
  const viewBadge = (label: string) => page.locator('[data-slot="badge"]').getByText(label, { exact: true });
  await resetMockApi(request);
  await page.goto("/attacks");
  await expect(viewBadge("Live")).toBeVisible();

  await page.route("**/api/platform/attacks?**", (route) =>
    route.fulfill({ status: 503, contentType: "application/json", body: '{"detail":"attack store unavailable"}' }),
  );
  await page.locator("#attack-attacker").selectOption({ index: 2 });
  await page.getByRole("button", { name: "Apply view" }).click();

  await expect(page.getByTestId("attack-feed-error")).toContainText("attack store unavailable");
  await expect(viewBadge("Filtered")).toHaveCount(0);
  await expect(viewBadge("Live")).toBeVisible();
  await expect(page.getByText(/^Selected view:/)).toBeVisible();

  await page.unroute("**/api/platform/attacks?**");
  await page.getByRole("button", { name: "Apply view" }).click();

  await expect(page.getByTestId("attack-feed-error")).toHaveCount(0);
  await expect(viewBadge("Filtered")).toBeVisible();
  await expect(page.getByText(/^Showing attacker /)).toBeVisible();
});

test("an unsent flag survives a reload", async ({ context, page, request }) => {
  await resetMockApi(request);
  await request.post(`${mockApiBaseUrl}/api/v2/admin/game/match/start`);
  await loginAsParticipant(context);
  await page.goto("/services");

  const flags = page.getByRole("textbox", { name: "Flags" });
  await flags.fill("PLAYIT{before_reload}");
  await expect(page.getByText("1 flag ready")).toBeVisible();

  await page.reload();

  await expect(flags).toHaveValue("PLAYIT{before_reload}");
  await expect(page.getByText("1 flag ready")).toBeVisible();
});

test("a failed sign-in is exposed after its live region", async ({ page }) => {
  await page.goto("/login");
  await page.evaluate(() => {
    const seen: Array<{ text: string; hidden: boolean }> = [];
    (window as unknown as { statusLog: typeof seen }).statusLog = seen;
    new MutationObserver((records) => {
      for (const record of records) {
        for (const node of record.addedNodes) {
          if (!(node instanceof HTMLElement)) continue;
          const region = node.matches('[role="status"]') ? node : node.querySelector('[role="status"]');
          const text = region?.querySelector("span");
          if (text) seen.push({ text: text.textContent ?? "", hidden: getComputedStyle(text).visibility === "hidden" });
        }
      }
    }).observe(document.body, { childList: true, subtree: true });
  });

  await page.getByLabel("Email").fill("nobody@example.test");
  await page.getByLabel("Password").fill("wrong-password");
  await page.locator('form button[type="submit"]').click();

  const status = page.getByRole("status").filter({ hasText: /./ }).first();
  await expect(status).toBeVisible();
  await expect(status.locator("span")).toBeVisible();
  const log = await page.evaluate(
    () => (window as unknown as { statusLog: Array<{ hidden: boolean }> }).statusLog,
  );
  expect(log.length).toBeGreaterThan(0);
  expect(log[0].hidden).toBe(true);
});
