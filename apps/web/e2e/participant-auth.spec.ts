import { expect, test, Page } from "@playwright/test";
import {
  loginAsOrganizer,
  loginAsParticipant as setParticipantSession,
  mockApiBaseUrl,
  organizerSessionToken,
} from "./test-utils";

test.use({
  viewport: { width: 1280, height: 900 },
  storageState: { cookies: [], origins: [] },
});

test.beforeEach(async ({ request }) => {
  await request.post(`${mockApiBaseUrl}/__reset`);
});

async function loginAsParticipant(page: Page) {
  await page.goto("/login");
  await page.getByLabel("Email").fill("alpha.captain@college.local");
  await page.getByLabel("Password").fill("alpha-password");
  await page.getByRole("button", { name: "Sign In" }).click();
  await expect(page).toHaveURL(/\/services$/);
  await expect(page.locator("h1", { hasText: "Services" })).toBeVisible();
}

test("unauthenticated admin access redirects to the participant login page", async ({
  page,
}) => {
  await page.goto("/admin");

  await expect(page).toHaveURL(/\/login$/);
  await expect(
    page.getByRole("heading", { name: "Sign In" }),
  ).toBeVisible();
});

test("participant login shows an error for invalid credentials", async ({
  page,
}) => {
  await page.goto("/login");

  await page.getByLabel("Email").fill("alpha.captain@college.local");
  await page.getByLabel("Password").fill("wrong-password");
  await page.getByRole("button", { name: "Sign In" }).click();

  await expect(
    page.getByText("email or password is wrong."),
  ).toBeVisible();
  await expect(page).toHaveURL(/\/login$/);
});

test("participant can sign in and sign out through the session routes", async ({
  page,
}) => {
  await loginAsParticipant(page);

  await page.getByRole("button", { name: "Sign Out" }).click();

  await expect(page).toHaveURL(/\/$/);
  await expect(page.getByRole("link", { name: "Sign In" })).toBeVisible();
});

test("authenticated participants are redirected away from organizer routes and the login page", async ({
  page,
}) => {
  await loginAsParticipant(page);

  await page.goto("/admin");

  await expect(page).toHaveURL(/\/$/);
  await expect(page.locator("h1", { hasText: "Participant" })).toBeVisible();

  await page.goto("/login");

  await expect(page).toHaveURL(/\/services$/);
  await expect(page.locator("h1", { hasText: "Services" })).toBeVisible();
});

test("unauthenticated callers cannot use admin api proxies", async ({
  page,
}) => {
  const response = await page.request.get("/api/admin/operations/status");
  expect(response.status()).toBe(403);
  await expect(await response.json()).toEqual({
    title: "Authentication required",
    status: 403,
    detail: "please authenticate before accessing organizer routes.",
  });
});

test("unauthenticated callers cannot use representative admin api methods", async ({
  page,
}) => {
  const cases = [
    { method: "GET", path: "/api/admin/game/status" },
    { method: "GET", path: "/api/admin/realtime/game/status/stream" },
    { method: "POST", path: "/api/admin/game/ticks/advance" },
    {
      method: "PUT",
      path: "/api/admin/game/scheduler/interval",
      data: { interval_seconds: 90 },
    },
    { method: "DELETE", path: "/api/admin/teams/101" },
  ];

  for (const item of cases) {
    const response = await page.request.fetch(item.path, {
      method: item.method,
      data: item.data,
    });
    expect(response.status(), `${item.method} ${item.path}`).toBe(403);
    await expect(await response.json()).toEqual({
      title: "Authentication required",
      status: 403,
      detail: "please authenticate before accessing organizer routes.",
    });
  }
});

test("participant session cannot use admin api proxies", async ({
  page,
  context,
}) => {
  await setParticipantSession(context);

  const response = await page.request.get("/api/admin/operations/status");
  expect(response.status()).toBe(403);
  await expect(await response.json()).toEqual({
    title: "Authentication required",
    status: 403,
    detail: "please authenticate as organizer.",
  });
});

