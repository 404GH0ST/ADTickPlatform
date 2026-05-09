import { defineConfig, devices } from "@playwright/test";
import { createHmac } from "node:crypto";

const mockApiPort = 4010;
const appPort = 3007;
const mockApiUrl = `http://127.0.0.1:${mockApiPort}`;
const appUrl = `http://127.0.0.1:${appPort}`;
const testTeamJWTSecret = "playwright-team-jwt-secret";

function base64UrlJSON(value: unknown) {
  return Buffer.from(JSON.stringify(value)).toString("base64url");
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

const organizerSessionToken = signTestSessionToken({
  team_id: 101,
  player_id: 1,
  team_name: "College Alpha",
  display_name: "Organizer",
  email: "organizer@college.local",
  role: "organizer",
});
const organizerStorageState = {
  cookies: [
    {
      name: "ad_platform_team_jwt",
      value: organizerSessionToken,
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

export default defineConfig({
  testDir: "./e2e",
  fullyParallel: false,
  workers: 1,
  retries: process.env.CI ? 2 : 0,
  expect: {
    toHaveScreenshot: {
      animations: "disabled",
      caret: "hide",
      maxDiffPixelRatio: 0.01,
    },
  },
  reporter: process.env.CI ? [["github"], ["html", { open: "never" }]] : "list",
  timeout: 30_000,
  use: {
    baseURL: appUrl,
    screenshot: "only-on-failure",
    trace: "retain-on-failure",
    video: "retain-on-failure",
    storageState: organizerStorageState,
  },
  projects: [
    {
      name: "chromium",
      use: {
        ...devices["Desktop Chrome"],
        viewport: { width: 1280, height: 900 },
      },
    },
  ],
  webServer: [
    {
      command: `MOCK_PLATFORM_API_PORT=${mockApiPort} node e2e/mock-platform-api.mjs`,
      port: mockApiPort,
      reuseExistingServer: !process.env.CI,
      stdout: "pipe",
      stderr: "pipe",
    },
    {
      command: [
        `AD_PLATFORM_API_URL=${mockApiUrl}`,
        `AD_PLATFORM_GAME_CORE_URL=${mockApiUrl}/game-core`,
        `AD_PLATFORM_SUBMISSION_SERVICE_URL=${mockApiUrl}/submission-service`,
        `AD_PLATFORM_REALTIME_URL=${mockApiUrl}`,
        `AD_PLATFORM_CONTROLLER_METRICS_URL=${mockApiUrl}/controller-service`,
        `AD_PLATFORM_WIREGUARD_GATEWAY_URL=${mockApiUrl}/wireguard-gateway`,
        "ADMIN_API_TOKEN=dev-admin-token",
        `TEAM_JWT_SECRET=${testTeamJWTSecret}`,
        "NEXT_TELEMETRY_DISABLED=1",
        `bun run start --hostname 127.0.0.1 --port ${appPort}`,
      ].join(" "),
      url: appUrl,
      reuseExistingServer: !process.env.CI,
      stdout: "pipe",
      stderr: "pipe",
    },
  ],
});
