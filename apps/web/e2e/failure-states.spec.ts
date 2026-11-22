import { expect, test, type APIRequestContext, type Page } from "@playwright/test";

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

test("a failed organizer filter does not relabel the rows", async ({ context, page, request }) => {
  const viewBadge = (label: string) => page.locator('[data-slot="badge"]').getByText(label, { exact: true });
  await resetMockApi(request);
  await loginAsOrganizer(context);
  await page.goto("/admin/attacks");
  await expect(viewBadge("Live")).toBeVisible();

  await page.route("**/api/admin/game/attacks?**", (route) =>
    route.fulfill({ status: 503, contentType: "application/json", body: '{"detail":"attack store unavailable"}' }),
  );
  await page.locator("#admin-attack-attacker").selectOption({ index: 2 });
  await page.getByRole("button", { name: "Apply view" }).click();

  await expect(page.getByText("attack store unavailable")).toBeVisible();
  await expect(viewBadge("Filtered")).toHaveCount(0);
  await expect(viewBadge("Live")).toBeVisible();

  await page.unroute("**/api/admin/game/attacks?**");
  await page.getByRole("button", { name: "Apply view" }).click();
  await expect(viewBadge("Filtered")).toBeVisible();
});

test("the SSH password really copies when the browser refuses the clipboard API", async ({ context, page, request }) => {
  await context.grantPermissions(["clipboard-read", "clipboard-write"]);
  await resetMockApi(request);
  await request.post(`${mockApiBaseUrl}/api/v2/admin/game/match/start`);
  await loginAsParticipant(context);
  await page.goto("/services");

  const card = page.getByTestId("service-card-svc-1");
  await page.getByRole("textbox", { name: "Flags", exact: true }).fill("PLAYIT{enemy-challenge-1}");
  await page.getByRole("button", { name: "Submit", exact: true }).click();
  await expect(card.getByText("SSH unlocked by accepted enemy flag")).toBeVisible();

  await page.evaluate(async () => {
    await navigator.clipboard.writeText("old clipboard");
    navigator.clipboard.writeText = () => Promise.reject(new Error("denied"));
  });
  await card.getByRole("button", { name: "SSH Access" }).click();
  const dialog = page.getByRole("dialog", { name: "SSH Access" });
  const copy = dialog.getByRole("button", { name: "Copy password" });
  await copy.click();

  await expect(dialog.getByRole("button", { name: "Copied" })).toBeFocused();
  await expect(dialog.getByText("Password copied")).toBeAttached();
  expect(await page.evaluate(() => navigator.clipboard.readText())).toBe("Adp-team-credential-Aa1!");
});

async function failRoutes(request: APIRequestContext, routes: string[]) {
  const response = await request.post(`${mockApiBaseUrl}/__failed-routes`, { data: { routes } });
  expect(response.ok()).toBeTruthy();
}

test("an unknown match status hides the match controls", async ({ context, page, request }) => {
  await resetMockApi(request);
  await loginAsOrganizer(context);
  // The live stream also reports the match state, so both have to be down.
  await failRoutes(request, ["GET /api/v2/admin/game/status", "GET /admin/v1/game/status/stream"]);

  await page.goto("/admin/game");

  await expect(page.getByTestId("match-status-unknown")).toBeVisible();
  await expect(page.getByTestId("start-match")).toHaveCount(0);
  await expect(page.getByTestId("organizer-match-status")).toContainText("Status unavailable");

  await failRoutes(request, []);
  await page.getByTestId("refresh-game-status").click();

  await expect(page.getByTestId("match-status-unknown")).toHaveCount(0);
  await expect(page.getByTestId("organizer-match-status")).not.toContainText("Status unavailable");
});

test("a failed organizer list is not called empty", async ({ context, page, request }) => {
  await resetMockApi(request);
  await loginAsOrganizer(context);
  await failRoutes(request, ["GET /api/v2/admin/teams"]);

  await page.goto("/admin/teams");

  await expect(page.getByText("Teams can't be loaded right now.")).toBeVisible();
  await expect(page.getByText("No teams yet")).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Try again" })).toHaveCount(1);
});

