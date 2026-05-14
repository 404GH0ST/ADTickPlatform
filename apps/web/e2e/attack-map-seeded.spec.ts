import { expect, test, type Locator, type Page } from "@playwright/test";

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

async function getAttackRouteClickPoint(globe: Locator) {
  return globe.locator("[data-attack-arc-hit]").evaluateAll((paths: Element[]) => {
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
}

async function expectVisibleTeamLabelsInsideViewport(
  page: Page,
  globe: Locator,
) {
  const viewport = page.viewportSize();
  expect(viewport).not.toBeNull();
  if (!viewport) {
    return;
  }

  const overflowedLabels = await globe
    .locator('[data-attack-team-label="true"]')
    .evaluateAll((labels: Element[], viewportSize: { width: number; height: number }) =>
      labels
        .map((label) => {
          const box = label.getBoundingClientRect();
          return {
            id: label.getAttribute("data-attack-team-id"),
            left: box.left,
            right: box.right,
            top: box.top,
            bottom: box.bottom,
          };
        })
        .filter((box) =>
          box.left < 0 ||
          box.top < 0 ||
          box.right > viewportSize.width ||
          box.bottom > viewportSize.height,
        ),
      viewport,
    );

  expect(overflowedLabels).toEqual([]);
}

async function expectFeaturedRouteInsideGlobe(globe: Locator) {
  const overflow = await globe.evaluate((element) => {
    const rootBox = element.getBoundingClientRect();
    const featuredArc = element.querySelector<SVGPathElement>(
      '[data-attack-arc="true"][data-featured="true"]',
    );
    const labels = Array.from(
      element.querySelectorAll<SVGGElement>('[data-attack-team-label="true"]'),
    );

    return {
      arc: featuredArc
        ? getBoxOverflow(featuredArc.getBoundingClientRect(), rootBox)
        : { missing: true },
      labels: labels
        .map((label) => ({
          id: label.getAttribute("data-attack-team-id"),
          ...getBoxOverflow(label.getBoundingClientRect(), rootBox),
        }))
        .filter((box) => box.left > 1 || box.right > 1 || box.top > 1 || box.bottom > 1),
    };

    function getBoxOverflow(box: DOMRect, bounds: DOMRect) {
      return {
        bottom: box.bottom - bounds.bottom,
        left: bounds.left - box.left,
        right: box.right - bounds.right,
        top: bounds.top - box.top,
      };
    }
  });

  expect(overflow.arc).not.toHaveProperty("missing");
  expect(overflow.arc.left).toBeLessThanOrEqual(1);
  expect(overflow.arc.right).toBeLessThanOrEqual(1);
  expect(overflow.arc.top).toBeLessThanOrEqual(1);
  expect(overflow.arc.bottom).toBeLessThanOrEqual(1);
  expect(overflow.labels).toEqual([]);
}

async function featureAttack(globe: Locator, attackId: string) {
  await globe.evaluate((element, selectedAttackId) => {
    element.dispatchEvent(
      new CustomEvent("ad-platform:feature-attack", {
        detail: { animate: false, attackId: selectedAttackId },
      }),
    );
  }, attackId);
  await expect(globe).toHaveAttribute("data-featured-attack-id", attackId);
  await expect(globe.locator('[data-attack-arc="true"][data-featured="true"]')).toHaveCount(1);
}

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

  await featureAttack(globe, attackId!);

  const featuredPartialArc = globe.locator(
    `[data-attack-arc="true"][data-attack-id="${attackId}"][data-featured="true"][data-partial="true"]`,
  );
  await expect(featuredPartialArc).toHaveCount(1);
  await expect(featuredPartialArc).toBeVisible();
  await expect(featuredPartialArc).not.toHaveAttribute("stroke-dasharray", /.+/);
});

test("seeded dense attack globe keeps every featured route away from viewport edges", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1365, height: 768 });
  await page.goto("/attacks");

  const globe = page.getByTestId("cyber-attack-map");
  await expect(globe).toBeVisible();

  for (let index = 1; index <= 24; index += 1) {
    const attackId = `map-seed-${String(index).padStart(2, "0")}`;
    await featureAttack(globe, attackId);
    await expectFeaturedRouteInsideGlobe(globe);
  }
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
  const clickPoint = await getAttackRouteClickPoint(globe);

  expect(clickPoint).not.toBeNull();
  if (!clickPoint) {
    return;
  }
  await page.mouse.click(clickPoint.x, clickPoint.y);

  await expect(panel.getByText("Clear attack focus")).toBeVisible();
});

test("seeded dense attack globe keeps labels bounded and selected routes visible after drag", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1365, height: 768 });
  await page.goto("/attacks");

  const panel = page.getByTestId("attack-map-panel");
  const globe = page.getByTestId("cyber-attack-map");
  await expect(globe).toBeVisible();

  await globe.locator('[data-attack-team-hit="true"]').first().click();
  await expect(panel.getByText("Clear team focus")).toBeVisible();
  await expectVisibleTeamLabelsInsideViewport(page, globe);

  const clickPoint = await getAttackRouteClickPoint(globe);
  expect(clickPoint).not.toBeNull();
  if (!clickPoint) {
    return;
  }
  await page.mouse.click(clickPoint.x, clickPoint.y);
  await expect(panel.getByText("Clear attack focus")).toBeVisible();
  await expect(globe.locator('[data-attack-arc="true"][data-selected="true"]')).toHaveCount(1);

  const box = await globe.boundingBox();
  expect(box).not.toBeNull();
  if (!box) {
    return;
  }
  await page.mouse.move(box.x + box.width * 0.5, box.y + box.height * 0.5);
  await page.mouse.down();
  await page.mouse.move(box.x + box.width * 0.72, box.y + box.height * 0.42, { steps: 8 });
  await page.mouse.up();

  await expect(globe.locator('[data-attack-arc="true"][data-selected="true"]')).toBeVisible();
  await expectVisibleTeamLabelsInsideViewport(page, globe);
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
