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

  if ("missing" in overflow.arc) {
    throw new Error("featured route is missing");
  }
  expect(overflow.arc.left).toBeLessThanOrEqual(1);
  expect(overflow.arc.right).toBeLessThanOrEqual(1);
  expect(overflow.arc.top).toBeLessThanOrEqual(1);
  expect(overflow.arc.bottom).toBeLessThanOrEqual(1);
  expect(overflow.labels).toEqual([]);
}

async function featureAttack(globe: Locator, attackId: string) {
  await globe.scrollIntoViewIfNeeded();
  await expect(globe).toHaveAttribute('data-surface-active', 'true');
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

  await globe.scrollIntoViewIfNeeded();
  const featuredArc = globe.locator('[data-attack-arc="true"][data-featured="true"]');
  await expect(featuredArc).toHaveCount(1, { timeout: 12_500 });
  await expect(panel.getByText("Featured transmission")).toBeVisible();

  const dashPattern = await featuredArc.first().getAttribute("stroke-dasharray");
  expect(dashPattern).toBeNull();
});

test("neubrutalism keeps dense globe controls usable on mobile", async ({ page, request }) => {
  const appearance = await request.put(`${mockApiBaseUrl}/api/v2/admin/platform/appearance`, {
    data: { theme_id: "neubrutalism" },
  });
  expect(appearance.ok()).toBeTruthy();
  await page.setViewportSize({ width: 320, height: 660 });
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.goto("/attacks/map");

  await expect(page.locator("html")).toHaveAttribute("data-scheme", "neubrutalism");
  const globe = page.getByTestId("cyber-attack-map");
  await expect(globe).toHaveAttribute("data-keyboard-mode", "jump");
  for (const [region, longitude] of [["Americas", "-82.00"], ["APAC", "118.00"], ["EMEA", "34.00"]]) {
    await globe.getByRole("button", { name: region, exact: true }).click();
    await expect(globe).toHaveAttribute("data-rotation-lon", longitude);
  }

  await globe.getByRole("button", { name: "Rotate up" }).press("Enter");
  await expect(globe).toHaveAttribute("data-rotation-lat", "6.00");
  await globe.getByRole("button", { name: "Rotate right" }).press("Space");
  await expect(globe).toHaveAttribute("data-rotation-lon", "22.00");
  await globe.getByRole("combobox", { name: "Jump to team" }).selectOption({ index: 1 });
  await expect(page.getByText("Clear team focus")).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(320);
});

test("pausing the automatic camera holds its view through manual input, maximize, and reload", async ({ page }) => {
  await page.clock.install();
  await page.goto("/attacks/map");
  const globe = page.getByRole("region", { name: "Interactive attack globe" });
  const autoCamera = globe.getByRole("button", { name: "Auto camera", pressed: true });
  await globe.locator('[data-fx="map-canvas"]').scrollIntoViewIfNeeded();
  await expect(globe).toHaveAttribute("data-surface-active", "true");
  await page.clock.fastForward(11_000);
  await expect(globe).toHaveAttribute("data-featured-attack-id", /.+/);

  await autoCamera.press("Enter");
  await expect(globe).toHaveAttribute("data-auto-camera", "paused");
  const longitude = await globe.getAttribute("data-rotation-lon");
  const featured = await globe.getAttribute("data-featured-attack-id");
  await page.clock.fastForward(20_000);
  await expect(globe).toHaveAttribute("data-rotation-lon", longitude!);
  await expect(globe).toHaveAttribute("data-featured-attack-id", featured!);

  await globe.getByRole("button", { name: "Rotate right" }).press("Enter");
  await expect(globe).not.toHaveAttribute("data-rotation-lon", longitude!);
  const manualLongitude = await globe.getAttribute("data-rotation-lon");
  await page.clock.fastForward(20_000);
  await expect(globe).toHaveAttribute("data-rotation-lon", manualLongitude!);

  await page.getByRole("button", { name: "Maximize" }).press("Enter");
  const dialog = page.getByTestId("attack-map-maximize-dialog");
  await expect(dialog.getByRole("button", { name: "Auto camera", pressed: false })).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(dialog).toBeHidden();
  await page.reload();
  await expect(globe.getByRole("button", { name: "Auto camera", pressed: false })).toBeVisible();
});

