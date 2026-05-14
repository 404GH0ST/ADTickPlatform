import { readFile } from "node:fs/promises";

import { expect } from "@playwright/test";
import {
  expectNoHorizontalOverflow,
  expectSchedulerFiltersVisible,
  openDisclosureIfNeeded,
} from "./test-layout-utils";
import { adminTest as test, mockApiBaseUrl } from "./test-utils";

async function expectActionNote(page: any, pattern: RegExp | string) {
  await expect(page.getByText(pattern)).toBeVisible();
}

test("organizer game page keeps scheduler audit filters inside the card width", async ({
  page,
}) => {
  await page.goto("/admin/game");

  await expect(page.locator("h1", { hasText: "Game" })).toBeVisible();
  await expect(
    page.getByTestId("scheduler-audit-disclosure").locator("summary"),
  ).toBeVisible();
  await openDisclosureIfNeeded(page.getByTestId("scheduler-audit-disclosure"));
  const schedulerFilters = page.getByTestId("scheduler-audit-filters");
  await expectSchedulerFiltersVisible(schedulerFilters);

  await expectNoHorizontalOverflow(schedulerFilters);
});

test("organizer game page renders mocked scheduler audit data", async ({
  page,
}) => {
  await page.goto("/admin/game");
  await openDisclosureIfNeeded(page.getByTestId("scheduler-audit-disclosure"));

  await expect(page.getByText("scheduler started")).toBeVisible();
  await expect(page.getByText("tick #12 completed")).toBeVisible();
  await expect(
    page.getByText("scheduler stopped after match end"),
  ).toBeVisible();
  await expect(page.getByText("stopped").first()).toBeVisible();
});

test("organizer game page shows checker history with faust service state summaries", async ({
  page,
}) => {
  await page.goto("/admin/game");
  await openDisclosureIfNeeded(
    page.getByTestId("checker-investigation-disclosure"),
  );
  await openDisclosureIfNeeded(page.getByTestId("checker-runs-disclosure"));

  const checkerCard = page.getByTestId("checker-runs-card");
  await expect(checkerCard.getByText("Service State")).toBeVisible();
  await expect(checkerCard.getByText("ok").first()).toBeVisible();
  await expect(
    checkerCard.getByText(
      "service passed storage, retrieval, and functionality checks",
    ).first(),
  ).toBeVisible();
});

