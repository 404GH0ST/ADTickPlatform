import { expect } from "@playwright/test";
import { adminTest as test, mockApiBaseUrl } from "./test-utils";

for (const mobile of [false, true]) {
  test(`organizer can rotate a team invitation on ${mobile ? "mobile" : "desktop"}`, async ({ page, request }) => {
    if (mobile) {
      await page.setViewportSize({ width: 390, height: 844 });
      const response = await request.put(`${mockApiBaseUrl}/api/v2/admin/platform/appearance`, {
        data: { theme_id: "neubrutalism" },
      });
      expect(response.ok()).toBeTruthy();
    }
    await page.goto("/admin/teams");

    const row = page.getByTestId("team-row-101");
    const originalKey = row.getByText("TEAM-ALPHA-JOIN", { exact: true });
    await expect(originalKey).toBeVisible();
    const openRotation = async () => {
      if (mobile) await row.getByRole("button", { name: "Actions for College Alpha" }).click();
      await row.getByRole("button", { name: "Rotate join key for College Alpha" }).click();
    };
    const dialog = page.getByRole("dialog", { name: "Rotate join key" });
    await openRotation();
    await expect(dialog).toBeVisible();
    await dialog.getByRole("button", { name: "Cancel" }).click();
    await expect(originalKey).toBeVisible();

    await openRotation();
    const rotation = page.waitForResponse((response) => response.url().endsWith("/teams/101/join-key/rotate"));
    await dialog.getByRole("button", { name: "Rotate key" }).click();
    const response = await rotation;
    expect(response.ok()).toBeTruthy();
    expect(response.headers()["cache-control"]).toBe("no-store");
    await expect(page.getByText('Join key rotated for "College Alpha".', { exact: false })).toBeVisible();
    await expect(originalKey).toHaveCount(0);
    await expect(row.getByText(/^TEAM-(?:[0-9A-F]{8}-){2}[0-9A-F]{8}$/)).toBeVisible();
  });
}

test("organizer can deactivate and reactivate a team", async ({ page }) => {
  await page.goto("/admin/teams");

  const statusBadge = page.getByTestId("team-status-101");
  const toggle = page.getByTestId("toggle-team-active-101");
  await expect(statusBadge).toHaveText("Active");
  await expect(toggle).toHaveAttribute("aria-label", "Deactivate team College Alpha");

  await toggle.click();
  await expect(
    page.getByText('Team "College Alpha" deactivated.'),
  ).toBeVisible();
  await expect(statusBadge).toHaveText("Inactive");
  // Wait for the row to re-render into its reactivate state before toggling
  // back, so the click carries the updated (now-inactive) team.
  await expect(toggle).toHaveAttribute("aria-label", "Reactivate team College Alpha");

  await toggle.click();
  await expect(
    page.getByText('Team "College Alpha" reactivated.'),
  ).toBeVisible();
  await expect(statusBadge).toHaveText("Active");
});

test("a deactivated team stays on the scoreboard last with DQ status", async ({ page, request }) => {
  const deactivated = await request.post(`${mockApiBaseUrl}/api/v2/admin/teams/101/deactivate`);
  expect(deactivated.ok()).toBeTruthy();

  await page.goto("/scoreboard");
  const table = page.getByRole("table");
  const rows = table.locator("tbody tr");
  const lastRow = rows.last();
  await expect(lastRow).toContainText("College Alpha");
  await expect(lastRow.getByTestId("scoreboard-dq-badge")).toBeVisible();
  await expect(table.getByTestId("scoreboard-dq-badge")).toHaveCount(1);

  // Sorting by total must not lift the disqualified leader off the bottom.
  await page.getByRole("button", { name: "Total" }).click();
  await page.getByRole("button", { name: "Total" }).click();
  await expect(rows.last()).toContainText("College Alpha");

  const reactivated = await request.post(`${mockApiBaseUrl}/api/v2/admin/teams/101/reactivate`);
  expect(reactivated.ok()).toBeTruthy();
  await page.reload();
  await expect(rows.first()).toContainText("College Alpha");
  await expect(table.getByTestId("scoreboard-dq-badge")).toHaveCount(0);
});

test("organizer can deactivate and reactivate a player", async ({ page }) => {
  await page.goto("/admin/players");

  const statusBadge = page.getByTestId("player-status-1001");
  const toggle = page.getByTestId("toggle-player-active-1001");
  await expect(statusBadge).toHaveText("Active");
  await expect(toggle).toHaveAttribute("aria-label", "Deactivate player Alpha Captain");

  await toggle.click();
  await expect(
    page.getByText('Player "Alpha Captain" deactivated.'),
  ).toBeVisible();
  await expect(statusBadge).toHaveText("Inactive");
  await expect(toggle).toHaveAttribute("aria-label", "Reactivate player Alpha Captain");

  await toggle.click();
  await expect(
    page.getByText('Player "Alpha Captain" reactivated.'),
  ).toBeVisible();
  await expect(statusBadge).toHaveText("Active");
});