test("an organizer form protects typed text and shows save errors inside the dialog", async ({ context, page, request }) => {
  await resetMockApi(request);
  await loginAsOrganizer(context);
  await page.goto("/admin/teams");
  await page.getByRole("button", { name: "Create Team" }).first().click();
  const dialog = page.getByRole("dialog");
  const name = dialog.getByLabel("Team name");
  await name.fill("Half typed team");

  await page.keyboard.press("Escape");
  await expect(dialog.getByText("Discard your changes?")).toBeVisible();
  await expect(dialog.getByRole("button", { name: "Keep editing" })).toBeFocused();
  await page.keyboard.press("Escape");
  await expect(dialog.getByText("Discard your changes?")).toHaveCount(0);
  await expect(name).toHaveValue("Half typed team");

  await dialog.getByLabel("Contact email").fill("team@example.test");
  await page.route("**/api/admin/teams", (route) =>
    route.request().method() === "POST"
      ? route.fulfill({ status: 403, contentType: "application/json", body: '{"title":"Authentication required","status":403,"detail":"please authenticate before accessing organizer routes."}' })
      : route.continue(),
  );
  await dialog.getByRole("button", { name: "Create", exact: true }).click();

  await expect(dialog.getByText("Your session has ended. Sign in again to continue.")).toBeVisible();
  await expect(dialog.getByRole("link", { name: /Sign in again/ })).toHaveAttribute("target", "_blank");
  await expect(name).toHaveValue("Half typed team");

  await page.keyboard.press("Escape");
  await dialog.getByRole("button", { name: "Discard changes" }).click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await expect(page.getByText("Your session has ended.")).toHaveCount(0);
});

test("account settings ask before discarding typed changes", async ({ context, page, request }) => {
  await resetMockApi(request);
  await loginAsParticipant(context);
  await page.goto("/services");
  await page.getByRole("button", { name: "Account settings" }).click();
  const dialog = page.getByRole("dialog", { name: "Account settings" });
  const name = dialog.getByLabel("Display name");
  await name.fill("Changed display name");

  await page.keyboard.press("Escape");
  await expect(dialog.getByText("Discard your changes?")).toBeVisible();
  await expect(dialog.getByRole("button", { name: "Keep editing" })).toBeFocused();
  await dialog.getByRole("button", { name: "Keep editing" }).click();
  await expect(name).toHaveValue("Changed display name");

  await dialog.getByRole("button", { name: "Close" }).last().click();
  await dialog.getByRole("button", { name: "Discard changes" }).click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
});

test("schedule times are entered in WIB on any device", async ({ browser }) => {
  const context = await browser.newContext({
    storageState: { cookies: [], origins: [] },
    timezoneId: "America/New_York",
    locale: "en-US",
  });
  await loginAsOrganizer(context);
  const page = await context.newPage();
  await page.goto("/admin/game");

  await expect(page.locator("#match-schedule-zone")).toHaveText("Times are in WIB (UTC+7).");
  await page.getByLabel("Scheduled Start Date").fill("2026-12-01");
  await page.getByLabel("Scheduled Start Time").fill("10:00");
  await expect(page.locator("#match-start-preview")).toContainText("2026-12-01 10:00:00 WIB");
  await expect(page.locator("#match-start-preview")).toContainText("On this device:");
  await expect(page.locator("#match-start-preview")).toContainText("America/New_York");

  await page.getByLabel("Scheduled End Date").fill("2026-12-01");
  await page.getByLabel("Scheduled End Time").fill("18:00");
  await page.getByRole("button", { name: "Save Window" }).click();
  await expect(page.getByText("Start: 2026-12-01 10:00:00 WIB; end: 2026-12-01 18:00:00 WIB.")).toBeVisible();
  await expect(page.getByLabel("Scheduled Start Time")).toHaveValue("10:00");
  await context.close();
});

