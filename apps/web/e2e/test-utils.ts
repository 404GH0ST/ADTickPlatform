import {
  type APIRequestContext,
  type BrowserContext,
  expect,
  test as base,
} from "@playwright/test";

export const mockApiBaseUrl = "http://127.0.0.1:4010";
export const organizerSessionToken =
  "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0.eyJ0ZWFtX2lkIjoxMDEsInBsYXllcl9pZCI6MSwidGVhbV9uYW1lIjoiQ29sbGVnZSBBbHBoYSIsImRpc3BsYXlfbmFtZSI6Ik9yZ2FuaXplciIsImVtYWlsIjoib3JnYW5pemVyQGNvbGxlZ2UubG9jYWwiLCJyb2xlIjoib3JnYW5pemVyIn0.";
export const participantSessionToken =
  "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0.eyJ0ZWFtX2lkIjoxMDEsInBsYXllcl9pZCI6MTAwMSwidGVhbV9uYW1lIjoiQ29sbGVnZSBBbHBoYSIsImRpc3BsYXlfbmFtZSI6IkFscGhhIENhcHRhaW4iLCJlbWFpbCI6ImFscGhhLmNhcHRhaW5AY29sbGVnZS5sb2NhbCIsInJvbGUiOiJjYXB0YWluIn0.";

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
