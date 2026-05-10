import { expect } from "@playwright/test";
import { adminTest as test } from "./test-utils";

async function expectWireGuardNote(page: any, pattern: RegExp | string) {
  await expect(page.getByText(pattern)).toBeVisible();
}

test("organizer can inspect, rotate, revoke, and reconcile a player's WireGuard access", async ({
  page,
}) => {
  await page.goto("/admin/players");

  const playerRow = page.getByTestId("player-row-1001");
  await expect(playerRow).toBeVisible();
  await expect(playerRow).toContainText("wg-alpha");
  await expect(playerRow).toContainText("10.70.11.20/32");

  await page.getByTestId("inspect-wireguard-1001").click();
  const wireGuardDialog = page.getByRole("dialog", { name: "WireGuard Config" });
  await expect(wireGuardDialog).toBeVisible();
  await expect(
    page.getByText("Loaded WireGuard config for Alpha Captain."),
  ).toBeVisible();
  await expect(wireGuardDialog).toContainText("10.70.11.20/32");
  await expect(wireGuardDialog).toContainText("vpn.college.local:51820");
  await wireGuardDialog.getByRole("button", { name: "Close" }).first().click();

  await page.getByTestId("rotate-wireguard-1001").click();
  await expectWireGuardNote(
    page,
    /Rotated WireGuard config for Alpha Captain\./,
  );
  await expect(playerRow).toContainText("wg-alpha-rotated");
  await expect(playerRow).toContainText("10.70.15.21/32");
  await expect(wireGuardDialog).toContainText("10.70.15.21/32");
  await wireGuardDialog.getByRole("button", { name: "Close" }).first().click();

  await page.getByTestId("revoke-wireguard-1001").click();
  await expectWireGuardNote(
    page,
    /Revoked WireGuard config for Alpha Captain\./,
  );
  await expect(playerRow).toContainText("revoked");

  await page.getByRole("button", { name: "Reconcile Gateway" }).click();
  await expectWireGuardNote(
    page,
    /Maintenance reconcile applied WireGuard revision mock-wireguard-revision-\d+-\d+ with \d+ active peer\(s\) and \d+ revoked peer\(s\)\./,
  );
});

test("organizer can teardown and recover gateway and access runtime controls", async ({
  page,
}) => {
  page.on("dialog", (dialog) => dialog.accept());

  await page.goto("/admin/players");

  await page.getByTestId("teardown-wireguard-gateway").click();
  await expectWireGuardNote(page, /WireGuard gateway rules and interface torn down\./);
  await expect(page.getByText("mock-wireguard-teardown")).toBeVisible();

  await page.getByTestId("reconcile-wireguard-gateway").click();
  await expect(
    page.getByText(/Maintenance reconcile applied WireGuard revision mock-wireguard-revision-/),
  ).toBeVisible();

  await page.getByTestId("teardown-access").click();
  await expectWireGuardNote(page, /Controller service access rules torn down\./);
  await expect(page.getByText("mock-access-teardown")).toBeVisible();

  await page.getByTestId("reconcile-access").click();
  await expect(
    page.getByText(/Maintenance reconcile applied controller access revision mock-access-revision-/),
  ).toBeVisible();
});
