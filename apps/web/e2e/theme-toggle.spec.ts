import { expect, test } from "@playwright/test";

test.use({ viewport: { width: 1280, height: 900 } });

test("theme toggle persists dark mode across reload and organizer navigation", async ({
  page,
}) => {
  await page.emulateMedia({ colorScheme: "light" });

  await page.goto("/");
  await page.evaluate(() => {
    window.localStorage.removeItem("ad-platform-theme");
    window.localStorage.removeItem("ad-platform-scheme");
  });
  await page.reload();

  // Default: Graphite scheme + system mode (light OS → light).
  await expect(page.locator("html")).toHaveAttribute("data-theme", "light");
  await expect(page.locator("html")).toHaveAttribute("data-scheme", "graphite");

  await page.evaluate(() => {
    window.localStorage.setItem("ad-platform-theme", "light");
    document.cookie = "ad-platform-theme=light; path=/; max-age=31536000; samesite=lax";
    document.documentElement.dataset.theme = "light";
    document.documentElement.dataset.themePreference = "light";
  });
  await page.reload();

  await expect(page.locator("html")).toHaveAttribute("data-theme", "light");
  await expect(page.locator('[data-mode-option="light"]')).toBeVisible();

  // Collapsed mode chip; expand on hover then pick Dark
  await selectExpandedOption(page, ".mode-picker", '[data-mode-option="dark"]');

  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  await expect(page.locator("html")).toHaveAttribute("data-theme-preference", "dark");
  await expect
    .poll(() => page.evaluate(() => window.localStorage.getItem("ad-platform-theme")))
    .toBe("dark");

  await page.reload();

  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  await expect(page.locator('[data-mode-option="dark"]')).toBeVisible();

  await page.goto("/admin");

  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  await expect(page.locator("html")).toHaveAttribute("data-scheme", "graphite");
  await expect(page.locator('[data-mode-option="dark"]')).toBeVisible();
});

test("system theme follows OS color scheme", async ({ page }) => {
  await page.emulateMedia({ colorScheme: "dark" });

  await page.goto("/");
  await page.evaluate(() => {
    window.localStorage.setItem("ad-platform-theme", "system");
    document.cookie = "ad-platform-theme=system; path=/; max-age=31536000; samesite=lax";
  });
  await page.reload();

  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  await expect(page.locator("html")).toHaveAttribute("data-theme-preference", "system");
  await expect(page.locator('[data-mode-option="system"]')).toBeVisible();

  await page.emulateMedia({ colorScheme: "light" });
  await expect
    .poll(() => page.locator("html").getAttribute("data-theme"))
    .toBe("light");
  await expect(page.locator("html")).toHaveAttribute("data-theme-preference", "system");
});

test("color scheme picker persists and keeps independent mode", async ({ page }) => {
  await page.emulateMedia({ colorScheme: "light" });

  await page.goto("/");
  await page.evaluate(() => {
    window.localStorage.setItem("ad-platform-theme", "dark");
    window.localStorage.setItem("ad-platform-scheme", "graphite");
    document.cookie = "ad-platform-theme=dark; path=/; max-age=31536000; samesite=lax";
    document.cookie = "ad-platform-scheme=graphite; path=/; max-age=31536000; samesite=lax";
  });
  await page.reload();

  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  await expect(page.locator("html")).toHaveAttribute("data-scheme", "graphite");

  await selectExpandedOption(page, ".scheme-picker", '[data-scheme-option="ink"]');

  await expect(page.locator("html")).toHaveAttribute("data-scheme", "ink");
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  await expect
    .poll(() => page.evaluate(() => window.localStorage.getItem("ad-platform-scheme")))
    .toBe("ink");

  await page.goto("/scoreboard");
  await expect(page.locator("html")).toHaveAttribute("data-scheme", "ink");
  await expect(page.locator('[data-scheme-option="ink"][aria-checked="true"]')).toBeVisible({
    timeout: 5000,
  });

  await page.reload();
  await expect(page.locator("html")).toHaveAttribute("data-scheme", "ink");
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");

  await selectExpandedOption(page, ".scheme-picker", '[data-scheme-option="paper"]');
  await expect(page.locator("html")).toHaveAttribute("data-scheme", "paper");
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
});

/**
 * Expand pickers hide non-active options with opacity/max-width, which confuses
 * Playwright hit-testing. Drive selection via DOM click for reliability.
 */
async function selectExpandedOption(
  page: import("@playwright/test").Page,
  pickerSelector: string,
  optionSelector: string,
) {
  await page.locator(pickerSelector).locator(optionSelector).evaluate((el) => {
    (el as HTMLButtonElement).click();
  });
}
