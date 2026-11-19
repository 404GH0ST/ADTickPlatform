import { chromium, expect, test, type APIRequestContext } from "@playwright/test";

import { expectNoHorizontalOverflow } from "./test-layout-utils";
import { mockApiBaseUrl, participantStorageState, resetMockApi } from "./test-utils";

/*
 * The neubrutalism theme replaces the shell, home, scoreboard, services, flag
 * form and login views. These tests drive the real participant features
 * through the redesigned views to prove they still work end to end.
 */

async function useTheme(request: APIRequestContext, themeID: string) {
  const response = await request.put(`${mockApiBaseUrl}/api/v2/admin/platform/appearance`, {
    data: { theme_id: themeID },
  });
  expect(response.ok()).toBeTruthy();
}

test.use({ viewport: { width: 1280, height: 900 } });

test.beforeEach(async ({ request }) => {
  await resetMockApi(request);
  await request.post(`${mockApiBaseUrl}/api/v2/admin/game/match/start`);
  await useTheme(request, "neubrutalism");
});

test.describe("as a participant", () => {
  test.use({ storageState: participantStorageState });

  test("home and scoreboard render the theme's views", async ({ page }) => {
    await page.goto("/");
    await expect(page.getByTestId("nb-overview")).toBeVisible();
    await page.getByTestId("nb-overview").getByRole("link", { name: /Scoreboard/ }).click();

    await expect(page).toHaveURL(/\/scoreboard$/);
    await expect(page.getByTestId("nb-scoreboard")).toBeVisible();
    await expect(page.getByTestId("participant-scoreboard-card")).toHaveCount(0);
    await expect(page.getByTestId("nb-score-row").first()).toBeVisible();
    await expect(page.locator('[data-current-team="true"]')).toHaveCount(1);
  });

  test("services features work through the redesigned view", async ({ page }) => {
    await page.goto("/services");
    const card = page.getByTestId("nb-service-svc-1");
    await expect(card).toBeVisible();
    await expect(page.getByTestId("service-card-svc-1")).toHaveCount(0);
    await expect(card.getByRole("link", { name: "Download Source" })).toHaveAttribute(
      "href",
      "/api/platform/challenges/1/source",
    );
    await expect(card.getByRole("button", { name: "SSH Locked" })).toBeDisabled();

    // Flag submission (themed form) unlocks SSH.
    await page.getByRole("textbox", { name: "Flags", exact: true }).fill("PLAYIT{enemy-challenge-1}");
    await page.getByRole("button", { name: "Submit", exact: true }).click();
    await expect(page.locator('[data-testid="flag-verdict"][data-verdict="accepted"]')).toBeVisible();
    await expect(card.getByRole("button", { name: "SSH Access" })).toBeEnabled();

    // SSH session dialog (stock, restyled): credentials are requested on open by the shared logic.
    await card.getByRole("button", { name: "SSH Access" }).click();
    const sshDialog = page.getByRole("dialog", { name: "SSH Access" });
    await expect(sshDialog.getByText("ssh root@10.80.50.11 -p 22").first()).toBeVisible();
    await sshDialog.getByRole("button", { name: "Close" }).first().click();

    // Restart and its cooldown.
    const notice = card.getByTestId(/^service-action-notice-/);
    await card.getByRole("button", { name: /^Restart/ }).click();
    await expect(notice).toContainText("Restart complete");

    // Factory reset through the confirmation dialog, then cooldown.
    await card.getByRole("button", { name: /^Factory Reset/ }).click();
    const resetDialog = page.getByRole("dialog", { name: "Factory Reset Service" });
    await resetDialog.getByRole("button", { name: "Confirm Reset" }).click();
    await expect(notice).toContainText("Factory reset complete");
    await expect(card.getByText(/factory reset cooldown: \d+s/)).toBeVisible();
  });

  test("enlarged flag verdicts show the reason and support keyboard retries", async ({ baseURL }) => {
    const browser = await chromium.launch({ args: ["--blink-settings=defaultFontSize=32"] });
    try {
      const page = await browser.newPage({ baseURL, storageState: participantStorageState, viewport: { width: 320, height: 700 } });
      let releaseSubmission!: () => void;
      const pendingSubmission = new Promise<void>((resolve) => {
        releaseSubmission = resolve;
      });
      await page.route("**/api/platform/submit", async (route) => {
        await pendingSubmission;
        await route.continue();
      });

      await page.goto("/services");
      const field = page.getByRole("textbox", { name: "Flags", exact: true });
      await expect(field).toHaveAccessibleDescription(/One per line or comma-separated/);
      const flag = "PLAYIT{audit-invalid-0}";
      await field.fill(flag);
      await field.press("Control+Enter");
      try {
        await expect(field).toBeFocused();
        await expect(field).not.toBeEditable();
      } finally {
        releaseSubmission();
      }

      const verdicts = page.getByRole("list", { name: "Flag verdicts" });
      const verdict = verdicts.getByTestId("flag-verdict");
      await expect(verdict).toHaveAttribute("data-verdict", "rejected");
      await expect(verdict).toContainText(flag);
      const reason = verdict.getByText("flag is wrong or expired.");
      expect(await reason.evaluate((element) => {
        const reasonBounds = element.getBoundingClientRect();
        const scrollport = element.closest("ul")!.parentElement!;
        const viewport = scrollport.getBoundingClientRect();
        return reasonBounds.bottom <= viewport.bottom && scrollport.scrollWidth === scrollport.clientWidth;
      })).toBe(true);

      await expect(field).toBeFocused();
      await page.keyboard.press("End");
      await page.keyboard.type("x");
      await expect(field).toHaveValue(`${flag}x`);

      await field.fill(Array.from({ length: 12 }, (_, index) => `PLAYIT{audit-invalid-${index}}`).join("\n"));
      await field.press("Control+Enter");
      await expect(verdict).toHaveCount(12);
      const results = page.getByRole("region", { name: "Flag results" });
      await expect(results).toHaveAttribute("tabindex", "0");
      await results.press("End");
      await expect.poll(() => results.evaluate((element) => element.scrollTop)).toBeGreaterThan(0);

      await page.unroute("**/api/platform/submit");
      await page.route("**/api/platform/submit", (route) => route.fulfill({
        status: 503,
        json: { detail: "Flag submission temporarily unavailable" },
      }));
      await field.fill(flag);
      await field.press("Control+Enter");
      await expect(field).toHaveAccessibleDescription(/Flag submission temporarily unavailable/);
    } finally {
      await browser.close();
    }
  });

  test("a short reset dialog scrolls from the service identity to the warning", async ({ page }) => {
    await page.setViewportSize({ width: 320, height: 320 });
    await page.goto("/services");
    const reset = page.getByTestId("nb-service-svc-1").getByRole("button", { name: "Factory Reset" });
    await reset.press("Enter");

    const dialog = page.getByRole("dialog", { name: "Factory Reset Service" });
    const heading = dialog.getByRole("heading", { name: "Factory Reset Service" });
    await expect(heading).toBeFocused();
    await expect(dialog.getByText("college-http", { exact: true })).toBeInViewport();
    await expect(dialog.getByText("10.80.50.11:30050", { exact: true })).toBeInViewport();

    await page.keyboard.press("PageDown");
    await expect.poll(() => dialog.evaluate((element) => element.scrollTop)).toBeGreaterThan(0);
    const warning = dialog.getByText(/Factory reset deletes stored flags/);
    await expect(warning).toBeInViewport({ ratio: 0.5 });

    await page.keyboard.press("Escape");
    await expect(dialog).toBeHidden();
    await expect(reset).toBeFocused();
  });

  test("theme default mode applies until the participant picks one", async ({ page }) => {
    await page.emulateMedia({ colorScheme: "dark" });
    await page.goto("/scoreboard");
    // Neubrutalism defaults to light even on a dark OS.
    await expect(page.locator("html")).toHaveAttribute("data-theme", "light");

    await page.locator(".mode-picker summary").click();
    await page.locator('[data-mode-option="dark"]').click();
    await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
    await page.reload();
    await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  });

  test("header popovers fit the viewport across mobile breakpoints", async ({ page }) => {
    const appearance = page.locator(".mode-picker > summary");
    const menu = page.getByTestId("nb-account-menu").locator("summary");

    for (const width of [320, 390, 639, 640]) {
      await page.setViewportSize({ width, height: 740 });
      await page.goto("/services");

      for (const [trigger, panel] of [
        [appearance, page.locator(".mode-picker > div")],
        [menu, page.locator(".nb-menu-panel")],
      ] as const) {
        await trigger.click();
        await expect(panel).toBeVisible();
        const bounds = await panel.boundingBox();
        if (!bounds) throw new Error("Expected an open header popover");
        expect(bounds.x).toBeGreaterThanOrEqual(0);
        expect(bounds.x + bounds.width).toBeLessThanOrEqual(width);
        for (const option of await panel.getByRole("radio").all()) {
          expect(await option.evaluate((element) => {
            const label = document.createRange();
            label.selectNodeContents(element.querySelector("span")!);
            const labelBounds = label.getBoundingClientRect();
            const controlBounds = element.getBoundingClientRect();
            return label.getClientRects().length === 1 &&
              labelBounds.left >= controlBounds.left && labelBounds.right <= controlBounds.right;
          })).toBe(true);
        }
        await trigger.click();
      }
    }
  });

  test("header disclosures keep keyboard focus predictable", async ({ page }) => {
    await page.setViewportSize({ width: 320, height: 740 });
    await page.goto("/services");

    const appearance = page.locator(".mode-picker > summary");
    await appearance.focus();
    await page.keyboard.press("Enter");
    await page.keyboard.press("Tab");
    await expect(page.getByRole("radio", { name: /^Light/ })).toBeFocused();
    await page.keyboard.press("ArrowRight");
    await expect(page.getByRole("radio", { name: /^Dark/, checked: true })).toBeFocused();
    await page.keyboard.press("Escape");
    await expect(appearance).toBeFocused();
    await expect(page.getByRole("radiogroup", { name: "Color mode" })).toBeHidden();

    const menu = page.getByTestId("nb-account-menu").locator("summary");
    await menu.focus();
    await page.keyboard.press("Enter");
    await page.getByRole("button", { name: "Account settings" }).focus();
    await page.keyboard.press("Enter");
    const dialog = page.getByRole("dialog", { name: "Account settings" });
    await expect(dialog).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(dialog).toBeHidden();
    await expect(menu).toBeFocused();
  });

  test("enlarged service facts and the match clock fit inside their panels", async ({ baseURL, request }) => {
    const browser = await chromium.launch({ args: ["--blink-settings=defaultFontSize=32"] });
    try {
      const page = await browser.newPage({
        baseURL,
        storageState: participantStorageState,
        viewport: { width: 320, height: 740 },
      });
      const clock = page.getByRole("region", { name: "Match progress" });
      const service = page.getByTestId("nb-service-svc-1");

      for (const mode of ["light", "dark"]) {
        await page.context().addCookies([{ name: "ad-platform-theme", value: mode, url: baseURL! }]);
        await page.goto("/services");
        await expect(page.locator("html")).toHaveAttribute("data-theme", mode);
        await expect(clock.getByText("Scheduling…", { exact: true })).toBeVisible();
        await expect(service.getByText(/to unlock SSH automatically/)).toBeVisible();
        await page.evaluate(() => document.fonts.ready);
        await expectNoHorizontalOverflow(service);
        await expectNoHorizontalOverflow(clock);
      }

      const started = await request.post(`${mockApiBaseUrl}/api/v2/admin/game/scheduler/start`);
      expect(started.ok()).toBeTruthy();
      await page.reload();
      await expect(clock.locator(".nb-countdown")).toBeVisible();
      await page.evaluate(() => document.fonts.ready);
      await expectNoHorizontalOverflow(clock);
    } finally {
      await browser.close();
    }
  });

  test("the manual fits and the attack table scrolls by keyboard on mobile", async ({ page }) => {
    await page.setViewportSize({ width: 320, height: 660 });
    await page.goto("/docs/participant");
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(320);

    await page.goto("/attacks");
    const region = page.getByRole("region", { name: "Accepted attacks" });
    await expect(region).toHaveAttribute("tabindex", "0");
    await region.press("ArrowRight");
    await expect.poll(() => region.evaluate((element) => element.scrollLeft)).toBeGreaterThan(0);
  });

  test("attack map view controls stay in place when the selected view changes", async ({ page }) => {
    await page.emulateMedia({ reducedMotion: "reduce" });
    await page.goto("/attacks");
    const group = page.getByRole("group", { name: "View", exact: true });
    for (const width of [1024, 1440]) {
      await page.setViewportSize({ width, height: 900 });
      await page.evaluate(() => document.fonts.ready);
      const all = group.getByRole("button", { name: "All Feed", exact: true });
      await all.press("Enter");
      const positions = () => group.getByRole("button").evaluateAll((buttons) =>
        buttons.map((button) => {
          const { x, y, width, height } = button.getBoundingClientRect();
          return { x, y: y + window.scrollY, width, height };
        }),
      );
      const initial = await positions();
      expect(initial).toHaveLength(3);
      for (const name of ["Recent", "Current Tick"]) {
        const button = group.getByRole("button", { name, exact: true });
        await button.press("Enter");
        await expect(button).toHaveAttribute("aria-pressed", "true");
        expect(await positions()).toEqual(initial);
      }
    }
  });

  test("attack map view and sound controls keep 44px touch targets on phones and tablets", async ({ browser, baseURL }) => {
    const page = await browser.newPage({ baseURL, hasTouch: true, isMobile: true, storageState: participantStorageState });
    try {
      for (const width of [390, 768]) {
        await page.setViewportSize({ width, height: 850 });
        await page.goto("/attacks");
        const view = page.getByRole("group", { name: "View", exact: true });
        await expect(view).toBeVisible();
        for (const control of [
          page.getByRole("button", { name: "SFX: Attack sound" }),
          ...await view.getByRole("button").all(),
        ]) {
          const size = await control.boundingBox();
          expect(size?.width).toBeGreaterThanOrEqual(44);
          expect(size?.height).toBeGreaterThanOrEqual(44);
        }
      }
    } finally {
      await page.context().close();
    }
  });

  test("long live first-blood names fit outside the globe canvas with enlarged text", async ({ baseURL, request }) => {
    await resetMockApi(request, "empty-attacks");
    await useTheme(request, "neubrutalism");
    const browser = await chromium.launch({ args: ["--blink-settings=defaultFontSize=32"] });
    try {
      const page = await browser.newPage({
        baseURL,
        storageState: participantStorageState,
        viewport: { width: 320, height: 740 },
        reducedMotion: "reduce",
      });
      await page.goto("/attacks");
      const globe = page.getByTestId("cyber-attack-map");
      const canvas = globe.locator('[data-fx="map-canvas"]');
      await canvas.scrollIntoViewIfNeeded();
      await expect(globe).toHaveAttribute("data-surface-active", "true");
      await resetMockApi(request, "realtime-first-blood-long-names");
      const banner = globe.getByTestId("first-blood-banner");
      await expect(banner).toBeVisible({ timeout: 12_000 });
      await expect(canvas.getByTestId("first-blood-banner")).toHaveCount(0);
      await expect(banner.getByTestId("first-blood-banner-team")).toHaveText(/International Cyber Defense/);
      await expect(banner.getByTestId("first-blood-banner-challenge")).toHaveText(/RegionalReplicasAndSecondaryFailover$/);
      await page.evaluate(() => document.fonts.ready);
      await expectNoHorizontalOverflow(banner);
      const fittingNames = await banner.evaluate((element) => {
        const panel = element.getBoundingClientRect();
        return Array.from(element.querySelectorAll('[data-testid^="first-blood-banner-"]')).every((name) => {
          const range = document.createRange();
          range.selectNodeContents(name);
          return Array.from(range.getClientRects()).every((line) => line.left >= panel.left && line.right <= panel.right);
        });
      });
      expect(fittingNames).toBe(true);

      const retained = globe.getByTestId("first-blood-result");
      await expect(retained).toBeVisible({ timeout: 6500 });
      await expect(retained.getByTestId("first-blood-banner-challenge")).toHaveText(/RegionalReplicasAndSecondaryFailover$/);
      await retained.getByRole("button", { name: "Dismiss first blood" }).press("Enter");
      await expect(retained).toHaveCount(0);
      await expect(globe.locator('[data-fx="map-control"]').first()).toBeFocused();
    } finally {
      await browser.close();
    }
  });
});

