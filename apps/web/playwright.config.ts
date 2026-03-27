import { defineConfig, devices } from "@playwright/test";

const mockApiPort = 4010;
const appPort = 3007;
const mockApiUrl = `http://127.0.0.1:${mockApiPort}`;
const appUrl = `http://127.0.0.1:${appPort}`;
const organizerSessionToken =
  "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0.eyJ0ZWFtX2lkIjoxMDEsInBsYXllcl9pZCI6MSwidGVhbV9uYW1lIjoiQ29sbGVnZSBBbHBoYSIsImRpc3BsYXlfbmFtZSI6Ik9yZ2FuaXplciIsImVtYWlsIjoib3JnYW5pemVyQGNvbGxlZ2UubG9jYWwiLCJyb2xlIjoib3JnYW5pemVyIn0.";
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
        `AD_PLATFORM_REALTIME_URL=${mockApiUrl}`,
        "ADMIN_API_TOKEN=dev-admin-token",
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
