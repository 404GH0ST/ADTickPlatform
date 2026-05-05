import { expect } from "@playwright/test";
import { adminTest as test } from "./test-utils";


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
  await expect(
    page.getByText(
      "Rotated WireGuard config for Alpha Captain. Reconcile the WireGuard gateway so the old peer material stops working.",
    ),
  ).toBeVisible();
  await expect(playerRow).toContainText("wg-alpha-rotated");
  await expect(playerRow).toContainText("10.70.15.21/32");
  await expect(wireGuardDialog).toContainText("10.70.15.21/32");
  await wireGuardDialog.getByRole("button", { name: "Close" }).first().click();

  await page.getByTestId("revoke-wireguard-1001").click();
  await expect(
    page.getByText(
      "Revoked WireGuard config for Alpha Captain. Reconcile the WireGuard gateway and service access policy to remove the peer from runtime access.",
    ),
  ).toBeVisible();
  await expect(playerRow).toContainText("revoked");

  await page.getByRole("button", { name: "Reconcile Gateway" }).click();
  await expect(
    page.getByText(
      "Maintenance reconcile applied WireGuard revision mock-wireguard-revision-1-1 with 1 active peer(s) and 1 revoked peer(s).",
    ),
  ).toBeVisible();
});

test("organizer can teardown and recover gateway and access runtime controls", async ({
  page,
}) => {
  page.on("dialog", (dialog) => dialog.accept());

  await page.goto("/admin/players");

  await page.getByTestId("teardown-wireguard-gateway").click();
  await expect(
    page.getByText("WireGuard gateway rules and interface torn down."),
  ).toBeVisible();
  await expect(page.getByText("mock-wireguard-teardown")).toBeVisible();

  await page.getByTestId("reconcile-wireguard-gateway").click();
  await expect(
    page.getByText(/Maintenance reconcile applied WireGuard revision mock-wireguard-revision-/),
  ).toBeVisible();

  await page.getByTestId("teardown-access").click();
  await expect(
    page.getByText("Controller service access rules torn down."),
  ).toBeVisible();
  await expect(page.getByText("mock-access-teardown")).toBeVisible();

  await page.getByTestId("reconcile-access").click();
  await expect(
    page.getByText(/Maintenance reconcile applied controller access revision mock-access-revision-/),
  ).toBeVisible();
});
