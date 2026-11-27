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

test("login and registration reject oversized and malformed credentials", async ({ request }) => {
  for (const action of ["login", "register"]) {
    const url = `/api/platform/session/${action}`;
    const oversized = await request.post(url, { data: { password: "x".repeat(16 * 1024) } });
    expect(oversized.status()).toBe(413);
    const malformed = await request.post(url, { data: { email: [], password: true } });
    expect(malformed.status()).toBe(400);
  }
});

test("participant account updates reject oversized bodies and invalid field types", async ({ request }) => {
  for (const [method, path, field] of [
    ["POST", "team", "team_key"],
    ["POST", "team/captain", "player_id"],
    ["PUT", "profile", "display_name"],
    ["PUT", "password", "new_password"],
  ]) {
    const url = `/api/platform/session/${path}`;
    const oversized = await request.fetch(url, { method, data: { [field]: "x".repeat(16 * 1024) } });
    expect(oversized.status()).toBe(413);
    const invalid = await request.fetch(url, { method, data: { [field]: { value: true } } });
    expect(invalid.status()).toBe(400);
    const crossSite = await request.fetch(url, { method, headers: { Origin: "https://different.example" }, data: {} });
    expect(crossSite.status()).toBe(403);
  }
});

test("flag submissions require same-origin JSON, authentication, and a bounded body", async ({
  baseURL,
  context,
}) => {
  const url = "/api/platform/submit";
  const origin = new URL(baseURL!).origin;
  const data = { flags: ["test-flag"] };

  const unsigned = await context.request.post(url, { headers: { Origin: origin }, data });
  expect(unsigned.status()).toBe(403);
  await setParticipantSession(context);

  const wrongOrigin = await context.request.post(url, {
    headers: { Origin: "https://different.example" },
    data,
  });
  expect(wrongOrigin.status()).toBe(403);
  const unsupported = await context.request.post(url, {
    headers: { Origin: origin, "Content-Type": "text/plain" },
    data: JSON.stringify(data),
  });
  expect(unsupported.status()).toBe(415);
  const oversized = await context.request.post(url, {
    headers: { Origin: origin },
    data: { flags: ["x".repeat(128 * 1024)] },
  });
  expect(oversized.status()).toBe(413);
  const allowed = await context.request.post(url, { headers: { Origin: origin }, data });
  expect(allowed.status()).toBe(200);
  expect(await allowed.json()).toMatchObject({ rejected_count: 1 });
});

test("participant announcement remains visible while switching pages", async ({
  page,
}) => {
  let requestCount = 0;
  await page.route("**/api/platform/announcements", async (route) => {
    requestCount += 1;
    if (requestCount > 1) {
      await new Promise((resolve) => setTimeout(resolve, 1500));
    }
    await route.fulfill({
      contentType: "application/json",
      body: JSON.stringify([
        {
          id: 1,
          body: "Scoring resumes at the next tick.",
          created_at: "2026-07-12T00:00:00Z",
        },
      ]),
    });
  });

  await page.goto("/");
  const announcement = page.getByText(
    "Organizer: Scoring resumes at the next tick.",
  );
  await expect(announcement).toBeVisible();

  await page.getByRole("link", { name: "Scoreboard", exact: true }).click();
  await expect(page).toHaveURL(/\/scoreboard$/);
  await expect(announcement).toBeVisible({ timeout: 500 });
});

type ApiCase = {
  method: "GET" | "POST" | "PUT" | "DELETE";
  path: string;
  data?: unknown;
};

const adminApiCases: ApiCase[] = [
  { method: "GET", path: "/api/admin/access/status" },
  { method: "POST", path: "/api/admin/access/reconcile" },
  { method: "POST", path: "/api/admin/access/teardown" },
  { method: "POST", path: "/api/admin/challenges" },
  { method: "PUT", path: "/api/admin/challenges/1" },
  { method: "DELETE", path: "/api/admin/challenges/1" },
  { method: "POST", path: "/api/admin/challenges/1/deploy" },
  { method: "POST", path: "/api/admin/challenges/1/validate" },
  { method: "GET", path: "/api/admin/deployments" },
  { method: "DELETE", path: "/api/admin/deployments/77" },
  { method: "POST", path: "/api/admin/deployments/reconcile" },
  { method: "GET", path: "/api/admin/deployments/reconcile/job" },
  { method: "POST", path: "/api/admin/deployments/reconcile/job" },
  { method: "GET", path: "/api/admin/game/attacks" },
  { method: "GET", path: "/api/admin/game/checker-runs" },
  { method: "GET", path: "/api/admin/game/match" },
  { method: "PUT", path: "/api/admin/game/match/schedule" },
  { method: "POST", path: "/api/admin/game/match/start" },
  { method: "POST", path: "/api/admin/game/match/stop" },
  { method: "GET", path: "/api/admin/game/scheduler/events" },
  { method: "PUT", path: "/api/admin/game/scheduler/interval" },
  { method: "GET", path: "/api/admin/game/scheduler" },
  { method: "POST", path: "/api/admin/game/scheduler/start" },
  { method: "POST", path: "/api/admin/game/scheduler/stop" },
  { method: "GET", path: "/api/admin/game/scoreboard" },
  { method: "GET", path: "/api/admin/game/scoring/audit" },
  { method: "POST", path: "/api/admin/game/scoring/recompute" },
  { method: "GET", path: "/api/admin/game/status" },
  { method: "POST", path: "/api/admin/game/ticks/advance" },
  { method: "GET", path: "/api/admin/operations/metrics" },
  { method: "GET", path: "/api/admin/operations/status" },
  { method: "POST", path: "/api/admin/players" },
  { method: "PUT", path: "/api/admin/players/1001" },
  { method: "DELETE", path: "/api/admin/players/1001" },
  { method: "GET", path: "/api/admin/players/1001/wireguard" },
  { method: "POST", path: "/api/admin/players/1001/wireguard/revoke" },
  { method: "POST", path: "/api/admin/players/1001/wireguard/rotate" },
  { method: "GET", path: "/api/admin/realtime/game/attacks/stream" },
  { method: "GET", path: "/api/admin/realtime/game/checker-runs/stream" },
  { method: "GET", path: "/api/admin/realtime/game/scheduler/events/stream" },
  { method: "GET", path: "/api/admin/realtime/game/scoreboard/stream" },
  { method: "GET", path: "/api/admin/realtime/game/status/stream" },
  { method: "GET", path: "/api/admin/runtime-health/report" },
  { method: "POST", path: "/api/admin/teams" },
  { method: "POST", path: "/api/admin/teams/101/join-key/rotate" },
  { method: "PUT", path: "/api/admin/teams/101" },
  { method: "DELETE", path: "/api/admin/teams/101" },
  { method: "POST", path: "/api/admin/wireguard/reconcile" },
  { method: "GET", path: "/api/admin/wireguard/status" },
  { method: "POST", path: "/api/admin/wireguard/teardown" },
];

