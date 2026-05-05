import { expect } from "@playwright/test";
import { expectNoHorizontalOverflow, expectSchedulerFiltersVisible } from "./test-layout-utils";
import { adminTest as test, mockApiBaseUrl } from "./test-utils";


test("organizer game page keeps scheduler audit filters inside the card width", async ({
  page,
}) => {
  await page.goto("/admin/game");

  await expect(page.locator("h1", { hasText: "Game" })).toBeVisible();
  await expect(page.getByText("Scheduler Audit Trail")).toBeVisible();
  const schedulerFilters = page.getByTestId("scheduler-audit-filters");
  await expectSchedulerFiltersVisible(schedulerFilters);

  await expectNoHorizontalOverflow(schedulerFilters);
});

test("organizer game page renders mocked scheduler audit data", async ({
  page,
}) => {
  await page.goto("/admin/game");

  await expect(page.getByText("scheduler started")).toBeVisible();
  await expect(page.getByText("tick #12 completed")).toBeVisible();
  await expect(
    page.getByText("scheduler stopped after match end"),
  ).toBeVisible();
  await expect(page.getByText("stopped").first()).toBeVisible();
});

test("organizer game page shows explicit empty states for scheduler and checker history", async ({
  page,
  request,
}) => {
  await request.post(`${mockApiBaseUrl}/__reset`, {
    data: { scenario: "empty-game-history" },
  });
  await page.goto("/admin/game");

  await expect(page.getByText("Scheduler Audit Trail")).toBeVisible();
  await expect(
    page.getByText("No scheduler events recorded yet."),
  ).toBeVisible();
  await expect(
    page.getByText("No checker runs persisted yet."),
  ).toBeVisible();
});

test("organizer game page shows degraded warning when scheduler and checker history feeds fail", async ({
  page,
  request,
}) => {
  await request.post(`${mockApiBaseUrl}/__reset`, {
    data: { scenario: "degraded-game-history" },
  });
  await page.goto("/admin/game");

  await expect(
    page.getByText(
      "Organizer data is partially unavailable. Only live responses that succeeded are shown. No sample data is injected.",
    ),
  ).toBeVisible();
  await expect(
    page.getByText("No scheduler events recorded yet."),
  ).toBeVisible();
  await expect(
    page.getByText("No checker runs persisted yet."),
  ).toBeVisible();
});

test("organizer game page surfaces runtime alerts with direct action links", async ({
  page,
  request,
}) => {
  await request.post(`${mockApiBaseUrl}/__reset`, {
    data: { scenario: "ops-alerts" },
  });
  await page.goto("/admin/game");

  const alertsCard = page.getByTestId("operations-alerts-card");
  await expect(alertsCard).toBeVisible();
  await expect(
    alertsCard.getByText("Scheduler next run is overdue."),
  ).toBeVisible();
  await expect(
    alertsCard.getByText("2 deployment job(s) are still active."),
  ).toBeVisible();
  await expect(
    alertsCard.getByRole("link", { name: "Scheduler Controls" }),
  ).toBeVisible();
  await expect(
    alertsCard.getByRole("link", { name: "Deployment Jobs" }),
  ).toBeVisible();
});

test("organizer game page refreshes runtime alerts on focus", async ({
  page,
  request,
}) => {
  await request.post(`${mockApiBaseUrl}/__reset`, {
    data: { scenario: "default" },
  });
  await page.goto("/admin/game");

  const alertsCard = page.getByTestId("operations-alerts-card");
  await expect(alertsCard).toContainText("No runtime alerts are currently active.");

  await request.post(`${mockApiBaseUrl}/__reset`, {
    data: { scenario: "ops-alerts" },
  });

  await page.evaluate(() => {
    window.dispatchEvent(new Event("focus"));
  });

  await expect(
    alertsCard.getByText("Scheduler next run is overdue."),
  ).toBeVisible();
  await expect(
    alertsCard.getByText("2 deployment job(s) are still active."),
  ).toBeVisible();
});

