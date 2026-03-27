import { expect, test } from "@playwright/test";

const mockApiBaseUrl = "http://127.0.0.1:4010";

test.use({ viewport: { width: 1280, height: 900 } });

test.beforeEach(async ({ page, request }) => {
  await request.post(`${mockApiBaseUrl}/__reset`);
  await page.emulateMedia({ colorScheme: "dark" });
  await page.addInitScript(() => {
    window.localStorage.removeItem("ad-platform-theme");
  });
});

test("participant service card keeps its visual baseline", async ({ page }) => {
  await page.goto("/services");

  const serviceCard = page.getByTestId("service-card-svc-1");
  await expect(serviceCard).toBeVisible();
  await expect(serviceCard).toHaveScreenshot(
    "participant-service-card-dark.png",
    {
      animations: "disabled",
      caret: "hide",
    },
  );
});

test("participant attack map panel keeps its visual baseline", async ({
  page,
}) => {
  await page.goto("/attacks");

  const attackMapPanel = page.getByTestId("attack-map-panel");
  await expect(attackMapPanel).toBeVisible();
  await expect(attackMapPanel).toHaveScreenshot(
    "participant-attack-map-panel-dark.png",
    {
      animations: "disabled",
      caret: "hide",
    },
  );
});

test("organizer scheduler card keeps its visual baseline", async ({ page }) => {
  await page.goto("/admin/game");

  const schedulerCard = page.getByTestId("scheduler-card");
  await expect(schedulerCard).toBeVisible();
  await expect(schedulerCard).toHaveScreenshot(
    "organizer-scheduler-card-dark.png",
    {
      animations: "disabled",
      caret: "hide",
    },
  );
});

test("organizer authoritative scoreboard card keeps its visual baseline", async ({
  page,
}) => {
  await page.goto("/admin/scoreboard");

  const scoreboardCard = page.getByTestId("authoritative-scoreboard-card");
  await expect(scoreboardCard).toBeVisible();
  await expect(scoreboardCard).toHaveScreenshot(
    "organizer-authoritative-scoreboard-card-dark.png",
    {
      animations: "disabled",
      caret: "hide",
    },
  );
});