const participantOwnedApiCases: ApiCase[] = [
  { method: "GET", path: "/api/platform/team/services" },
  { method: "GET", path: "/api/platform/challenges/1/source" },
  { method: "GET", path: "/api/platform/me/wireguard" },
  { method: "POST", path: "/api/platform/services/1/ssh-session" },
  { method: "POST", path: "/api/platform/services/1/reset/factory" },
  { method: "POST", path: "/api/platform/services/1/reset/restart" },
];

async function loginAsParticipant(page: Page) {
  await page.goto("/login");
  await page.getByLabel("Email").fill("alpha.captain@college.local");
  await page.getByLabel("Password").fill("alpha-password");
  await page.locator('form button[type="submit"]').click();
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
  await page.locator('form button[type="submit"]').click();

  await expect(
    page.getByText("email or password is wrong."),
  ).toBeVisible();
  await expect(page).toHaveURL(/\/login$/);
});

test("participant can sign in and sign out through the session routes", async ({
  page,
  request,
}) => {
  await loginAsParticipant(page);
  const session = (await page.context().cookies()).find(({ name }) => name === "ad_platform_team_jwt");
  expect(session).toBeTruthy();
  const headers = { Authorization: `Bearer ${session!.value}` };
  const before = await request.get(`${mockApiBaseUrl}/api/v2/session`, { headers });
  expect(before.status()).toBe(200);

  await page.getByRole("button", { name: "Sign Out" }).click();

  await expect(page).toHaveURL(/\/$/);
  await expect(page.getByRole("link", { name: "Sign In" })).toBeVisible();
  const replay = await request.get(`${mockApiBaseUrl}/api/v2/session`, { headers });
  expect(replay.status()).toBe(403);
});

test("registered participant can join a team with a team key", async ({ page }) => {
  await page.goto("/login");

  await page.getByRole("button", { name: "Register" }).click();
  await page.getByLabel("Display name").fill("Joined Member");
  await page.getByLabel("Email").fill("joined.member@college.local");
  await page.getByLabel("Password").fill("joined-password");
  await page.locator('form button[type="submit"]').click();

  await expect(page).toHaveURL(/\/services$/);
  await expect(page.locator("h1", { hasText: "Services" })).toBeVisible();
  await expect(page.getByLabel("Team key")).toBeVisible();

  await page.getByLabel("Team key").fill("TEAM-ALPHA-JOIN");
  await page.getByRole("button", { name: "Join Team" }).click();

  await expect(page.getByTestId("participant-vpn-config")).toBeVisible();
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

test("unauthenticated callers cannot use admin api methods", async ({
  page,
}) => {
  for (const item of adminApiCases) {
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

test("participant session cannot use admin api methods", async ({
  page,
  context,
}) => {
  await setParticipantSession(context);

  for (const item of adminApiCases) {
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
  for (const item of participantOwnedApiCases) {
    const response = await page.request.fetch(item.path, {
      method: item.method,
      data: item.data,
    });
    expect(response.status(), `${item.method} ${item.path}`).toBe(403);
  }
});

test("browser mutation api proxies reject explicit cross-site requests", async ({
  page,
  context,
}) => {
  await loginAsOrganizer(context);

  const cases = [
    { method: "POST", path: "/api/admin/game/ticks/advance" },
    { method: "POST", path: "/api/platform/services/1/reset/restart" },
    {
      method: "POST",
      path: "/api/platform/session/login",
      data: { email: "alpha.captain@college.local", password: "alpha-password" },
    },
    { method: "POST", path: "/api/platform/session/logout" },
  ];

  for (const item of cases) {
    const response = await page.request.fetch(item.path, {
      method: item.method,
      data: item.data,
      headers: {
        Origin: "https://attacker.example",
      },
    });
    expect(response.status(), `${item.method} ${item.path}`).toBe(403);
    await expect(await response.json()).toEqual({
      title: "Authentication required",
      status: 403,
      detail: "cross-site mutation requests are not allowed.",
    });
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