test("organizer tabs keep stock content inside the theme's shell", async ({ page }) => {
  // Default storage state is the organizer session.
  await page.goto("/admin/game?view=settings");
  await expect(page.getByTestId("nb-shell")).toBeVisible();
  await expect(page.getByTestId("theme-settings-card")).toBeVisible();

  await page.goto("/admin");
  await expect(page.getByTestId("trusted-state-panel")).toBeVisible();
  const announcements = page.getByTestId("announcements-panel");
  await announcements.locator("textarea").fill("Theme test: warmup in 5 minutes.");
  await announcements.getByRole("button", { name: "Publish" }).click();
  await expect(announcements.getByText("Announcement published to participants.")).toBeVisible();
  await expect(announcements.getByText("Theme test: warmup in 5 minutes.")).toBeVisible();
});

test("mobile navigation reveals each link as keyboard focus moves", async ({ baseURL, context }) => {
  const browser = await chromium.launch({ args: ["--blink-settings=defaultFontSize=32"] });
  try {
    const enlarged = await browser.newContext({
      baseURL,
      storageState: await context.storageState(),
      viewport: { width: 320, height: 700 },
    });
    const page = await enlarged.newPage();
    await page.goto("/admin");
    await page.evaluate(() => document.fonts.ready);
    const nav = page.getByRole("navigation", { name: "Primary", exact: true });
    const links = await nav.getByRole("link").all();
    expect(links.length).toBeGreaterThan(1);

    await page.getByRole("link", { name: "Skip to content" }).focus();
    await page.keyboard.press("Tab"); // Brand link.

    const expectVisibleFocus = async (link: (typeof links)[number]) => {
      await expect(link).toBeFocused();
      await expect.poll(() => link.evaluate((element) => {
        const bounds = element.getBoundingClientRect();
        const strip = element.closest("nav")!.getBoundingClientRect();
        return bounds.left >= strip.left - 1 && bounds.right <= strip.right + 1;
      })).toBe(true);
    };

    for (const link of links) {
      await page.keyboard.press("Tab");
      await expectVisibleFocus(link);
    }
    for (const link of links.slice(0, -1).reverse()) {
      await page.keyboard.press("Shift+Tab");
      await expectVisibleFocus(link);
    }
  } finally {
    await browser.close();
  }
});