test("reduced motion starts with a stationary camera and allows an explicit resume", async ({ page }) => {
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.clock.install();
  await page.goto("/attacks/map");
  const globe = page.getByRole("region", { name: "Interactive attack globe" });
  const autoCamera = globe.getByRole("button", { name: "Auto camera" });
  const canvas = globe.locator('[data-fx="map-canvas"]');
  await expect(autoCamera).toHaveAttribute("aria-pressed", "false");
  await canvas.scrollIntoViewIfNeeded();
  await expect(globe).toHaveAttribute("data-surface-active", "true");
  await page.clock.fastForward(18_000);
  await expect(globe).toHaveAttribute("data-rotation-lon", "34.00");
  await expect(globe).not.toHaveAttribute("data-featured-attack-id");

  await autoCamera.press("Enter");
  await expect(autoCamera).toHaveAttribute("aria-pressed", "true");
  await canvas.scrollIntoViewIfNeeded();
  await expect(globe).toHaveAttribute("data-surface-active", "true");
  await page.clock.fastForward(11_000);
  await expect(globe).toHaveAttribute("data-featured-attack-id", /.+/);
  await expect(globe).toHaveAttribute("data-motion-enabled", "false");
});

test("attack feed selection is exposed and retains its check in forced colors", async ({ page, request }) => {
  const appearance = await request.put(`${mockApiBaseUrl}/api/v2/admin/platform/appearance`, {
    data: { theme_id: "neubrutalism" },
  });
  expect(appearance.ok()).toBeTruthy();
  await page.emulateMedia({ forcedColors: "active", reducedMotion: "reduce" });
  await page.goto("/attacks/map");
  const group = page.getByRole("group", { name: "View", exact: true });
  for (const label of ["Recent", "Current Tick", "All Feed"]) {
    const option = group.getByRole("button", { name: label, exact: true });
    await option.press("Enter");
    await page.getByRole("textbox", { name: "Focus team" }).focus();
    await expect(option).toHaveAttribute("aria-pressed", "true");
    await expect(option.locator(".lucide-check")).toBeVisible();
    await expect(group.getByRole("button", { pressed: true })).toHaveCount(1);
    await expect(group.getByRole("button", { pressed: false }).locator(".lucide-check:visible")).toHaveCount(0);
  }
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
  await expect(globe).toBeVisible();
  await globe.scrollIntoViewIfNeeded();
  await expect
    .poll(async () => globe.locator("[data-attack-arc-hit]").count())
    .toBeGreaterThan(0);
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

  await page.getByRole("button", { name: "Maximize" }).click();
  await expect(page.getByTestId("attack-map-maximize-dialog")).toBeVisible();
});

test("seeded dense attack globe uses maximize without presentation mode", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1365, height: 768 });
  await page.goto("/attacks");

  await expect(page.getByRole("button", { name: "Present" })).toHaveCount(0);
  await page.getByRole("button", { name: "Maximize" }).click();
  const dialog = page.getByTestId("attack-map-maximize-dialog");
  await expect(dialog).toBeVisible();
  await expect(dialog.getByText("24 attacks")).toBeVisible();
  const globe = dialog.getByTestId("cyber-attack-map");
  await expect(globe).toBeVisible();
  await expect(dialog.getByText("Focus team")).toHaveCount(0);
  const inspector = dialog.getByText("Map overview");
  await expect(inspector).toBeVisible();

  const viewport = page.viewportSize();
  const box = await dialog.boundingBox();
  expect(viewport).not.toBeNull();
  expect(box).not.toBeNull();
  if (viewport && box) {
    expect(box.x).toBeGreaterThanOrEqual(0);
    expect(box.y).toBeGreaterThanOrEqual(0);
    expect(box.x + box.width).toBeLessThanOrEqual(viewport.width);
    expect(box.y + box.height).toBeLessThanOrEqual(viewport.height);
  }
});
