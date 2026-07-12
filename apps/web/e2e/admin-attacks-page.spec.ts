import { expect } from "@playwright/test";
import { adminTest as test, mockApiBaseUrl } from "./test-utils";

test("organizer attacks page renders the paginated accepted-attack feed", async ({
  page,
}) => {
  await page.goto("/admin/attacks");

  await expect(page.locator("h1", { hasText: "Attacks" })).toBeVisible();
  await expect(page.getByText("Accepted Attacks")).toBeVisible();
  await expect(page.getByText("Showing 1-13 of 13")).toBeVisible();
  await expect(
    page.getByRole("cell", { name: "College Alpha" }).first(),
  ).toBeVisible();
  await expect(
    page.getByRole("cell", { name: "College Beta" }).first(),
  ).toBeVisible();
  await expect(
    page.getByRole("cell", { name: "first valid submission accepted" }).first(),
  ).toBeVisible();
});

test("organizer attacks page filters the accepted-attack feed", async ({
  page,
}) => {
  await page.goto("/admin/attacks");

  await page.getByLabel("Attacker").selectOption("College Alpha");
  await page.getByRole("button", { name: "Apply view" }).click();

  const table = page.locator("table").filter({ hasText: "Attacker" });
  await expect(table.getByRole("cell", { name: "College Alpha" }).first()).toBeVisible();
  await expect(table.getByRole("row")).toHaveCount(4); // header + 3 College Alpha rows
});

test("organizer attacks page shows empty-state messaging when no accepted attacks exist", async ({
  page,
  request,
}) => {
  await request.post(`${mockApiBaseUrl}/__reset`, {
    data: { scenario: "empty-attacks" },
  });

  await page.goto("/admin/attacks");
  await expect(
    page.getByText("No accepted attacks in this view"),
  ).toBeVisible();
});

test("organizer attacks page shows degraded warning and empty-state messaging when the attack feed fails", async ({
  page,
  request,
}) => {
  await request.post(`${mockApiBaseUrl}/__reset`, {
    data: { scenario: "degraded-admin-attacks" },
  });

  await page.goto("/admin/attacks");

  await expect(
    page.getByText(
      "Organizer data is partially unavailable. Only live responses that succeeded are shown. No sample data is injected.",
    ),
  ).toBeVisible();
  await expect(
    page.getByText("No accepted attacks in this view"),
  ).toBeVisible();
});
