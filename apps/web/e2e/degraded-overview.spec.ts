import { expect, test } from "@playwright/test";

const mockApiBaseUrl = "http://127.0.0.1:4010";
const organizerSessionToken =
  "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0.eyJ0ZWFtX2lkIjoxMDEsInBsYXllcl9pZCI6MSwidGVhbV9uYW1lIjoiQ29sbGVnZSBBbHBoYSIsImRpc3BsYXlfbmFtZSI6Ik9yZ2FuaXplciIsImVtYWlsIjoib3JnYW5pemVyQGNvbGxlZ2UubG9jYWwiLCJyb2xlIjoib3JnYW5pemVyIn0.";

test.use({
  viewport: { width: 1280, height: 900 },
  storageState: { cookies: [], origins: [] },
});

test("participant overview shows degraded-data warning when public feeds fail", async ({
  page,
  request,
}) => {
  await request.post(`${mockApiBaseUrl}/__reset`, {
    data: { scenario: "degraded-participant" },
  });

  await page.goto("/", {
    waitUntil: "networkidle",
  });

  await expect(page.locator("h1", { hasText: "Participant Overview" })).toBeVisible();
  await expect(
    page.getByText(
      "Participant login is required for owned services, unlock, SSH, and reset actions. Participant data is partially unavailable. Only live responses that succeeded are shown. No sample data is injected.",
    ),
  ).toBeVisible();
  await expect(page.getByText("0 accepted events")).toBeVisible();
});

test("organizer overview shows degraded-data warning when admin feeds fail", async ({
  page,
  request,
}) => {
  await request.post(`${mockApiBaseUrl}/__reset`, {
    data: { scenario: "degraded-admin" },
  });

  await page.context().addCookies([
    {
      name: "ad_platform_team_jwt",
      value: organizerSessionToken,
      domain: "127.0.0.1",
      path: "/",
      expires: -1,
      httpOnly: true,
      secure: false,
      sameSite: "Lax",
    },
  ]);

  await page.goto("/admin", {
    waitUntil: "networkidle",
  });

  await expect(page.locator("h1", { hasText: "Organizer Overview" })).toBeVisible();
  await expect(
    page.getByText(
      "Organizer data is partially unavailable. Only live responses that succeeded are shown. No sample data is injected.",
    ),
  ).toBeVisible();
  await expect(page.getByText("0 rows")).toBeVisible();
});

test("organizer overview refreshes runtime alerts on focus", async ({
  page,
  request,
}) => {
  await request.post(`${mockApiBaseUrl}/__reset`, {
    data: { scenario: "default" },
  });

  await page.context().addCookies([
    {
      name: "ad_platform_team_jwt",
      value: organizerSessionToken,
      domain: "127.0.0.1",
      path: "/",
      expires: -1,
      httpOnly: true,
      secure: false,
      sameSite: "Lax",
    },
  ]);

  await page.goto("/admin", {
    waitUntil: "networkidle",
  });

  await expect(page.getByTestId("organizer-summary")).toContainText("Ops Alerts");
  await expect(page.getByTestId("organizer-summary")).toContainText("0");

  await request.post(`${mockApiBaseUrl}/__reset`, {
    data: { scenario: "ops-alerts" },
  });

  await page.evaluate(() => {
    window.dispatchEvent(new Event("focus"));
  });

  await expect(page.getByText("Scheduler next run is overdue.")).toBeVisible();
  await expect(page.getByText("2 deployment job(s) are still active.")).toBeVisible();
  await expect(page.getByTestId("organizer-summary")).toContainText("2");
});