test("the desktop sidebar animates both ways and honors reduced motion", async ({ page }) => {
  await page.emulateMedia({ reducedMotion: "no-preference" });
  await page.goto("/admin/scoreboard");
  const toggle = page.getByRole("button", { name: /sidebar/ });
  const surface = page.locator(".nb-rail-motion");

  for (const expanded of ["false", "true"]) {
    const transition = await toggle.evaluate((button) => {
      (button as HTMLButtonElement).click();
      const motion = button.closest(".nb-rail-motion");
      return motion?.getAnimations().some((animation) =>
        animation instanceof CSSTransition && animation.transitionProperty === "clip-path");
    });
    expect(transition).toBe(true);
    await expect(toggle).toHaveAttribute("aria-expanded", expanded);
    await expect.poll(() => surface.evaluate((element) => element.getAnimations().length)).toBe(0);
  }

  await page.emulateMedia({ reducedMotion: "reduce" });
  await toggle.press("Enter");
  await expect(toggle).toHaveAttribute("aria-expanded", "false");
  await expect(surface).toHaveCSS("transition-property", "none");
});

test("organizer scoreboard fits long team names and large service scores", async ({ page }) => {
  const teams = ["East", "West"].map(
    (suffix) => `Northern Collegiate Infrastructure and Security Research Team ${suffix}`,
  );
  const services = [1, 2, 3].map((id) => ({
    challenge_id: id,
    service: `Scoring fixture ${id}`,
    attack: 6530.25,
    defense: id === 1 ? -1 : -3678.75,
    sla: 2801.59,
    total: 13010.59,
  }));
  const rows = teams.map((team, index) => ({
    rank: index + 100,
    team,
    attack: 155309.75,
    defense: -367891.75,
    sla: 5120.09,
    total: index === 0 ? 9999999.96 : -27150.96,
    delta: index === 0 ? "+12" : "-10",
    services,
  }));
  await page.route("**/api/admin/realtime/game/scoreboard/stream", (route) =>
    route.fulfill({
      contentType: "text/event-stream",
      body: `data: ${JSON.stringify(rows)}\n\n`,
    }),
  );

  await page.setViewportSize({ width: 320, height: 850 });
  await page.goto("/admin/scoreboard");
  const standings = page.getByTestId("nb-score-row");
  const teamName = standings.first().getByText(teams[0], { exact: true });
  await expect(teamName).toBeVisible();
  await page.evaluate(() => document.fonts.ready);
  await expect.poll(() => teamName.evaluate((name) => name.scrollWidth - name.clientWidth)).toBeLessThanOrEqual(0);
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(320);

  const total = standings.first().getByText("9999999.96", { exact: true });
  function fitsOnOneLine(element: Element) {
    const text = element.lastChild;
    if (!(text instanceof Text)) return false;
    const range = document.createRange();
    range.selectNode(text);
    const lines = range.getClientRects();
    return lines.length === 1 && lines[0].width <= element.clientWidth + 1;
  }

  for (const width of [320, 390, 640, 768, 1024, 1280]) {
    await page.setViewportSize({ width, height: 850 });
    for (const standing of await standings.all()) {
      const rank = standing.locator(":scope > span").first();
      await expectNoHorizontalOverflow(rank);
      expect(await rank.locator(":scope > span").first().evaluate(fitsOnOneLine),
        `rank fits on one line at ${width}px`).toBe(true);
    }
    expect(await total.evaluate(fitsOnOneLine), `total fits on one line at ${width}px`).toBe(true);

    const label = standings.first().getByText("Total", { exact: true });
    expect(await label.evaluate((element) => getComputedStyle(element).position === "absolute"),
      `Total is labeled at ${width}px`).toBe(width >= 768);

    if (width >= 768) {
      expect(await standings.evaluateAll((rows) => {
        const headings = rows[0].parentElement!.previousElementSibling!;
        return rows.every((row) => [...headings.children].every((heading, index) => {
          const column = heading.getBoundingClientRect();
          const cell = row.children[index].getBoundingClientRect();
          return Math.abs(column.left - cell.left) <= 1 && Math.abs(column.right - cell.right) <= 1;
        }));
      }), `headings and rows share columns at ${width}px`).toBe(true);
    }
  }

  const cards = page.getByTestId("nb-service-score");
  await expect(cards).toHaveCount(6);
  const stacked = await cards.evaluateAll((cards) => cards.every((card) => {
    const [attack, defense, sla] = Array.from(card.querySelectorAll(".nb-sticker"),
      (chip) => chip.getBoundingClientRect(),
    );
    return Math.abs(attack.width - defense.width) <= 1 && attack.top === defense.top &&
      sla.top > defense.top && sla.left === attack.left && sla.right === defense.right;
  }));
  expect(stacked).toBe(true);

  await page.setViewportSize({ width: 390, height: 850 });
  const singleRow = await cards.evaluateAll((cards) => cards.every((card) => {
    const [attack, defense, sla] = Array.from(card.querySelectorAll(".nb-sticker"),
      (chip) => chip.getBoundingClientRect(),
    );
    return Math.abs(attack.width - defense.width) <= 1 && sla.width > attack.width &&
      attack.top === defense.top && defense.top === sla.top;
  }));
  expect(singleRow).toBe(true);

  await page.setViewportSize({ width: 320, height: 850 });
  const enlargedText = await page.addStyleTag({ content: "html { font-size: 32px; }" });
  expect(await total.evaluate(fitsOnOneLine), "total fits at 200% text size").toBe(true);
  await expect(cards.first().getByText("Scoring fixture 1", { exact: true })).toBeVisible();
  const fullRows = await cards.evaluateAll((cards) => cards.every((card) => {
    const [attack, defense, sla] = Array.from(card.querySelectorAll(".nb-sticker"),
      (chip) => chip.getBoundingClientRect(),
    );
    return attack.top < defense.top && defense.top < sla.top &&
      attack.left === sla.left && defense.left === sla.left &&
      attack.right === sla.right && defense.right === sla.right;
  }));
  expect(fullRows).toBe(true);
  await enlargedText.evaluate((element) => element.parentNode?.removeChild(element));

  await page.setViewportSize({ width: 1024, height: 850 });
  const hasOverflow = await cards.evaluateAll((cards) =>
    cards.some((card) => {
      const bounds = card.getBoundingClientRect();
      return Array.from(card.querySelectorAll(".nb-sticker")).some((chip) => {
        const { left, right } = chip.getBoundingClientRect();
        return left < bounds.left || right > bounds.right;
      });
    }),
  );
  expect(hasOverflow).toBe(false);
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(1024);
});

