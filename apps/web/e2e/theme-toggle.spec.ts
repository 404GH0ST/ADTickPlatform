import { expect, test } from "@playwright/test";

test.use({ viewport: { width: 1280, height: 900 } });

test("theme toggle persists dark mode across reload and organizer navigation", async ({
  page,
}) => {
  await page.emulateMedia({ colorScheme: "light" });

  await page.goto("/");
  await page.evaluate(() => {
    window.localStorage.removeItem("ad-platform-theme");
  });
  await page.reload();

  await expect(page.locator("html")).toHaveAttribute("data-theme", "light");

  await page.getByRole("button", { name: "Switch to dark theme" }).click();

  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  await expect(page.getByRole("button", { name: "Switch to light theme" })).toBeVisible();
  await expect
    .poll(() => page.evaluate(() => window.localStorage.getItem("ad-platform-theme")))
    .toBe("dark");

  await page.reload();

  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  await expect(page.getByRole("button", { name: "Switch to light theme" })).toBeVisible();

  await page.goto("/admin");

  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  await expect(page.getByRole("button", { name: "Switch to light theme" })).toBeVisible();
});
