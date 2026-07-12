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
  await expect(page.locator('[data-mode-option="light"]')).toHaveAttribute("aria-checked", "true");

  // Collapsed mode chip; expand on hover then pick Dark
  await selectExpandedOption(page, ".mode-picker", '[data-mode-option="dark"]');

  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  await expect(page.locator("html")).toHaveAttribute("data-theme-preference", "dark");
  await expect
    .poll(() => page.evaluate(() => window.localStorage.getItem("ad-platform-theme")))
    .toBe("dark");

  await page.reload();

  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  await expect(page.locator('[data-mode-option="dark"]')).toHaveAttribute("aria-checked", "true");

  await page.goto("/admin");

  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  await expect(page.locator("html")).toHaveAttribute("data-scheme", "graphite");
  await expect(page.locator('[data-mode-option="dark"]')).toHaveAttribute("aria-checked", "true");
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
  await expect(page.locator('[data-mode-option="system"]')).toHaveAttribute("aria-checked", "true");

  await page.emulateMedia({ colorScheme: "light" });
  await expect
    .poll(() => page.locator("html").getAttribute("data-theme"))
    .toBe("light");
  await expect(page.locator("html")).toHaveAttribute("data-theme-preference", "system");
});

test("first visit defaults to system without locking a resolved mode", async ({
  page,
}) => {
  await page.emulateMedia({ colorScheme: "dark" });
  await page.goto("/");
  await page.evaluate(() => {
    window.localStorage.removeItem("ad-platform-theme");
    document.cookie =
      "ad-platform-theme=; path=/; max-age=0; samesite=lax";
  });
  await page.reload();

  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  await expect(page.locator("html")).toHaveAttribute(
    "data-theme-preference",
    "system",
  );
  // FOUC must not write light/dark; absence or "system" both keep OS following.
  const stored = await page.evaluate(() =>
    window.localStorage.getItem("ad-platform-theme"),
  );
  expect(stored === null || stored === "system").toBe(true);

  await page.emulateMedia({ colorScheme: "light" });
  await expect
    .poll(() => page.locator("html").getAttribute("data-theme"))
    .toBe("light");
  await expect(page.locator("html")).toHaveAttribute(
    "data-theme-preference",
    "system",
  );
});

test("explicit system selection persists and keeps following OS", async ({
  page,
}) => {
  await page.emulateMedia({ colorScheme: "light" });
  await page.goto("/");
  await page.evaluate(() => {
    window.localStorage.setItem("ad-platform-theme", "dark");
    document.cookie =
      "ad-platform-theme=dark; path=/; max-age=31536000; samesite=lax";
  });
  await page.reload();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");

  await selectExpandedOption(page, ".mode-picker", '[data-mode-option="system"]');
  await expect(page.locator("html")).toHaveAttribute(
    "data-theme-preference",
    "system",
  );
  await expect(page.locator("html")).toHaveAttribute("data-theme", "light");
  await expect
    .poll(() => page.evaluate(() => window.localStorage.getItem("ad-platform-theme")))
    .toBe("system");

  await page.emulateMedia({ colorScheme: "dark" });
  await expect
    .poll(() => page.locator("html").getAttribute("data-theme"))
    .toBe("dark");
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
  await expect(page.locator('[data-scheme-option="ink"]')).toHaveAttribute("aria-checked", "true");

  await page.reload();
  await expect(page.locator("html")).toHaveAttribute("data-scheme", "ink");
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");

  await selectExpandedOption(page, ".scheme-picker", '[data-scheme-option="paper"]');
  await expect(page.locator("html")).toHaveAttribute("data-scheme", "paper");
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
});

/**
 * Appearance options live in a native disclosure so low-frequency preferences
 * do not occupy the primary header scan path.
 */
async function selectExpandedOption(
  page: import("@playwright/test").Page,
  pickerSelector: string,
  optionSelector: string,
) {
  const picker = page.locator(pickerSelector);
  if (!(await picker.evaluate((element) => (element as HTMLDetailsElement).open))) {
    await picker.locator("summary").click();
  }
  await picker.locator(optionSelector).click();
}