test("organizer details follow the live roster size until explicitly changed", async ({ page }) => {
  const service = { challenge_id: 1, service: "Fixture service", attack: 5, defense: 4, sla: 3, total: 12 };
  const rows = Array.from({ length: 12 }, (_, index) => ({
    rank: index + 1, team: `Fixture team ${index + 1}`, attack: 5, defense: 4, sla: 3, total: 12, delta: "0", services: [service],
  }));
  let publish!: () => void;
  const update = new Promise<void>((resolve) => { publish = resolve; });
  await page.route("**/api/admin/realtime/game/scoreboard/stream", async (route) => {
    await update;
    await route.fulfill({
      contentType: "text/event-stream",
      body: `data: ${JSON.stringify(rows)}\n\n`,
    });
  });

  await page.goto("/admin/scoreboard");
  const toggle = page.getByRole("button", { name: "Per-service scores" });
  const scores = page.getByTestId("nb-service-score");
  await expect(toggle).toHaveAttribute("aria-pressed", "true");
  publish();
  await expect(page.getByTestId("nb-score-row")).toHaveCount(rows.length);
  await expect(toggle).toHaveAttribute("aria-pressed", "false");
  await expect(scores).toHaveCount(0);

  const filter = page.getByLabel("Team filter");
  await filter.fill("Fixture team 2");
  await expect(page.getByTestId("nb-score-row")).toHaveCount(1);
  await expect(toggle).toHaveAttribute("aria-pressed", "false");
  await toggle.click();
  await filter.clear();
  await expect(toggle).toHaveAttribute("aria-pressed", "true");
  await expect(scores).toHaveCount(rows.length);
});

