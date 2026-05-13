import {
  expect,
  test,
  type APIRequestContext,
  type Locator,
  type Page,
} from "@playwright/test";

const mockApiBaseUrl = "http://127.0.0.1:4010";

test.use({ viewport: { width: 1280, height: 900 } });

type FeatureAttackOptions = {
  animate?: boolean;
  rotation?: {
    lat: number;
    lon: number;
  };
};

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

async function loadDenseAttackMap(page: Page, request: APIRequestContext) {
  await page.setViewportSize({ width: 1920, height: 1080 });
  await page.emulateMedia({ colorScheme: "dark", reducedMotion: "reduce" });
  await request.post(`${mockApiBaseUrl}/__reset`, {
    data: { scenario: "attack-map-dense" },
  });
  await page.goto("/attacks");

  const panel = page.getByTestId("attack-map-panel");
  const globe = page.getByTestId("cyber-attack-map");
  await expect(panel).toBeVisible();
  await expect(globe).toBeVisible();
  return { globe, panel };
}

async function featureAttack(
  globe: Locator,
  attackId: string,
  options: FeatureAttackOptions = {},
) {
  await globe.evaluate((element, detail) => {
    element.dispatchEvent(
      new CustomEvent("ad-platform:feature-attack", {
        detail,
      }),
    );
  }, { animate: false, ...options, attackId });
  await expect(
    globe.locator(`[data-attack-arc="true"][data-attack-id="${attackId}"][data-featured="true"]`),
  ).toHaveCount(1);
  await expect(globe).toHaveAttribute("data-featured-attack-id", attackId);
  if (options.rotation) {
    await expect(globe).toHaveAttribute(
      "data-rotation-lat",
      options.rotation.lat.toFixed(2),
    );
    await expect(globe).toHaveAttribute(
      "data-rotation-lon",
      options.rotation.lon.toFixed(2),
    );
  }
}

async function getLongestVisibleAttackId(globe: Locator, partial: boolean) {
  return globe.evaluate((element, wantsPartial) => {
    const arcs = Array.from(
      element.querySelectorAll<SVGPathElement>(
        `[data-attack-arc="true"][data-partial="${wantsPartial ? "true" : "false"}"]`,
      ),
    );
    const longestArc = arcs
      .map((arc) => ({
        id: arc.getAttribute("data-attack-id"),
        length: arc.getTotalLength(),
      }))
      .filter((arc): arc is { id: string; length: number } => arc.id !== null)
      .sort((left, right) => right.length - left.length)[0];
    return longestArc?.id ?? null;
  }, partial);
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

test("participant dense attack globe keeps its visual baseline", async ({
  page,
  request,
}) => {
  const { panel } = await loadDenseAttackMap(page, request);
  await stabilize(page, panel);
  await expect(panel).toHaveScreenshot(
    "participant-dense-attack-globe-dark.png",
  );
});

test("participant dense attack globe keeps featured route visual baseline", async ({
  page,
  request,
}) => {
  const { globe, panel } = await loadDenseAttackMap(page, request);
  await featureAttack(globe, "map-seed-24", {
    rotation: { lat: -4, lon: 94 },
  });

  await stabilize(page, panel);
  await expect(panel).toHaveScreenshot(
    "participant-dense-attack-globe-featured-dark.png",
  );
});

test("participant dense attack globe keeps partial featured route visual baseline", async ({
  page,
  request,
}) => {
  const { globe, panel } = await loadDenseAttackMap(page, request);
  const attackId = await getLongestVisibleAttackId(globe, true);
  expect(attackId).toBeTruthy();
  await featureAttack(globe, attackId!);

  await stabilize(page, panel);
  await expect(panel).toHaveScreenshot(
    "participant-dense-attack-globe-partial-featured-dark.png",
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
