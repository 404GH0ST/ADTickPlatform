import { expect, test, type APIRequestContext, type Locator, type Page } from "@playwright/test";

import { mockApiBaseUrl } from "./test-utils";

/*
 * A team's institution is display metadata joined into scoreboard rows and
 * attack events. Every theme must show it on the leaderboard, the attack
 * globe and the first-blood banner, in light and dark.
 */

const institutions = {
  "College Alpha": "Institut Teknologi Bandung",
  "College Beta": "Universitas Indonesia",
  "College Gamma": "Universitas Gadjah Mada",
};

const themes = ["graphite", "paper", "neubrutalism"] as const;
const modes = ["light", "dark"] as const;

test.use({ viewport: { width: 1280, height: 900 } });

async function reset(request: APIRequestContext, scenario: string, themeID: string) {
  const response = await request.post(`${mockApiBaseUrl}/__reset`, {
    data: { scenario, institutions },
  });
  expect(response.ok()).toBeTruthy();
  const theme = await request.put(`${mockApiBaseUrl}/api/v2/admin/platform/appearance`, {
    data: { theme_id: themeID },
  });
  expect(theme.ok()).toBeTruthy();
}

async function useMode(page: Page, baseURL: string | undefined, mode: string) {
  await page.context().addCookies([{ name: "ad-platform-theme", value: mode, url: baseURL! }]);
  await page.addInitScript((value) => localStorage.setItem("ad-platform-theme", value), mode);
}

async function revealGlobe(globe: Locator) {
  await expect(async () => {
    await globe.scrollIntoViewIfNeeded({ timeout: 1000 });
  }).toPass({ timeout: 7000 });
  await expect(globe).toHaveAttribute("data-surface-active", "true");
}

for (const themeID of themes) {
  for (const mode of modes) {
    test.describe(`${themeID} ${mode}`, () => {
      test("leaderboard shows each team's institution", async ({ page, request, baseURL }) => {
        await reset(request, "default", themeID);
        await useMode(page, baseURL, mode);
        await page.goto("/scoreboard");
        await expect(page.locator("html")).toHaveAttribute("data-scheme", themeID);
        await expect(page.locator("html")).toHaveAttribute("data-theme", mode);
        // Stock tables render desktop rows and mobile cards; check the visible one.
        const institution = page
          .getByTestId(themeID === "neubrutalism" ? "nb-podium-institution" : "scoreboard-team-institution")
          .filter({ visible: true })
          .first();
        await expect(institution).toBeVisible();
        await expect(page.getByText("Institut Teknologi Bandung").filter({ visible: true }).first()).toBeVisible();
        await page.screenshot({ path: `test-results/institution-scoreboard-${themeID}-${mode}.png`, fullPage: true });
      });

      test("globe labels and first-blood banner carry the institution", async ({ page, request, baseURL }) => {
        await reset(request, "empty-attacks", themeID);
        await useMode(page, baseURL, mode);
        await page.emulateMedia({ reducedMotion: "reduce" });
        await page.goto("/attacks");
        const globe = page.getByTestId("cyber-attack-map");
        await revealGlobe(globe);
        await reset(request, "realtime-first-blood-empty", themeID);

        // The intro and charge run first, so allow for the whole lead-in.
        await expect(globe).toHaveAttribute("data-first-blood-stage", "impact", { timeout: 15_000 });
        await expect(globe.getByTestId("first-blood-announcement")).toContainText("Universitas Gadjah Mada");
        await expect(globe.getByTestId("first-blood-announcement")).toContainText("Institut Teknologi Bandung");
        await globe.screenshot({ path: `test-results/institution-notice-${themeID}-${mode}.png` });

        const banner = globe.getByTestId("first-blood-banner");
        await expect(banner).toBeVisible();
        await expect(banner.getByTestId("first-blood-banner-team")).toHaveText("College Gamma");
        await expect(banner.getByTestId("first-blood-banner-institution")).toHaveText("Universitas Gadjah Mada");
        await expect(banner.getByTestId("first-blood-banner-victim-institution")).toContainText("Institut Teknologi Bandung");
        await globe.screenshot({ path: `test-results/institution-banner-${themeID}-${mode}.png` });
        await expect(banner).toHaveCount(0, { timeout: 6000 });

        const gammaLabel = globe.locator('[data-attack-team-label="true"][data-attack-team-id="College Gamma"]');
        await expect(gammaLabel).toHaveAttribute("data-attack-team-institution", "Universitas Gadjah Mada");
        await expect(gammaLabel.locator('[data-fx="team-institution"]')).toHaveText("Universitas Gadjah Mada");
        // The plate is sized for the institution: its text stays inside it.
        const fits = await gammaLabel.evaluate((label) => {
          const plate = label.querySelector('[data-fx="team-plate"]')!.getBoundingClientRect();
          const meta = label.querySelector('[data-fx="team-meta"]')!.getBoundingClientRect();
          return meta.left >= plate.left - 1 && meta.right <= plate.right + 1;
        });
        expect(fits).toBe(true);
        await globe.screenshot({ path: `test-results/institution-globe-${themeID}-${mode}.png` });
      });
    });
  }
}