test("the player registry fits on mobile and returns focus after closing its form", async ({ page }) => {
  await page.setViewportSize({ width: 320, height: 450 });
  await page.goto("/admin/players");
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(320);

  const create = page.getByRole("button", { name: "Create Player" });
  await create.press("Enter");
  const dialog = page.getByRole("dialog", { name: "Create player" });
  await expect(dialog).toBeVisible();
  await expect(dialog.getByRole("combobox", { name: "Team" })).toBeFocused();
  await page.keyboard.press("Escape");
  await expect(dialog).toBeHidden();
  await expect(create).toBeFocused();
});

test("appearance shows the checked mode in forced colors", async ({ page }) => {
  await page.emulateMedia({ forcedColors: "active" });
  await page.goto("/scoreboard");
  const appearance = page.locator(".mode-picker summary");
  await appearance.press("Enter");

  const light = page.getByRole("radio", { name: /^Light/ });
  await expect(light).toHaveAttribute("aria-checked", "true");
  await expect(light.locator(".lucide-check")).toBeVisible();
  const dark = page.getByRole("radio", { name: /^Dark/ });
  await dark.click();
  await appearance.focus();
  await expect(dark).toHaveAttribute("aria-checked", "true");
  await expect(dark.locator(".lucide-check")).toBeVisible();
  await expect(light.locator(".lucide-check")).not.toBeVisible();

  await page.goto("/admin/scoreboard");
  const filter = page.getByLabel("Team filter");
  await filter.focus();
  await expect(filter).toBeFocused();
  await expect(filter).toHaveCSS("outline-style", "solid");
  await expect(filter).toHaveCSS("outline-width", "2px");
});