test("organizer game page shows live service metrics and refreshes derived health", async ({
  page,
  request,
}) => {
  await request.post(`${mockApiBaseUrl}/__reset`, {
    data: { scenario: "default" },
  });
  await page.goto("/admin/game");

  const metricsCard = page.getByTestId("operations-metrics-card");
  await expect(metricsCard).toBeVisible();
  await expect(
    metricsCard.getByText("All tracked service metrics currently look healthy."),
  ).toBeVisible();
  await expect(metricsCard.getByText("Game Core: healthy")).toBeVisible();
  await expect(metricsCard.getByText("Submission: healthy")).toBeVisible();
  await expect(metricsCard.getByText("WireGuard: healthy")).toBeVisible();
  await expect(metricsCard.getByText("Checker runs")).toBeVisible();
  await expect(metricsCard.getByText("72")).toBeVisible();

  await request.post(`${mockApiBaseUrl}/__reset`, {
    data: { scenario: "metrics-attention" },
  });

  await page.evaluate(() => {
    window.dispatchEvent(new Event("focus"));
  });

  await expect(
    metricsCard.getByText("service area(s) need operator attention."),
  ).toBeVisible();
  await expect(metricsCard.getByText("Game Core: attention")).toBeVisible();
  await expect(metricsCard.getByText("Submission: attention")).toBeVisible();
  await expect(metricsCard.getByText("Realtime: attention")).toBeVisible();
  await expect(metricsCard.getByText("WireGuard: attention")).toBeVisible();
  await expect(metricsCard.getByText("scheduler stopped")).toBeVisible();
  await expect(metricsCard.getByText("2 checker failures")).toBeVisible();
  await expect(metricsCard.getByText("3 submit failures")).toBeVisible();
  await expect(
    metricsCard.getByText("last sync failed, 4 sync errors"),
  ).toBeVisible();
});

test("organizer scheduler controls can start, update, and stop with visible notes", async ({
  page,
}) => {
  await page.goto("/admin/game");

  const schedulerCard = page.getByTestId("scheduler-card");

  await schedulerCard.getByRole("button", { name: "Resume Scheduler" }).click();
  await expect(
    page.getByText("Scheduler started with 60s interval."),
  ).toBeVisible();
  await expect(
    schedulerCard.getByRole("button", { name: "Stop Scheduler" }),
  ).toBeEnabled();

  const intervalInput = schedulerCard.locator('input[type="number"]').first();
  await intervalInput.fill("90");
  await schedulerCard.getByRole("button", { name: "Update" }).click();
  await expect(
    page.getByText("Scheduler interval updated to 90 seconds."),
  ).toBeVisible();
  await expect(intervalInput).toHaveValue("90");

  await schedulerCard.getByRole("button", { name: "Stop Scheduler" }).click();
  await expect(
    page.getByText("Scheduler stopped. Manual tick advance remains available."),
  ).toBeVisible();
  await expect(
    schedulerCard.getByRole("button", { name: "Resume Scheduler" }),
  ).toBeEnabled();
});

async function setupGameTest(page: any, request: any, scenario: string) {
  await request.post(`${mockApiBaseUrl}/__reset`, {
    data: { scenario },
  });
  await page.goto("/admin/game");
}

test("organizer match controls start the game and auto-start the scheduler", async ({
  page,
  request,
}) => {
  await setupGameTest(page, request, "prestart");

  const matchCard = page.getByTestId("match-card");
  const schedulerCard = page.getByTestId("scheduler-card");

  await matchCard.getByRole("button", { name: "Start Game" }).click();

  await expect(
    page.getByText("Game started. Submissions are open and the scheduler is running at 60s."),
  ).toBeVisible();
  await expect(matchCard.getByText("running").first()).toBeVisible();
  await expect(matchCard.getByText("open").first()).toBeVisible();
  await expect(
    schedulerCard.getByRole("button", { name: "Stop Scheduler" }),
  ).toBeEnabled();
});

