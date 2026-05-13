import { expect, test } from "@playwright/test";

import { mockApiBaseUrl } from "./test-utils";

test.use({ viewport: { width: 1440, height: 900 } });

test.beforeEach(async ({ page, request }) => {
  await request.post(`${mockApiBaseUrl}/__reset`, {
    data: { scenario: "attack-map-dense" },
  });
  await page.emulateMedia({ colorScheme: "dark" });
  await page.addInitScript(() => {
    window.localStorage.removeItem("ad-platform-theme");
  });
});

test("seeded dense attack globe renders deterministic landmark coverage", async ({
  page,
}) => {
  await page.goto("/attacks");

  const panel = page.getByTestId("attack-map-panel");
  await expect(panel).toBeVisible();
  await expect(panel.getByText("24 attacks")).toBeVisible();
  await expect(panel.getByText("18 teams")).toBeVisible();
  await expect(panel.getByText("3 services")).toBeVisible();

  const globe = page.getByTestId("cyber-attack-map");
  await expect(globe).toBeVisible();
  await expect.poll(
    async () => globe.locator('[data-attack-arc="true"]').count(),
  ).toBeGreaterThanOrEqual(20);
  await expect(
    panel.getByText("No accepted attacks in the current slice to plot."),
  ).toHaveCount(0);
  await expect(globe.locator('[data-attack-count="2"]')).toHaveCount(1);
});

test("seeded dense attack globe keeps idle featured routes solid", async ({
  page,
}) => {
  await page.goto("/attacks");

  const panel = page.getByTestId("attack-map-panel");
  const globe = page.getByTestId("cyber-attack-map");
  await expect(globe).toBeVisible();
  await expect(globe.locator('[data-attack-arc="true"]').first()).toBeVisible();

  const featuredArc = globe.locator('[data-attack-arc="true"][data-featured="true"]');
  await expect(featuredArc).toHaveCount(1, { timeout: 12_500 });
  await expect(panel.getByText("Featured transmission")).toBeVisible();

  const dashPattern = await featuredArc.first().getAttribute("stroke-dasharray");
  expect(dashPattern).toBeNull();
});

test("seeded dense attack globe keeps partial featured routes solid", async ({
  page,
}) => {
  await page.goto("/attacks");

  const globe = page.getByTestId("cyber-attack-map");
  await expect(globe).toBeVisible();
  const partialArc = globe.locator(
    '[data-attack-arc="true"][data-partial="true"]',
  ).first();
  await expect(partialArc).toBeVisible();

  const attackId = await partialArc.getAttribute("data-attack-id");
  expect(attackId).toBeTruthy();

  await globe.evaluate((element, selectedAttackId) => {
    element.dispatchEvent(
      new CustomEvent("ad-platform:feature-attack", {
        detail: { animate: false, attackId: selectedAttackId },
      }),
    );
  }, attackId);

  const featuredPartialArc = globe.locator(
    `[data-attack-arc="true"][data-attack-id="${attackId}"][data-featured="true"][data-partial="true"]`,
  );
  await expect(featuredPartialArc).toHaveCount(1);
  await expect(featuredPartialArc).toBeVisible();
  await expect(featuredPartialArc).not.toHaveAttribute("stroke-dasharray", /.+/);
});

test("seeded dense attack globe supports keyboard route inspection", async ({
  page,
}) => {
  await page.goto("/attacks");

  const panel = page.getByTestId("attack-map-panel");
  const globe = page.getByTestId("cyber-attack-map");
  const routeButton = globe.locator('[role="button"][aria-label^="Inspect attack route"]').first();

  await expect(routeButton).toBeVisible();
  await routeButton.focus();
  await page.keyboard.press("Enter");

  await expect(panel.getByText("Clear attack focus")).toBeVisible();
});

test("seeded dense attack globe supports pointer route inspection", async ({
  page,
}) => {
  await page.goto("/attacks");

  const panel = page.getByTestId("attack-map-panel");
  const globe = page.getByTestId("cyber-attack-map");
  const clickPoint = await globe.locator("[data-attack-arc-hit]").evaluateAll((paths) => {
    for (const path of paths) {
      if (!(path instanceof SVGPathElement)) {
        continue;
      }
      const matrix = path.getScreenCTM();
      if (!matrix) {
        continue;
      }
      const totalLength = path.getTotalLength();
      for (const fraction of [0.5, 0.35, 0.65, 0.2, 0.8]) {
        const point = path.getPointAtLength(totalLength * fraction).matrixTransform(matrix);
        const hit = document.elementFromPoint(point.x, point.y);
        if (hit?.closest("[data-attack-arc-hit]") === path) {
          return { x: point.x, y: point.y };
        }
      }
    }
    return null;
  });

  expect(clickPoint).not.toBeNull();
  if (!clickPoint) {
    return;
  }
  await page.mouse.click(clickPoint.x, clickPoint.y);

  await expect(panel.getByText("Clear attack focus")).toBeVisible();
});

test("seeded dense attack globe releases pointer capture after drag", async ({
  page,
}) => {
  await page.goto("/attacks");

  const globe = page.getByTestId("cyber-attack-map");
  await expect(globe).toBeVisible();
  const box = await globe.boundingBox();
  expect(box).not.toBeNull();
  if (!box) {
    return;
  }

  await page.mouse.move(box.x + box.width * 0.5, box.y + box.height * 0.5);
  await page.mouse.down();
  await page.mouse.move(box.x + box.width + 180, box.y + box.height + 120, { steps: 5 });
  await page.mouse.up();

  await page.getByRole("button", { name: "Present" }).click();
  await expect(page.getByTestId("attack-map-audience-dialog")).toBeVisible();
});

test("seeded dense attack globe has audience mode for presentation screens", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1365, height: 768 });
  await page.goto("/attacks");

  await page.getByRole("button", { name: "Present" }).click();
  const dialog = page.getByTestId("attack-map-audience-dialog");
  await expect(dialog).toBeVisible();
  await expect(dialog.getByText("24 attacks")).toBeVisible();
  const globe = dialog.getByTestId("cyber-attack-map");
  await expect(globe).toBeVisible();
  await expect(dialog.getByText("Focus team")).toHaveCount(0);

  await globe.evaluate((element) => {
    element.dispatchEvent(
      new CustomEvent("ad-platform:feature-attack", {
        detail: {
          animate: false,
          attackId: "map-seed-01",
          rotation: { lat: -2, lon: -24 },
        },
      }),
    );
  });
  const inspector = page.getByTestId("attack-map-presentation-inspector");
  await expect(inspector).toBeVisible();

  const viewport = page.viewportSize();
  const box = await inspector.boundingBox();
  expect(viewport).not.toBeNull();
  expect(box).not.toBeNull();
  if (viewport && box) {
    expect(box.x).toBeGreaterThanOrEqual(0);
    expect(box.y).toBeGreaterThanOrEqual(0);
    expect(box.x + box.width).toBeLessThanOrEqual(viewport.width);
    expect(box.y + box.height).toBeLessThanOrEqual(viewport.height);
  }
});