test("selects use native appearance when the forced palette differs from the app mode", async ({ page }) => {
  await page.addInitScript(() => localStorage.setItem("ad-platform-theme", "dark"));
  await page.emulateMedia({ colorScheme: "light", forcedColors: "active" });
  await page.goto("/admin/game?view=diagnostics");
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  const audit = page.getByTestId("scheduler-audit-disclosure");
  await expect(audit).toBeVisible();
  if (await audit.getAttribute("open") === null) await audit.locator("summary").click();
  const size = audit.getByLabel("Page Size", { exact: true });
  await expect(size).toHaveCSS("appearance", "auto");
  await expect(size).toHaveCSS("background-image", "none");
  await size.selectOption("24");
  await expect(size).toHaveValue("24");
});

test("organizers can read the team deletion consequences in a short window", async ({ page }) => {
  await page.setViewportSize({ width: 320, height: 320 });
  await page.addInitScript(() => localStorage.setItem("ad-platform-theme", "dark"));
  await page.goto("/admin/teams");
  const opener = page.getByRole("button", { name: "Actions for College Alpha" });
  const triggerBounds = await opener.boundingBox();
  expect(triggerBounds?.width).toBeGreaterThanOrEqual(44);
  expect(triggerBounds?.height).toBeGreaterThanOrEqual(44);
  const cell = await opener.locator("..").boundingBox();
  const actionsHeader = await page.getByRole("columnheader", { name: "Actions" }).boundingBox();
  expect(cell?.x).toBeCloseTo(actionsHeader?.x ?? 0);
  expect(cell?.width).toBeCloseTo(actionsHeader?.width ?? 0);
  await opener.press("Enter");
  const actions = page.getByRole("group", { name: "College Alpha", exact: true });
  await expect(actions).toBeVisible();
  await expect(actions.getByRole("button", { name: "Edit team College Alpha" })).toBeVisible();
  await expect(actions.getByRole("button", { name: "Deactivate team College Alpha" })).toBeVisible();
  await actions.getByRole("button", { name: "Delete team College Alpha" }).press("Enter");
  const dialog = page.getByRole("dialog", { name: "Delete team" });
  await expect(dialog.getByRole("heading")).toBeFocused();
  await page.keyboard.press("PageDown");
  const consequence = dialog.getByText(/All associated player accounts/);
  await expect(consequence).toBeInViewport({ ratio: 0.5 });
  const warningColor = await dialog.getByTestId("delete-impact-panel").evaluate((panel) => getComputedStyle(panel).color);
  await expect(consequence).toHaveCSS("color", warningColor);
  await page.keyboard.press("Escape");
  await expect(opener).toBeFocused();
});

