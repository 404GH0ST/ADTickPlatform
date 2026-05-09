import {
  type APIRequestContext,
  type BrowserContext,
  expect,
  test as base,
} from "@playwright/test";
import { createHmac } from "node:crypto";

export const mockApiBaseUrl = "http://127.0.0.1:4010";
export const testTeamJWTSecret = "playwright-team-jwt-secret";

function base64UrlJSON(value: unknown) {
  return Buffer.from(JSON.stringify(value))
    .toString("base64url");
}

function signTestSessionToken(claims: Record<string, unknown>) {
  const header = base64UrlJSON({ alg: "HS256", typ: "JWT" });
  const payload = base64UrlJSON({
    ...claims,
    iat: 1_777_777_777,
    exp: 4_102_444_800,
  });
  const signature = createHmac("sha256", testTeamJWTSecret)
    .update(`${header}.${payload}`)
    .digest("base64url");
  return `${header}.${payload}.${signature}`;
}

export const organizerSessionToken = signTestSessionToken({
  team_id: 101,
  player_id: 1,
  team_name: "College Alpha",
  display_name: "Organizer",
  email: "organizer@college.local",
  role: "organizer",
});
export const participantSessionToken = signTestSessionToken({
  team_id: 101,
  player_id: 1001,
  team_name: "College Alpha",
  display_name: "Alpha Captain",
  email: "alpha.captain@college.local",
  role: "captain",
});

export const participantStorageState = {
  cookies: [
    {
      name: "ad_platform_team_jwt",
      value: participantSessionToken,
      domain: "127.0.0.1",
      path: "/",
      expires: -1,
      httpOnly: true,
      secure: false,
      sameSite: "Lax" as const,
    },
  ],
  origins: [],
};

export async function loginAsOrganizer(context: BrowserContext) {
  await context.addCookies([
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
}

export async function loginAsParticipant(context: BrowserContext) {
  await context.addCookies([
    {
      name: "ad_platform_team_jwt",
      value: participantSessionToken,
      domain: "127.0.0.1",
      path: "/",
      expires: -1,
      httpOnly: true,
      secure: false,
      sameSite: "Lax",
    },
  ]);
}

export async function resetMockApi(
  request: APIRequestContext,
  scenario = "default",
) {
  const response = await request.post(`${mockApiBaseUrl}/__reset`, {
    data: { scenario },
  });
  expect(response.ok()).toBeTruthy();
}

/**
 * Pre-configured test fixture for admin spec files.
 * Sets viewport to 1280×900 and resets the mock API before each test.
 */
export const adminTest = base.extend({});
adminTest.use({ viewport: { width: 1280, height: 900 } });
adminTest.beforeEach(async ({ request }) => {
  await resetMockApi(request);
});

/**
 * Pre-configured test fixture for realtime spec files.
 * Sets viewport to 1280×900 and resets the mock API with `realtime-updates` scenario.
 */
export const realtimeTest = base.extend({});
realtimeTest.use({ viewport: { width: 1280, height: 900 } });
realtimeTest.beforeEach(async ({ request }) => {
  await resetMockApi(request, "realtime-updates");
});
