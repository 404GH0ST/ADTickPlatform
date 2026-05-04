import { expect, Locator, Page } from "@playwright/test";

export async function expectNoHorizontalOverflow(locator: Locator) {
  const layout = await locator.evaluate((element) => ({
    clientWidth: element.clientWidth,
    scrollWidth: element.scrollWidth,
  }));
  expect(layout.scrollWidth).toBeLessThanOrEqual(layout.clientWidth + 1);
}

export async function expectSchedulerFiltersVisible(schedulerFilters: Locator) {
  await expect(schedulerFilters.getByLabel("Page Size")).toBeVisible();
  await expect(schedulerFilters.getByLabel("Event Type")).toBeVisible();
  await expect(schedulerFilters.getByLabel("Source")).toBeVisible();
  await expect(schedulerFilters.getByLabel("State")).toBeVisible();
}

export async function assertEmptyAttacksState(page: Page) {
  await expect(
    page.getByText("No accepted attacks in the current slice to plot."),
  ).toBeVisible();
  await expect(
    page.getByText("No accepted attack events are available for this slice."),
  ).toBeVisible();
}