test("participant and organizer pages reflow with a 32px browser default font on mobile", async ({ baseURL, context }) => {
  const browser = await chromium.launch({ args: ["--blink-settings=defaultFontSize=32"] });
  try {
    const cases = [
      { storageState: participantStorageState, routes: ["/", "/services", "/scoreboard", "/attacks", "/docs/participant"] },
      { storageState: await context.storageState(), routes: ["/admin/players", "/admin/scoreboard", "/admin/submissions"] },
    ];
    for (const { storageState, routes } of cases) {
      const enlarged = await browser.newContext({ baseURL, storageState, viewport: { width: 320, height: 700 } });
      const page = await enlarged.newPage();
      for (const route of routes) {
        await page.goto(route);
        await expect(page.locator("html")).toHaveAttribute("data-scheme", "neubrutalism");
        await page.evaluate(() => document.fonts.ready);
        const layout = await page.evaluate(() => ({
          font: getComputedStyle(document.documentElement).fontSize,
          width: document.documentElement.scrollWidth,
        }));
        expect(layout, route).toEqual({ font: "32px", width: 320 });
      }
      await enlarged.close();
    }
  } finally {
    await browser.close();
  }
});

test("populated organizer views and scheduling values fit at narrow widths with enlarged text", async ({ baseURL, context, request }) => {
  await resetMockApi(request, "submissions");
  await useTheme(request, "neubrutalism");
  const browser = await chromium.launch({ args: ["--blink-settings=defaultFontSize=32"] });
  try {
    const enlarged = await browser.newContext({
      baseURL,
      storageState: await context.storageState(),
      viewport: { width: 320, height: 700 },
      locale: "en-US",
      timezoneId: "UTC",
    });
    const page = await enlarged.newPage();
    const cases = [
      { route: "/admin", ready: page.getByTestId("announcements-panel") },
      { route: "/admin/teams", ready: page.getByTestId("bulk-import-panel") },
      { route: "/admin/attacks", ready: page.getByRole("heading", { name: "Accepted Attacks" }) },
      { route: "/admin/game?view=settings", ready: page.getByRole("radio", { name: /Neubrutalism/ }) },
      { route: "/admin/game?view=health", ready: page.getByTestId("operations-metrics-card") },
      { route: "/admin/game?view=diagnostics", ready: page.getByTestId("scheduler-card") },
      { route: "/admin/game", ready: page.getByLabel("Scheduled Start Date") },
    ];
    const expectReflow = async (route: string) => {
      await expect(page.locator("html")).toHaveAttribute("data-scheme", "neubrutalism");
      const layout = await page.evaluate(async () => {
        await document.fonts.ready;
        return {
          font: getComputedStyle(document.documentElement).fontSize,
          width: document.documentElement.scrollWidth,
        };
      });
      expect(layout, route).toEqual({ font: "32px", width: 320 });
    };

    for (const { route, ready } of cases) {
      await page.goto(route);
      await expect(ready).toBeVisible();
      await expectReflow(route);
    }

    await page.getByLabel("Scheduled Start Date").fill("2029-06-07");
    const time = page.getByLabel("Scheduled Start Time");
    await time.fill("19:45");
    await expect(time).toHaveAccessibleDescription(/Jun 7, 2029.*7:45 PM/);
    await expect(page.locator("time[datetime='2029-06-07T19:45:00.000Z']")).toBeVisible();
    await expectReflow("filled match schedule");

    await page.goto("/admin/scoreboard");
    const freeze = page.getByRole("group", { name: "Freeze at", exact: true });
    const freezeTime = freeze.getByLabel("Time", { exact: true });
    await freezeTime.fill("19:45");
    await freeze.getByLabel("Date", { exact: true }).fill("2029-06-07");
    await expect(freezeTime).toHaveAccessibleDescription(/Jun 7, 2029.*7:45 PM/);
    await expect(page.locator("time[datetime='2029-06-07T19:45:00.000Z']")).toBeVisible();
    await expectReflow("filled freeze schedule");
  } finally {
    await browser.close();
  }
});

