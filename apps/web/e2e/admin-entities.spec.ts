import { expect, test } from "@playwright/test";

const mockApiBaseUrl = "http://127.0.0.1:4010";

test.use({ viewport: { width: 1280, height: 900 } });

test.beforeEach(async ({ request }) => {
  await request.post(`${mockApiBaseUrl}/__reset`);
});

test("organizer can create a team, create a player for it, and delete that player", async ({
  page,
}) => {
  await page.goto("/admin/teams");

  await page.getByRole("button", { name: "Create Team" }).click();
  await page.getByLabel("Team name").fill("College Delta");
  await page.getByLabel("Contact email").fill("delta@college.local");
  await page.getByRole("button", { name: "Create" }).click();

  await expect(
    page.getByText(
      'Created College Delta and seeded deployed services for published challenges.',
    ),
  ).toBeVisible();
  await expect(page.getByTestId("team-row-104")).toBeVisible();
  await expect(page.getByTestId("team-row-104")).toContainText("College Delta");

  await page.goto("/admin/players");

  await page.getByRole("button", { name: "Create Player" }).click();
  await page.getByLabel("Team").selectOption({ label: "College Delta" });
  await page.getByLabel("Display name").fill("Delta Captain");
  await page.getByLabel("Email").fill("captain.delta@college.local");
  await page.getByLabel("Password").fill("delta-password");
  await page.getByLabel("Role").selectOption("captain");
  await page.getByRole("button", { name: "Create" }).click();

  await expect(
    page.getByText(
      'Created player Delta Captain with peer wg-1003. Reconcile the WireGuard gateway and service access policy if that team already has unlocked services.',
    ),
  ).toBeVisible();
  await expect(page.getByTestId("player-row-1003")).toBeVisible();
  await expect(page.getByTestId("player-row-1003")).toContainText(
    "College Delta",
  );
  await expect(page.getByTestId("player-row-1003")).toContainText("captain");

  await page.getByTestId("delete-player-1003").click();
  const deleteDialog = page.getByRole("dialog", { name: "Delete player" });
  await expect(deleteDialog).toBeVisible();
  await deleteDialog.getByRole("button", { name: "Delete" }).click();

  await expect(page.getByText('Player "Delta Captain" deleted.')).toBeVisible();
  await expect(page.getByTestId("player-row-1003")).toHaveCount(0);
});

test("organizer can edit a team and update a player profile", async ({
  page,
}) => {
  await page.goto("/admin/teams");
  await expect(page.getByTestId("team-row-101")).toBeVisible();

  await page.getByTestId("edit-team-101").click();
  const teamDialog = page.getByRole("dialog", { name: "Edit team" });
  await expect(teamDialog).toBeVisible();
  await teamDialog.getByLabel("Team name").fill("College Alpha Prime");
  await teamDialog
    .getByLabel("Contact email")
    .fill("alpha.prime@college.local");
  await teamDialog.getByRole("button", { name: "Update" }).click();

  await expect(page.getByText("Updated team College Alpha Prime.")).toBeVisible();
  await expect(page.getByTestId("team-row-101")).toContainText(
    "College Alpha Prime",
  );
  await expect(page.getByTestId("team-row-101")).toContainText(
    "alpha.prime@college.local",
  );

  await page.goto("/admin/players");
  await expect(page.getByTestId("player-row-1001")).toBeVisible();

  await page.getByTestId("edit-player-1001").click();
  const playerDialog = page.getByRole("dialog", { name: "Edit player" });
  await expect(playerDialog).toBeVisible();
  await playerDialog.getByLabel("Display name").fill("Alpha Lead");
  await playerDialog.getByLabel("Email").fill("lead.alpha@college.local");
  await playerDialog.getByLabel("Role").selectOption("captain");
  await playerDialog.getByRole("button", { name: "Update" }).click();

  await expect(page.getByText("Updated player Alpha Lead.")).toBeVisible();
  await expect(page.getByTestId("player-row-1001")).toContainText(
    "Alpha Lead",
  );
  await expect(page.getByTestId("player-row-1001")).toContainText(
    "lead.alpha@college.local",
  );
  await expect(page.getByTestId("player-row-1001")).toContainText("captain");
  await expect(page.getByTestId("player-row-1001")).toContainText(
    "College Alpha Prime",
  );
});
