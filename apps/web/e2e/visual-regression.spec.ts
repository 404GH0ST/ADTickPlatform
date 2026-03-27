import { expect, test, type Locator, type Page } from "@playwright/test";

const mockApiBaseUrl = "http://127.0.0.1:4010";

test.use({ viewport: { width: 1280, height: 900 } });

async function stabilize(page: Page, locator: Locator) {
  await locator.scrollIntoViewIfNeeded();
  await page.evaluate(async () => {
    await document.fonts?.ready;
    await new Promise<void>((resolve) => {
      requestAnimationFrame(() => {
        requestAnimationFrame(() => resolve());
      });
    });
  });
  await page.waitForTimeout(100);
}

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
  await stabilize(page, serviceCard);
  await expect(serviceCard).toHaveScreenshot("participant-service-card-dark.png");
});

test("participant attack map panel keeps its visual baseline", async ({
  page,
}) => {
  await page.goto("/attacks");

  const attackMapPanel = page.getByTestId("attack-map-panel");
  await expect(attackMapPanel).toBeVisible();
  await stabilize(page, attackMapPanel);
  await expect(attackMapPanel).toHaveScreenshot(
    "participant-attack-map-panel-dark.png",
  );
});

test("organizer scheduler card keeps its visual baseline", async ({ page }) => {
  await page.goto("/admin/game");

  const schedulerCard = page.getByTestId("scheduler-card");
  await expect(schedulerCard).toBeVisible();
  await stabilize(page, schedulerCard);
  await expect(schedulerCard).toHaveScreenshot("organizer-scheduler-card-dark.png");
});

test("organizer authoritative scoreboard card keeps its visual baseline", async ({
  page,
}) => {
  await page.goto("/admin/scoreboard");

  const scoreboardCard = page.getByTestId("authoritative-scoreboard-card");
  await expect(scoreboardCard).toBeVisible();
  await stabilize(page, scoreboardCard);
  await expect(scoreboardCard).toHaveScreenshot(
    "organizer-authoritative-scoreboard-card-dark.png",
  );
});
