import { expect, test } from "@playwright/test";

const mockApiBaseUrl = "http://127.0.0.1:4010";

test.use({ viewport: { width: 1280, height: 900 } });

test.beforeEach(async ({ request }) => {
  await request.post(`${mockApiBaseUrl}/__reset`);
});

test("organizer audit page applies filters and paginates server-side", async ({
  page,
}) => {
  await page.goto("/admin/audit?actor_type=team&action=service.unlock&limit=1");

  await expect(page.locator("h1", { hasText: "Audit" })).toBeVisible();
  await expect(page.getByLabel("Actor Type")).toHaveValue("team");
  await expect(page.getByLabel("Action")).toHaveValue("service.unlock");
  await expect(
    page.getByText("Showing 1-1 of 2 audit entries."),
  ).toBeVisible();
  await expect(page.getByText("College Alpha")).toBeVisible();
  await expect(page.getByRole("link", { name: "Next" })).toBeVisible();

  await page.getByRole("link", { name: "Next" }).click();

  await expect(page).toHaveURL(/offset=1/);
  await expect(
    page.getByText("Showing 2-2 of 2 audit entries."),
  ).toBeVisible();
  await expect(page.getByText("College Beta")).toBeVisible();
  await expect(page.getByRole("link", { name: "Previous" })).toBeVisible();
});