test("organizer game page shows explicit empty states for scheduler and checker history", async ({
  page,
  request,
}) => {
  await request.post(`${mockApiBaseUrl}/__reset`, {
    data: { scenario: "empty-game-history" },
  });
  await page.goto("/admin/game");

  await expect(
    page.getByTestId("scheduler-audit-disclosure").locator("summary"),
  ).toBeVisible();
  await openDisclosureIfNeeded(page.getByTestId("scheduler-audit-disclosure"));
  await openDisclosureIfNeeded(
    page.getByTestId("checker-investigation-disclosure"),
  );
  await openDisclosureIfNeeded(page.getByTestId("checker-runs-disclosure"));
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

  await openDisclosureIfNeeded(page.getByTestId("scheduler-audit-disclosure"));
  await openDisclosureIfNeeded(
    page.getByTestId("checker-investigation-disclosure"),
  );
  await openDisclosureIfNeeded(page.getByTestId("checker-runs-disclosure"));
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

test("organizer game page can copy a concise runtime summary", async ({
  page,
}) => {
  await page.goto("/admin/game");

  await page.getByTestId("copy-runtime-health-summary").click();

  await expect(
    page.getByText("Copied runtime summary to clipboard."),
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
  await expect(metricsCard.getByText("Checker runs: 72")).toBeVisible();

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
  await expect(
    metricsCard.getByText("Game Core: scheduler stopped, 2 checker failures"),
  ).toBeVisible();
  await expect(
    metricsCard.getByText("Submission: 3 submit failures"),
  ).toBeVisible();
  await expect(
    metricsCard.getByText("Realtime: last sync failed, 4 sync errors"),
  ).toBeVisible();
});

test("organizer scheduler controls can start, update, and stop with visible notes", async ({
  page,
}) => {
  await page.goto("/admin/game");

  const schedulerCard = page.getByTestId("scheduler-card");

  await schedulerCard.getByRole("button", { name: "Resume Scheduler" }).click();
  await expectActionNote(page, /Scheduler started with 60s interval\./);
  await expect(
    schedulerCard.getByRole("button", { name: "Stop Scheduler" }),
  ).toBeEnabled();

  const intervalInput = schedulerCard.locator('input[type="number"]:visible').first();
  await intervalInput.fill("90");
  await schedulerCard.getByRole("button", { name: "Update" }).click();
  await expectActionNote(page, /Scheduler interval updated to 90 seconds\./);
  await expect(intervalInput).toHaveValue("90");

  await schedulerCard.getByRole("button", { name: "Stop Scheduler" }).click();
  await expectActionNote(page, /Scheduler stopped\./);
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

  await expectActionNote(
    page,
    /Game started\. Submissions are open and the scheduler is running at 60s\./,
  );
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

  await expectActionNote(
    page,
    /Trusted reconcile processed 1 job\(s\), advanced 2 team service instance\(s\), and refreshed deployment, SSH access, and WireGuard truth\./,
  );
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

test("organizer game page shows aggregated runtime drift warnings and refreshes them together", async ({
  page,
  request,
}) => {
  await request.post(`${mockApiBaseUrl}/__reset`, {
    data: { scenario: "default" },
  });
  await page.goto("/admin/game");

  const runtimeHealthCard = page.getByTestId("runtime-health-card");
  await expect(runtimeHealthCard).toContainText(
    "Trusted reconcile, controller access, WireGuard, deployments, and live metrics currently agree.",
  );

  await request.post(`${mockApiBaseUrl}/__reset`, {
    data: { scenario: "runtime-health-drift" },
  });

  await page.getByTestId("refresh-runtime-health").click();

  await expectActionNote(
    page,
    /Refreshed deployment, access policy, WireGuard, runtime alerts, and service metrics\./,
  );
  await expect(runtimeHealthCard).toContainText(
    "Runtime drift warnings are active. Review deployments, access, and gateway state before assuming the stack is converged.",
  );
  await expect(runtimeHealthCard).toContainText("Incident workflow");
  await expect(runtimeHealthCard).toContainText("1. Refresh truth");
  await expect(runtimeHealthCard).toContainText("2. Inspect drift");
  await expect(runtimeHealthCard).toContainText("3. Capture evidence");
  await expect(runtimeHealthCard).toContainText("1 pending / 0 failed");
  await expect(runtimeHealthCard).toContainText("idle (host)");
  await expect(runtimeHealthCard).toContainText(
    "Controller access policy is idle, so SSH truth may be stale.",
  );
  await expect(runtimeHealthCard).toContainText(
    "WireGuard gateway is idle, so peer truth may be stale.",
  );
  await expect(runtimeHealthCard).toContainText(
    "1 deployment job(s) still need trusted reconcile completion.",
  );
});

test("organizer game page can download a runtime evidence report", async ({
  page,
}) => {
  await page.goto("/admin/game");

  const [download] = await Promise.all([
    page.waitForEvent("download"),
    page.getByTestId("download-runtime-health-report").click(),
  ]);

  expect(download.suggestedFilename()).toMatch(/^runtime-health-.*\.json$/);

  const downloadPath = await download.path();
  expect(downloadPath).not.toBeNull();

  const report = JSON.parse(await readFile(downloadPath!, "utf8"));
  expect(Array.isArray(report.failures)).toBeTruthy();
  expect(report.deployments).toBeTruthy();
  expect(report.access_status).toEqual(
    expect.objectContaining({ state: expect.any(String) }),
  );
  expect(report.wireguard_status).toEqual(
    expect.objectContaining({ state: expect.any(String) }),
  );
  expect(report.operations_status).toEqual(
    expect.objectContaining({ healthy: expect.any(Boolean) }),
  );
  expect(report.summary).toContain("Report completeness: all sections loaded");
  await expectActionNote(
    page,
    /Downloaded complete runtime evidence report as .*\.json\./,
  );
});

test("organizer game page names missing sections in partial runtime evidence", async ({
  page,
  request,
}) => {
  await page.goto("/admin/game");
  await request.post(`${mockApiBaseUrl}/__reset`, {
    data: { scenario: "runtime-report-partial" },
  });

  const [download] = await Promise.all([
    page.waitForEvent("download"),
    page.getByTestId("download-runtime-health-report").click(),
  ]);
  const downloadPath = await download.path();
  expect(downloadPath).not.toBeNull();

  const report = JSON.parse(await readFile(downloadPath!, "utf8"));
  expect(report.failures).toEqual([
    expect.objectContaining({ section: "wireguard_status" }),
  ]);
  expect(report.summary).toContain(
    "Report completeness: partial, missing wireguard_status",
  );
  await expectActionNote(
    page,
    /Downloaded partial runtime evidence report as .*missing wireguard_status/,
  );
  await expect(page.getByText("missing wireguard_status")).toBeVisible();
});

test("organizer match controls handle manual match operations and recompute logic", async ({
  page,
  request,
}) => {
  await setupGameTest(page, request, "prestart");

  const matchCard = page.getByTestId("match-card");
  const quickActionsCard = page.getByTestId("quick-actions-card");

  await page.getByTestId("start-match").click();
  await expectActionNote(
    page,
    /Game started\. Submissions are open and the scheduler is running at 60s\./,
  );

  await page.getByTestId("stop-match").click();
  await expectActionNote(
    page,
    /Match stopped\. Participant submissions are now closed\./,
  );
  await expect(matchCard.getByText("finished").first()).toBeVisible();
  await expect(matchCard.getByText("closed").first()).toBeVisible();

  await page.getByTestId("advance-tick").click();
  await expectActionNote(
    page,
    /Tick 13 completed with 6 success, 0 failed, and 0 skipped checker runs\./,
  );
  await expect(quickActionsCard.getByText("#13 completed")).toBeVisible();

  await page.getByTestId("recompute-scores").click();
  await expectActionNote(
    page,
    /Recomputed 3 scoreboard row\(s\) from authoritative tick and submission state\./,
  );

  await page.getByRole("link", { name: "Scoreboard", exact: true }).click();
  const scoringAuditCard = page.getByTestId("scoring-audit-card");
  await expect(scoringAuditCard).toBeVisible();
  await scoringAuditCard.getByRole("button", { name: "Audit" }).click();
  await expectActionNote(
    page,
    /Score audit passed: 3 replayed row\(s\) match the stored scoreboard\./,
  );
  await expect(scoringAuditCard.getByText("ok")).toBeVisible();
  await expect(scoringAuditCard.getByText("0")).toBeVisible();
});
