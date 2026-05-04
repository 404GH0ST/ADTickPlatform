import { expect, test } from "@playwright/test";
import { expectNoHorizontalOverflow, expectSchedulerFiltersVisible } from "./test-layout-utils";

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

  await expectNoHorizontalOverflow(page.locator("body"));

  const actionBar = page
    .getByTestId("service-card-svc-1")
    .getByRole("button", { name: "Unlock Service" })
    .locator("..");
  await expectNoHorizontalOverflow(actionBar);
});

test("organizer game scheduler filters stay inside the page width on mobile", async ({
  page,
}) => {
  await page.goto("/admin/game");

  await expect(page.locator("h1", { hasText: "Game" })).toBeVisible();
  const schedulerFilters = page.getByTestId("scheduler-audit-filters");
  await expectSchedulerFiltersVisible(schedulerFilters);

  await expectNoHorizontalOverflow(page.locator("body"));
  await expectNoHorizontalOverflow(schedulerFilters);
});