test("the freeze card says when it can't read the freeze", async ({ context, page, request }) => {
  await loginAsOrganizer(context);
  const freeze = await request.post(`${mockApiBaseUrl}/api/v2/admin/game/scoreboard/freeze`, {
    data: { freeze_at: "2020-01-01T00:00:00Z" },
  });
  expect(freeze.ok()).toBeTruthy();
  let failing = true;
  await page.route("**/api/admin/game/scoreboard/freeze", (route) =>
    failing && route.request().method() === "GET"
      ? route.fulfill({ status: 503, contentType: "application/json", body: '{"detail":"upstream store unavailable"}' })
      : route.continue(),
  );

  await page.goto("/admin/scoreboard");
  const state = page.getByTestId("scoreboard-freeze-state");
  await expect(state).toHaveText("Status unavailable");
  await expect(page.getByTestId("scoreboard-freeze-clear")).toBeEnabled();

  failing = false;
  await page.getByTestId("scoreboard-freeze-card").getByRole("button", { name: "Try again" }).click();
  await expect(state).toHaveText("Frozen");
});

test("the freeze form checks the order of its times", async ({ context, page }) => {
  await loginAsOrganizer(context);
  let posts = 0;
  page.on("request", (sent) => {
    if (sent.method() === "POST" && sent.url().endsWith("/scoreboard/freeze")) posts += 1;
  });
  await page.goto("/admin/scoreboard");
  const card = page.getByTestId("scoreboard-freeze-card");
  const start = card.getByRole("group", { name: "Freeze at", exact: true });
  const end = card.getByRole("group", { name: "Unfreeze at (optional)" });
  await start.getByLabel("Date", { exact: true }).fill("2026-12-02");
  await start.getByLabel("Time", { exact: true }).fill("10:00");
  await end.getByLabel("Date", { exact: true }).fill("2026-12-01");
  await end.getByLabel("Time", { exact: true }).fill("10:00");
  await page.getByTestId("scoreboard-freeze-apply").click();
  await expect(card.getByText("The unfreeze time must be later than the freeze time.")).toBeVisible();
  expect(posts).toBe(0);
});

test("the organizer header follows the live state when the status read is down", async ({ context, page, request }) => {
  await loginAsOrganizer(context);
  await failRoutes(request, ["GET /api/v2/admin/game/status"]);
  await page.goto("/admin/game");
  await expect(page.getByTestId("organizer-match-status")).not.toContainText("Status unavailable");
  await expect(page.getByTestId("match-status-unknown")).toHaveCount(0);
});

async function openAccountSettings(page: Page) {
  await page.goto("/services");
  await page.getByRole("button", { name: "Account settings" }).click();
  const dialog = page.getByRole("dialog", { name: "Account settings" });
  await expect(dialog.getByLabel("Display name")).toBeVisible();
  return dialog;
}

test("a bad new password sends nothing", async ({ context, page }) => {
  await loginAsParticipant(context);
  let puts = 0;
  page.on("request", (sent) => {
    if (sent.method() === "PUT" && sent.url().includes("/api/platform/session/")) puts += 1;
  });
  const dialog = await openAccountSettings(page);
  await dialog.getByLabel("Display name").fill("Should not save");
  await dialog.getByLabel("Current password").fill("alpha-password");
  await dialog.getByLabel("New password", { exact: true }).fill("short");
  await dialog.getByLabel("Confirm new password").fill("short");
  await dialog.getByRole("button", { name: "Save" }).click();
  await expect(dialog.getByText("Your new password must be at least 8 characters.")).toBeVisible();
  await expect(dialog.getByLabel("New password", { exact: true })).toBeFocused();
  expect(puts).toBe(0);
});

