import { expect, test } from "@playwright/test";

import { mockApiBaseUrl, resetMockApi } from "./test-utils";

test.use({ viewport: { width: 1280, height: 900 } });
// The organizer theme lives in the mock API; start every test on the default.
test.beforeEach(async ({ request }) => {
  await resetMockApi(request);
});

test("theme toggle persists dark mode across reload and organizer navigation", async ({
  page,
}) => {
  await page.emulateMedia({ colorScheme: "light" });

  await page.goto("/");
  await page.evaluate(() => {
    window.localStorage.removeItem("ad-platform-theme");
  });
  await page.reload();

  // Default: Graphite theme (organizer default) + system mode (light OS → light).
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

test("an older page refresh cannot overwrite Dark during navigation", async ({ page, request, baseURL }) => {
  async function useNeubrutalism() {
    const response = await request.put(`${mockApiBaseUrl}/api/v2/admin/platform/appearance`, {
      data: { theme_id: "neubrutalism" },
    });
    expect(response.ok()).toBeTruthy();
  }

  await useNeubrutalism();
  await page.context().addCookies([{ name: "ad-platform-theme", value: "dark", url: baseURL! }]);
  await page.emulateMedia({ colorScheme: "light" });
  await page.goto("/admin/submissions");
  const html = page.locator("html");
  await expect(html).toHaveAttribute("data-theme", "dark");
  const refreshedRow = page.getByTestId("submission-30");
  await expect(refreshedRow).toHaveCount(0);

  await selectExpandedOption(page, ".mode-picker", '[data-mode-option="light"]');
  await expect(html).toHaveAttribute("data-theme", "light");
  await resetMockApi(request, "submissions");
  await useNeubrutalism();

  let markResponseReady!: () => void;
  let releaseRefresh!: () => void;
  const responseReady = new Promise<void>((resolve) => { markResponseReady = resolve; });
  const refreshCanFinish = new Promise<void>((resolve) => { releaseRefresh = resolve; });
  await page.route((url) => url.pathname === "/admin/submissions" && url.searchParams.has("_rsc"), async (route) => {
    const response = await route.fetch();
    markResponseReady();
    await refreshCanFinish;
    await route.fulfill({ response });
  }, { times: 1 });

  try {
    await page.getByRole("button", { name: "Refresh", exact: true }).click();
    await responseReady;
    await selectExpandedOption(page, ".mode-picker", '[data-mode-option="dark"]');
    await expect(html).toHaveAttribute("data-theme", "dark");
  } finally {
    releaseRefresh();
  }

  await expect(refreshedRow).toBeVisible();
  await expect(html).toHaveAttribute("data-theme", "dark");
  const primary = page.getByRole("navigation", { name: "Primary" });
  await primary.getByRole("link", { name: "Teams", exact: true }).click();
  await expect(page).toHaveURL(/\/admin\/teams$/);
  await expect(html).toHaveAttribute("data-theme", "dark");
  await expect(page.locator('[data-mode-option="dark"]')).toHaveAttribute("aria-checked", "true");
  await page.goBack();
  await expect(refreshedRow).toBeVisible();
  await expect(html).toHaveAttribute("data-theme-preference", "dark");
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

test("participants get a mode-only appearance control", async ({ page }) => {
  await page.goto("/");
  await page.locator(".mode-picker summary").click();
  await expect(page.locator("[data-mode-option]")).toHaveCount(3);
  // Theme (colors/layout) is the organizer's choice, not a participant control.
  await expect(page.locator("[data-scheme-option]")).toHaveCount(0);
  await expect(page.locator("html")).toHaveAttribute("data-scheme", "graphite");
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
