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

export async function openDisclosureIfNeeded(disclosure: Locator) {
  const expanded = await disclosure.evaluate((element) =>
    element.hasAttribute("open"),
  );
  if (!expanded) {
    await disclosure.locator("summary").first().click();
  }
}

export async function assertEmptyAttacksState(page: Page) {
  await expect(
    page.getByText(/No accepted attacks are available to plot in this view/),
  ).toBeVisible();
  await expect(
    page.getByText("No accepted attacks in this view"),
  ).toBeVisible();
}
