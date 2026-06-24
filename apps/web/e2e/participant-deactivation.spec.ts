import { expect, test } from "@playwright/test";
import { loginAsParticipant, mockApiBaseUrl } from "./test-utils";

test.use({
  viewport: { width: 1280, height: 900 },
  storageState: { cookies: [], origins: [] },
});

test.beforeEach(async ({ request }) => {
  await request.post(`${mockApiBaseUrl}/__reset`);
});

test("deactivated participant sees a clear notice instead of a generic prompt", async ({
  context,
  page,
  request,
}) => {
  await loginAsParticipant(context);

  // Deactivate the participant's player (1001) on the gateway. Their session is
  // re-validated on the next request, so the dashboard must explain why access
  // stopped rather than showing the generic "please log in" prompt.
  const res = await request.post(
    `${mockApiBaseUrl}/api/v2/admin/players/1001/deactivate`,
  );
  expect(res.ok()).toBeTruthy();

  await page.goto("/login");

  const notice = page.getByTestId("deactivated-notice");
  await expect(notice).toBeVisible();
  await expect(notice).toContainText("deactivated by the organizers");
});