test("a rejected password says the profile did save", async ({ context, page }) => {
  await loginAsParticipant(context);
  await page.route("**/api/platform/session/password", (route) =>
    route.fulfill({ status: 401, contentType: "application/json", body: '{"detail":"current password is incorrect"}' }),
  );
  const dialog = await openAccountSettings(page);
  await dialog.getByLabel("Display name").fill("Saved Display Name");
  await dialog.getByLabel("Current password").fill("wrong-password");
  await dialog.getByLabel("New password", { exact: true }).fill("new-password-1");
  await dialog.getByLabel("Confirm new password").fill("new-password-1");
  await dialog.getByRole("button", { name: "Save" }).click();
  await expect(dialog.getByText("Profile saved.")).toBeVisible();
  await expect(dialog.getByText(/Password not changed: .*incorrect/)).toBeVisible();
});

test("closing during a save doesn't offer to discard it", async ({ context, page }) => {
  await loginAsParticipant(context);
  await page.route("**/api/platform/session/profile", async (route) => {
    await new Promise((resolve) => setTimeout(resolve, 900));
    await route.continue();
  });
  const dialog = await openAccountSettings(page);
  await dialog.getByLabel("Display name").fill("Saved While Waiting");
  await dialog.getByRole("button", { name: "Save" }).click();
  await page.keyboard.press("Escape");
  await expect(dialog.getByText("Still saving. Closing won't cancel it.")).toBeVisible();
  await expect(dialog.getByRole("button", { name: "Discard changes" })).toHaveCount(0);
  await expect(dialog.getByText("Account settings saved.")).toBeVisible();
  await expect(dialog.getByText("Still saving. Closing won't cancel it.")).toHaveCount(0);
  await page.keyboard.press("Escape");
  await expect(page.getByRole("dialog")).toHaveCount(0);
});

test("captain transfer confirms the teammate and keeps other edits protected", async ({ context, page }) => {
  await loginAsParticipant(context);
  let posts = 0;
  await page.route("**/api/platform/session/team/members", (route) =>
    route.fulfill({
      contentType: "application/json",
      body: JSON.stringify([
        { player_id: 1001, display_name: "Alpha Captain", email: "alpha.captain@college.local", role: "captain" },
        { player_id: 1002, display_name: "Alpha Mate", email: "mate@college.local", role: "member" },
      ]),
    }),
  );
  await page.route("**/api/platform/session/team/captain", (route) => {
    posts += 1;
    return route.fulfill({ contentType: "application/json", body: "{}" });
  });
  const dialog = await openAccountSettings(page);
  await dialog.getByLabel("Display name").fill("Unsaved Draft Name");
  await dialog.getByLabel("Teammate").selectOption("1002");
  await dialog.getByRole("button", { name: "Transfer", exact: true }).click();

  await expect(dialog.getByText(/Make Alpha Mate the captain\?/)).toBeVisible();
  await expect(dialog.getByRole("button", { name: "Stay captain" })).toBeFocused();
  expect(posts).toBe(0);

  await dialog.getByRole("button", { name: "Make captain" }).click();
  await expect(dialog.getByText("Captainship transferred. You are now a team member.")).toBeVisible();
  expect(posts).toBe(1);

  await page.keyboard.press("Escape");
  await expect(dialog.getByText("Discard your changes?")).toBeVisible();
  await expect(dialog.getByLabel("Display name")).toHaveValue("Unsaved Draft Name");
});

test("a failed teammate list offers a retry instead of claiming none", async ({ context, page }) => {
  await loginAsParticipant(context);
  let failing = true;
  await page.route("**/api/platform/session/team/members", (route) =>
    failing
      ? route.fulfill({ status: 503, contentType: "application/json", body: '{"detail":"upstream store unavailable"}' })
      : route.fulfill({
          contentType: "application/json",
          body: JSON.stringify([{ player_id: 1002, display_name: "Alpha Mate", email: "mate@college.local", role: "member" }]),
        }),
  );
  const dialog = await openAccountSettings(page);
  await expect(dialog.getByText(/Couldn't load teammates/)).toBeVisible();
  await expect(dialog.getByText("No other teammates available yet.")).toHaveCount(0);
  failing = false;
  await dialog.getByRole("button", { name: "Try again" }).click();
  await expect(dialog.getByLabel("Teammate")).toBeVisible();
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
