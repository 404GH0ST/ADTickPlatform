import { expect, test } from "@playwright/test";

const mockApiBaseUrl = "http://127.0.0.1:4010";

test.use({ viewport: { width: 390, height: 844 } });

test.beforeEach(async ({ request }) => {
  await request.post(`${mockApiBaseUrl}/__reset`);
});

test("participant services page stays readable without horizontal overflow on mobile", async ({
  page,
}) => {
  await page.goto("/services");

  await expect(page.locator("h1", { hasText: "Services" })).toBeVisible();
  await expect(page.getByTestId("service-card-svc-1")).toBeVisible();

  const pageLayout = await page.locator("body").evaluate((element) => ({
    clientWidth: element.clientWidth,
    scrollWidth: element.scrollWidth,
  }));
  expect(pageLayout.scrollWidth).toBeLessThanOrEqual(pageLayout.clientWidth + 1);

  const actionBar = page
    .getByTestId("service-card-svc-1")
    .getByRole("button", { name: "Unlock Service" })
    .locator("..");
  const actionLayout = await actionBar.evaluate((element) => ({
    clientWidth: element.clientWidth,
    scrollWidth: element.scrollWidth,
  }));
  expect(actionLayout.scrollWidth).toBeLessThanOrEqual(
    actionLayout.clientWidth + 1,
  );
});

test("organizer game scheduler filters stay inside the page width on mobile", async ({
  page,
}) => {
  await page.goto("/admin/game");

  await expect(page.locator("h1", { hasText: "Game" })).toBeVisible();
  const schedulerFilters = page.getByTestId("scheduler-audit-filters");
  await expect(schedulerFilters.getByLabel("Page Size")).toBeVisible();
  await expect(schedulerFilters.getByLabel("Event Type")).toBeVisible();
  await expect(schedulerFilters.getByLabel("Source")).toBeVisible();
  await expect(schedulerFilters.getByLabel("State")).toBeVisible();

  const pageLayout = await page.locator("body").evaluate((element) => ({
    clientWidth: element.clientWidth,
    scrollWidth: element.scrollWidth,
  }));
  expect(pageLayout.scrollWidth).toBeLessThanOrEqual(pageLayout.clientWidth + 1);

  const filterLayout = await schedulerFilters.evaluate((element) => ({
    clientWidth: element.clientWidth,
    scrollWidth: element.scrollWidth,
  }));
  expect(filterLayout.scrollWidth).toBeLessThanOrEqual(
    filterLayout.clientWidth + 1,
  );
});
