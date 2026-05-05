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

test("participant attacks defaults to the map view and shows finished-match status", async ({
  page,
}) => {
  await page.goto("/attacks");

  await expect(
    page.locator("h1", { hasText: "Attack Map" }),
  ).toBeVisible();
  await expect(page.getByText("Participant attack map")).toBeVisible();
  await expect(
    page.getByText(
      "The match has finished at tick #12. Participant submissions are closed.",
    ),
  ).toBeVisible();
  await expect(page.getByRole("link", { name: "Table View" })).toHaveAttribute(
    "href",
    "/attacks/table",
  );
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
      "The match is currently stopped. Participant submissions are closed. The scheduler is still reporting as running and should be checked by the organizer.",
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
    page.getByText("No owned services are available for this team yet."),
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
  await page.goto("/attacks");

  const inlinePanel = page.getByTestId("attack-map-panel");
  const inlineBox = await inlinePanel.boundingBox();
  if (!inlineBox) {
    throw new Error("inline attack map panel did not render");
  }

  await page.getByRole("button", { name: "Maximize" }).click();

  const maximizeDialog = page.getByTestId("attack-map-maximize-dialog");
  await expect(maximizeDialog).toBeVisible();
  await expect(page.getByRole("button", { name: "Back to page" })).toBeVisible();

  const dialogBox = await maximizeDialog.boundingBox();
  if (!dialogBox) {
    throw new Error("maximize dialog did not render");
  }

  expect(dialogBox.width).toBeGreaterThan(inlineBox.width);
  expect(
    await maximizeDialog.evaluate((element) =>
      getComputedStyle(element).overflowY,
    ),
  ).toBe("auto");
});

test("participant attack table route remains reachable from the map-first navigation", async ({
  page,
}) => {
  await page.goto("/attacks");
  await page.getByRole("link", { name: "Table View" }).click();

  await expect(page).toHaveURL(/\/attacks\/table$/);
  await expect(page.locator("h1", { hasText: "Attacks" })).toBeVisible();
  await expect(page.getByRole("link", { name: "Map View" })).toHaveAttribute(
    "href",
    "/attacks",
  );
  await expect(
    page.getByText("first valid submission accepted").first(),
  ).toBeVisible();
});

test("participant attacks routes show empty-state messaging when no accepted attacks exist", async ({
  page,
  request,
}) => {
  await request.post(`${mockApiBaseUrl}/__reset`, {
    data: { scenario: "empty-attacks" },
  });

  await page.goto("/attacks");
  await expect(page.getByText("Loaded 0 of 0 attack(s)")).toBeVisible();
  await assertEmptyAttacksState(page);

  await page.goto("/attacks/table");
  await assertEmptyAttacksState(page);
});

test("participant attacks routes show degraded warning and empty-state messaging when the attack feed fails", async ({
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
  await expect(page.getByText("Loaded 0 of 0 attack(s)")).toBeVisible();
  await assertEmptyAttacksState(page);

  await page.goto("/attacks/table");
  await expect(
    page.getByText(
      "Participant data is partially unavailable. Only live responses that succeeded are shown. No sample data is injected.",
    ),
  ).toBeVisible();
  await assertEmptyAttacksState(page);
});

test("participant service actions update the service card through unlock, ssh, restart, and factory reset", async ({
  page,
}) => {
  await page.goto("/services");

  await expect(page.locator("h1", { hasText: "Services" })).toBeVisible();

  const serviceCard = page.getByTestId("service-card-svc-1");
  await expect(
    serviceCard.getByText("unlock required before requesting root access"),
  ).toBeVisible();
  await expect(serviceCard.getByText("check passed on tick #12")).toBeVisible();
  await expect(serviceCard.getByText("latest SLA cycle passed")).toBeVisible();
  await expect(
    serviceCard.getByRole("link", { name: "Download Source" }),
  ).toHaveAttribute("href", "/api/platform/challenges/1/source");

  await serviceCard.getByRole("button", { name: "Unlock Service" }).click();
  const unlockDialog = page.getByRole("dialog", { name: "Unlock Service" });
  await unlockDialog.getByLabel("Unlock proof").fill("proof-from-own-service");
  await unlockDialog.getByRole("button", { name: "Unlock Service" }).click();

  await expect(
    serviceCard.getByText("unlock granted via participant API"),
  ).toBeVisible();
  await expect(serviceCard.getByText("unlocked")).toBeVisible();

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
    sshDialog.getByRole("link", { name: "Download Source Bundle" }),
  ).toHaveAttribute("href", "/api/platform/challenges/1/source");
  await expect(
    sshDialog.getByText("Use Restart after a live patch"),
  ).toBeVisible();
  await expect(
    sshDialog.getByText("Use Factory Reset to discard the current patch state"),
  ).toBeVisible();
  await sshDialog.getByRole("button", { name: "Close" }).first().click();

  await serviceCard.getByRole("button", { name: "Restart" }).click();
  await expect(
    serviceCard.getByText("service restart triggered via participant API"),
  ).toBeVisible();
  await expect(serviceCard.getByText("restart requested")).toBeVisible();

  await serviceCard.getByRole("button", { name: "Factory Reset" }).click();
  const resetDialog = page.getByRole("dialog", {
    name: "Factory Reset Service",
  });
  await resetDialog.getByRole("button", { name: "Confirm Reset" }).click();

  await expect(
    serviceCard.getByText("factory reset triggered via participant API"),
  ).toBeVisible();
  await expect(serviceCard.getByText("cooldown: 90s")).toBeVisible();
  await expect(serviceCard.getByText("unlocked")).toBeVisible();
});
