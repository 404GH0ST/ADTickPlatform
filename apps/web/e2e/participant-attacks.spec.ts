import { expect, test } from "@playwright/test";
import { assertEmptyAttacksState } from "./test-layout-utils";
import {
  mockApiBaseUrl,
  participantStorageState,
  resetMockApi,
} from "./test-utils";

test.use({
  viewport: { width: 1280, height: 900 },
  storageState: participantStorageState,
});

test.beforeEach(async ({ request }) => {
  await resetMockApi(request);
});

test("participant attacks shows one paginated attack surface and finished-match status", async ({
  page,
}) => {
  await page.goto("/attacks");

  await expect(page.locator("h1", { hasText: "Attacks" })).toBeVisible();
  await expect(page.getByText("Attack flow")).toBeVisible();
  await expect(page.getByText("Current slice")).toBeVisible();
  await expect(
    page.getByText(
      "The match has finished at tick #12. Participant submissions are closed.",
    ),
  ).toBeVisible();
  await expect(page.getByRole("link", { name: "Table View" })).toHaveCount(0);
  await expect(page.getByRole("link", { name: "Map View" })).toHaveCount(0);
});

test("participant pages warn when the match is stopped but the scheduler still reports running", async ({
  page,
  request,
}) => {
  await request.post(`${mockApiBaseUrl}/__reset`, {
    data: { scenario: "stopped-running-scheduler" },
  });

  await page.goto("/services");

  await expect(page.locator("h1", { hasText: "Services" })).toBeVisible();
  await expect(
    page.getByText(
      "The match is currently stopped. Participant submissions are closed.",
    ),
  ).toBeVisible();
});

test("participant services page shows an explicit empty state when the team owns no services", async ({
  page,
  request,
}) => {
  await request.post(`${mockApiBaseUrl}/__reset`, {
    data: { scenario: "empty-services" },
  });

  await page.goto("/services");

  await expect(page.locator("h1", { hasText: "Services" })).toBeVisible();
  await expect(
    page.getByText(/No services are assigned to this team yet/),
  ).toBeVisible();
  await expect(page.getByTestId(/service-card-/)).toHaveCount(0);
});

test("participant services page falls back to endpoint-only rows when service-state data is degraded", async ({
  page,
  request,
}) => {
  await request.post(`${mockApiBaseUrl}/__reset`, {
    data: { scenario: "degraded-service-states" },
  });

  await page.goto("/services");

  await expect(
    page.getByText(
      "Participant data is partially unavailable. Only live responses that succeeded are shown. No sample data is injected.",
    ),
  ).toBeVisible();

  const serviceCard = page.getByTestId("service-card-svc-1-1");
  await expect(serviceCard).toBeVisible();
  await expect(serviceCard.getByText("warning")).toBeVisible();
  await expect(
    serviceCard.getByText("service state endpoint unavailable"),
  ).toHaveCount(3);
});

test("participant attack map maximize dialog expands beyond the inline panel and stays scrollable", async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/attacks");

  const inlinePanel = page.getByTestId("attack-map-panel");
  const inlineBox = await inlinePanel.boundingBox();
  if (!inlineBox) {
    throw new Error("inline attack map panel did not render");
  }

  const maximize = page.getByRole("button", { name: "Maximize" });
  await maximize.press("Enter");

  const maximizeDialog = page.getByTestId("attack-map-maximize-dialog");
  await expect(maximizeDialog).toBeVisible();
  await expect(page.getByRole("button", { name: "Back to page" })).toBeVisible();

  const dialogBox = await maximizeDialog.boundingBox();
  if (!dialogBox) {
    throw new Error("maximize dialog did not render");
  }

  expect(dialogBox.width).toBeGreaterThan(inlineBox.width);
  const mapRegion = maximizeDialog.getByRole("region", { name: "Maximized attack map" });
  await mapRegion.press("End");
  await expect.poll(() => mapRegion.evaluate((element) => element.scrollTop)).toBeGreaterThan(0);
  await expect(maximizeDialog.getByRole("button", { name: "Back to page" })).toBeInViewport();
  await page.keyboard.press("Escape");
  await expect(maximizeDialog).toBeHidden();
  await expect(maximize).toBeFocused();
});

test("retired attack page URLs return 404", async ({ request }) => {
  for (const path of ["/attacks/map", "/attacks/table"]) {
    const response = await request.get(path, { maxRedirects: 0 });
    expect(response.status(), path).toBe(404);
  }
});

test("the participant attacks page shows empty-state messaging when no accepted attacks exist", async ({
  page,
  request,
}) => {
  await request.post(`${mockApiBaseUrl}/__reset`, {
    data: { scenario: "empty-attacks" },
  });

  await page.goto("/attacks");
  await expect(page.getByText("Showing 0-0 of 0")).toBeVisible();
  await assertEmptyAttacksState(page);
});

