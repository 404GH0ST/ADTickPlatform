import { expect } from "@playwright/test";
import { adminTest as test } from "./test-utils";

test("organizer can deactivate and reactivate a team", async ({ page }) => {
  await page.goto("/admin/teams");

  const statusBadge = page.getByTestId("team-status-101");
  const toggle = page.getByTestId("toggle-team-active-101");
  await expect(statusBadge).toHaveText("Active");
  await expect(toggle).toHaveAttribute("aria-label", "Deactivate team");

  await toggle.click();
  await expect(
    page.getByText('Team "College Alpha" deactivated.'),
  ).toBeVisible();
  await expect(statusBadge).toHaveText("Inactive");
  // Wait for the row to re-render into its reactivate state before toggling
  // back, so the click carries the updated (now-inactive) team.
  await expect(toggle).toHaveAttribute("aria-label", "Reactivate team");

  await toggle.click();
  await expect(
    page.getByText('Team "College Alpha" reactivated.'),
  ).toBeVisible();
  await expect(statusBadge).toHaveText("Active");
});

test("organizer can deactivate and reactivate a player", async ({ page }) => {
  await page.goto("/admin/players");

  const statusBadge = page.getByTestId("player-status-1001");
  const toggle = page.getByTestId("toggle-player-active-1001");
  await expect(statusBadge).toHaveText("Active");
  await expect(toggle).toHaveAttribute("aria-label", "Deactivate player");

  await toggle.click();
  await expect(
    page.getByText('Player "Alpha Captain" deactivated.'),
  ).toBeVisible();
  await expect(statusBadge).toHaveText("Inactive");
  await expect(toggle).toHaveAttribute("aria-label", "Reactivate player");

  await toggle.click();
  await expect(
    page.getByText('Player "Alpha Captain" reactivated.'),
  ).toBeVisible();
  await expect(statusBadge).toHaveText("Active");
});