test("organizer match controls schedule a game window", async ({
  page,
  request,
}) => {
  await setupGameTest(page, request, "prestart");

  await expect(page.getByText("Schedule Window")).toBeVisible();
  const startDate = page.getByLabel("Scheduled Start Date");
  const startTime = page.getByLabel("Scheduled Start Time");
  const endDate = page.getByLabel("Scheduled End Date");
  const endTime = page.getByLabel("Scheduled End Time");

  await startDate.fill("2026-03-21");
  await startTime.fill("09:30");
  await endDate.fill("2026-03-21");
  await endTime.fill("11:00");
  await page.getByRole("button", { name: "Save Window" }).click();

  await expect(page.getByText("Match window updated. Start:")).toBeVisible();
  await expect(startDate).toHaveValue("2026-03-21");
  await expect(startTime).toHaveValue("09:30");
  await expect(endDate).toHaveValue("2026-03-21");
  await expect(endTime).toHaveValue("11:00");

  await page.getByRole("button", { name: "Clear Window" }).click();
  await expect(
    page.getByText("Match window updated. Start: manual; end: manual."),
  ).toBeVisible();
  await expect(startDate).toHaveValue("");
  await expect(startTime).toHaveValue("");
  await expect(endDate).toHaveValue("");
  await expect(endTime).toHaveValue("");
});

test("organizer deployments page can delete a completed deployment job", async ({
  page,
}) => {
  await page.goto("/admin/deployments");

  const deploymentRow = page.getByTestId("deployment-row-77");
  await expect(deploymentRow).toBeVisible();

  await page.getByTestId("delete-deployment-77").click();

  const deleteDialog = page.getByRole("dialog", {
    name: "Delete deployment",
  });
  await expect(deleteDialog).toBeVisible();
  await deleteDialog.getByRole("button", { name: "Delete" }).click();

  await expect(page.getByText("Deployment job #77 deleted.")).toBeVisible();
  await expect(page.getByTestId("deployment-row-77")).toHaveCount(0);
});

test("organizer deployments reconcile completes queued jobs", async ({
  page,
  request,
}) => {
  await request.post(`${mockApiBaseUrl}/__reset`, {
    data: { scenario: "pending-deployments" },
  });

  await page.goto("/admin/deployments");

  const deploymentRow = page.getByTestId("deployment-row-88");
  const queuedCountCell = deploymentRow.getByRole("cell").nth(4);
  await expect(deploymentRow).toBeVisible();
  await expect(deploymentRow.getByText("queued")).toBeVisible();
  await expect(deploymentRow.getByText("1/3")).toBeVisible();
  await expect(queuedCountCell).toHaveText("2");

  await page.getByRole("button", { name: "Reconcile Deployments" }).click();

  await expect(
    page.getByText(
      "Trusted reconcile processed 1 job(s), advanced 2 team service instance(s), and refreshed deployment, SSH access, and WireGuard truth.",
    ),
  ).toBeVisible();
  await expect(deploymentRow.getByText("completed")).toBeVisible();
  await expect(deploymentRow.getByText("3/3")).toBeVisible();
  await expect(queuedCountCell).toHaveText("0");
});

test("organizer deployments reconcile surfaces trusted-truth failure details", async ({
  page,
  request,
}) => {
  await request.post(`${mockApiBaseUrl}/__reset`, {
    data: { scenario: "deployment-reconcile-access-failure" },
  });

  await page.goto("/admin/deployments");
  await page.getByRole("button", { name: "Reconcile Deployments" }).click();

  await expect(
    page.getByText(
      "runtime converge completed but controller access reconcile failed, so host access truth was not established.",
    ),
  ).toBeVisible();
});

test("organizer match controls handle manual match operations and recompute logic", async ({
  page,
  request,
}) => {
  await setupGameTest(page, request, "prestart");

  const matchCard = page.getByTestId("match-card");
  const quickActionsCard = page.getByTestId("quick-actions-card");

  await page.getByTestId("start-match").click();
  await expect(
    page.getByText("Game started. Submissions are open and the scheduler is running at 60s."),
  ).toBeVisible();

  await page.getByTestId("stop-match").click();
  await expect(
    page.getByText("Match stopped. Participant submissions are now closed."),
  ).toBeVisible();
  await expect(matchCard.getByText("finished").first()).toBeVisible();
  await expect(matchCard.getByText("closed").first()).toBeVisible();

  await page.getByTestId("advance-tick").click();
  await expect(
    page.getByText(
      "Tick 13 completed with 6 success, 0 failed, and 0 skipped checker runs.",
    ),
  ).toBeVisible();
  await expect(quickActionsCard.getByText("#13 completed")).toBeVisible();

  await page.getByTestId("recompute-scores").click();
  await expect(
    page.getByText(
      "Recomputed 3 scoreboard row(s) from authoritative tick and submission state.",
    ),
  ).toBeVisible();
});