test("the participant attacks page says the feed is unavailable when it fails", async ({
  page,
  request,
}) => {
  await request.post(`${mockApiBaseUrl}/__reset`, {
    data: { scenario: "degraded-participant-attacks" },
  });

  await page.goto("/attacks");
  await expect(
    page.getByText(
      "Participant data is partially unavailable. Only live responses that succeeded are shown. No sample data is injected.",
    ),
  ).toBeVisible();
  await expect(page.getByTestId("data-unavailable")).toContainText("Attacks can't be loaded");
  await expect(page.getByTestId("attack-map-panel")).toHaveCount(0);
  await expect(page.getByLabel("Attacker")).toHaveCount(0);
  await expect(page.getByText("Showing 0-0 of 0")).toHaveCount(0);
  await expect(page.getByText("No accepted attacks in this view")).toHaveCount(0);
});

test("accepted enemy flag opens SSH before restart and factory reset", async ({
  page, request,
}) => {
  await request.post(`${mockApiBaseUrl}/api/v2/admin/game/match/start`);
  await page.goto("/services");

  await expect(page.locator("h1", { hasText: "Services" })).toBeVisible();

  const serviceCard = page.getByTestId("service-card-svc-1");
  await expect(
    serviceCard.getByText("unlock required before requesting root access"),
  ).toBeVisible();
  await expect(
    serviceCard.getByText("ok on tick #12"),
  ).toBeVisible();
  await expect(
    serviceCard.getByText(
      "service passed storage, retrieval, and functionality checks",
    ),
  ).toBeVisible();
  await expect(
    serviceCard.getByRole("link", { name: "Download Source" }),
  ).toHaveAttribute("href", "/api/platform/challenges/1/source");

  await expect(serviceCard.getByRole("button", { name: "SSH Locked" })).toBeDisabled();
  await expect(page.getByLabel("Unlock proof")).toHaveCount(0);
  await page.getByRole("textbox", { name: "Flags", exact: true }).fill("PLAYIT{enemy-challenge-1}");
  await page.getByRole("button", { name: "Submit", exact: true }).click();

  await expect(serviceCard.getByText("SSH unlocked by accepted enemy flag")).toBeVisible();
  await expect(serviceCard.getByText("unlocked", { exact: true })).toBeVisible();

  await serviceCard.getByRole("button", { name: "SSH Access" }).click();
  const sshDialog = page.getByRole("dialog", { name: "SSH Access" });
  await expect(
    sshDialog.getByText("ssh root@10.80.50.11 -p 22").first(),
  ).toBeVisible();
  await expect(
    sshDialog.getByText("Adp-team-credential-Aa1!").first(),
  ).toBeVisible();
  await expect(sshDialog.getByText("Patch Workflow")).toBeVisible();
  await expect(
    sshDialog.getByText(
      "Participant patching happens directly inside the owned service container.",
    ),
  ).toBeVisible();
  await expect(
    sshDialog.getByText("Use Restart after a live patch"),
  ).toBeVisible();
  await expect(
    sshDialog.getByText("Use Factory Reset to discard the current patch state"),
  ).toBeVisible();
  await sshDialog.getByRole("button", { name: "Close" }).first().click();

  const actionNotice = serviceCard.getByTestId(/^service-action-notice-/);
  const restartButton = serviceCard.getByRole("button", { name: /^Restart/ });

  await restartButton.click();
  await expect(
    serviceCard.getByText("service restart triggered via participant API"),
  ).toBeVisible();
  await expect(actionNotice).toContainText("Restart complete");
  await expect(actionNotice).toContainText("patches kept");
  await expect(serviceCard.getByText(/^restart cooldown: \d+s$/)).toBeVisible();

  await restartButton.click();
  await expect(actionNotice).toContainText(
    /Restart is on cooldown for this service\. Wait \d+ seconds? before trying again\./,
  );

  await serviceCard.getByRole("button", { name: /^Factory Reset/ }).click();
  const resetDialog = page.getByRole("dialog", {
    name: "Factory Reset Service",
  });
  await resetDialog.getByRole("button", { name: "Confirm Reset" }).click();

  await expect(
    serviceCard.getByText("factory reset triggered via participant API"),
  ).toBeVisible();
  await expect(actionNotice).toContainText("Factory reset complete");
  await expect(actionNotice).toContainText("baseline image");
  await expect(serviceCard.getByText(/factory reset cooldown: \d+s/)).toBeVisible();
  await expect(serviceCard.getByText("unlocked", { exact: true })).toBeVisible();

  await serviceCard.getByRole("button", { name: /^Factory Reset/ }).click();
  await expect(
    resetDialog.getByText(/Factory reset is on cooldown for this service\. Wait \d+ seconds/),
  ).toBeVisible();
  await resetDialog.getByRole("button", { name: "Confirm Reset" }).click();
  await expect(
    resetDialog.getByText(/Factory reset is on cooldown for this service\. Wait \d+ seconds/),
  ).toBeVisible();
});