test("login works through the redesigned form", async ({ browser }) => {
  const context = await browser.newContext({ storageState: { cookies: [], origins: [] } });
  const page = await context.newPage();
  await page.goto("/login");
  await expect(page.getByTestId("nb-login")).toBeVisible();
  await page.getByLabel("Email").fill("alpha.captain@college.local");
  await page.getByLabel("Password").fill("alpha-password");
  await page.locator('form button[type="submit"]').click();
  await expect(page).toHaveURL(/\/services$/);
  await expect(page.getByTestId("nb-services")).toBeVisible();
  await context.close();
});

test("account mode labels fit at narrow widths with enlarged text", async ({ baseURL }) => {
  const browser = await chromium.launch({ args: ["--blink-settings=defaultFontSize=32"] });
  try {
    const page = await browser.newPage({
      baseURL,
      storageState: { cookies: [], origins: [] },
      viewport: { width: 320, height: 700 },
    });
    await page.goto("/login");
    await page.evaluate(() => document.fonts.ready);
    expect(await page.evaluate(() => getComputedStyle(document.documentElement).fontSize)).toBe("32px");
    const modes = page.getByRole("group", { name: "Account mode" });
    const register = modes.getByRole("button", { name: "New member" });
    await register.press("Enter");
    await expect(register).toHaveAttribute("aria-pressed", "true");
    await expect(page.getByLabel("Display name")).toBeVisible();

    for (const button of await modes.getByRole("button").all()) {
      expect(await button.evaluate((element) => {
        const label = document.createRange();
        label.selectNodeContents(element);
        const buttonBounds = element.getBoundingClientRect();
        const fits = Array.from(label.getClientRects()).every((line) =>
          line.left >= buttonBounds.left && line.right <= buttonBounds.right,
        );
        const text = element.firstChild!;
        const words = [...(text.textContent ?? "").matchAll(/\S+/g)];
        return fits && words.every((word) => {
          label.setStart(text, word.index);
          label.setEnd(text, word.index + word[0].length);
          return label.getClientRects().length === 1;
        });
      })).toBe(true);
    }
  } finally {
    await browser.close();
  }
});