test("participant session cannot use representative admin api methods", async ({
  page,
  context,
}) => {
  await setParticipantSession(context);

  const cases = [
    { method: "GET", path: "/api/admin/game/status" },
    { method: "GET", path: "/api/admin/realtime/game/status/stream" },
    { method: "POST", path: "/api/admin/game/ticks/advance" },
    {
      method: "PUT",
      path: "/api/admin/game/scheduler/interval",
      data: { interval_seconds: 90 },
    },
    { method: "DELETE", path: "/api/admin/teams/101" },
  ];

  for (const item of cases) {
    const response = await page.request.fetch(item.path, {
      method: item.method,
      data: item.data,
    });
    expect(response.status(), `${item.method} ${item.path}`).toBe(403);
    await expect(await response.json()).toEqual({
      title: "Authentication required",
      status: 403,
      detail: "please authenticate as organizer.",
    });
  }
});

test("unauthenticated callers cannot use participant-owned api proxies", async ({
  page,
}) => {
  const cases = [
    { method: "GET", path: "/api/platform/team/services" },
    { method: "GET", path: "/api/platform/challenges/1/source" },
    { method: "POST", path: "/api/platform/services/1/ssh-session" },
    { method: "POST", path: "/api/platform/services/1/reset/factory" },
    { method: "POST", path: "/api/platform/services/1/reset/restart" },
    { method: "POST", path: "/api/platform/services/1/unlock", data: { proof: "bad" } },
  ];

  for (const item of cases) {
    const response = await page.request.fetch(item.path, {
      method: item.method,
      data: item.data,
    });
    expect(response.status(), `${item.method} ${item.path}`).toBe(403);
  }
});

test("organizer session can use admin api proxies", async ({
  page,
  context,
}) => {
  await loginAsOrganizer(context);

  const response = await page.request.get("/api/admin/operations/status");
  expect(response.status()).toBe(200);
  const payload = await response.json();
  await expect(payload.healthy).toBe(true);
  await expect(typeof payload.generated_at).toBe("string");
  await expect(Array.isArray(payload.alerts)).toBe(true);
});

test("demoted organizer session loses admin api proxy access", async ({
  page,
  context,
  request,
}) => {
  await loginAsOrganizer(context);

  const demoteResponse = await request.post(`${mockApiBaseUrl}/__set-player-role`, {
    data: { player_id: 1, role: "captain" },
  });
  expect(demoteResponse.ok()).toBeTruthy();

  const sessionResponse = await request.get(`${mockApiBaseUrl}/api/v2/session`, {
    headers: {
      Authorization: `Bearer ${organizerSessionToken}`,
    },
  });
  expect(sessionResponse.status()).toBe(403);

  const response = await page.request.get("/api/admin/operations/status");
  expect(response.status()).toBe(403);
  await expect(await response.json()).toEqual({
    title: "Authentication required",
    status: 403,
    detail: "please authenticate before accessing organizer routes.",
  });
});

test("deleted organizer session loses admin api proxy access", async ({
  page,
  context,
  request,
}) => {
  await loginAsOrganizer(context);

  const deleteResponse = await request.post(`${mockApiBaseUrl}/__delete-player`, {
    data: { player_id: 1 },
  });
  expect(deleteResponse.ok()).toBeTruthy();

  const sessionResponse = await request.get(`${mockApiBaseUrl}/api/v2/session`, {
    headers: {
      Authorization: `Bearer ${organizerSessionToken}`,
    },
  });
  expect(sessionResponse.status()).toBe(403);

  const response = await page.request.get("/api/admin/operations/status");
  expect(response.status()).toBe(403);
  await expect(await response.json()).toEqual({
    title: "Authentication required",
    status: 403,
    detail: "please authenticate before accessing organizer routes.",
  });
});
