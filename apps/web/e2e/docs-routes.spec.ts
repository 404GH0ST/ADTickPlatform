import { expect, test } from "@playwright/test";

test.use({ viewport: { width: 1280, height: 900 } });

test("participant manual links to Swagger and raw OpenAPI YAML", async ({
  page,
}) => {
  await page.goto("/docs/participant");

  await expect(
    page.locator("h1", { hasText: "Participant Manual" }),
  ).toBeVisible();
  await expect(page.getByText("Participant Flow")).toBeVisible();
  await expect(page.getByText("Participant Endpoints")).toBeVisible();
  await expect(
    page.getByRole("link", { name: "Open Swagger" }),
  ).toHaveAttribute("href", "/docs/platform-api");
  await expect(
    page.getByRole("link", { name: "Open Raw OpenAPI YAML" }),
  ).toHaveAttribute("href", "/docs/platform-api-v2.openapi.yaml");
});

test("participant API page exposes the raw YAML route and manual link", async ({
  page,
}) => {
  await page.goto("/docs/platform-api");

  await expect(
    page.locator("h1", { hasText: "Participant API" }),
  ).toBeVisible();
  await expect(page.getByText("Swagger Explorer")).toBeVisible();
  await expect(
    page.getByRole("link", { name: "Open Raw YAML" }),
  ).toHaveAttribute("href", "/docs/platform-api-v2.openapi.yaml");
  await expect(
    page.getByRole("link", { name: "Open Manual" }),
  ).toHaveAttribute("href", "/docs/participant");
  await expect(page.getByText("This page covers the participant API only.")).toBeVisible();
});

test("participant API page shows a fallback warning when Swagger CDN assets are blocked", async ({
  page,
}) => {
  await page.route("https://unpkg.com/swagger-ui-dist@5/**", (route) =>
    route.abort(),
  );

  await page.goto("/docs/platform-api");

  await expect(
    page.getByText("Swagger UI assets could not be loaded from the CDN."),
  ).toBeVisible();
  await expect(
    page.getByRole("link", { name: "Open Raw YAML" }),
  ).toHaveAttribute("href", "/docs/platform-api-v2.openapi.yaml");
});

test("raw OpenAPI YAML route serves the participant spec", async ({
  page,
  request,
}) => {
  const response = await request.get("/docs/platform-api-v2.openapi.yaml");
  expect(response.ok()).toBeTruthy();
  expect(response.headers()["content-type"]).toContain("application/yaml");
  const body = await response.text();
  expect(body).toContain("openapi:");
  expect(body).toContain("/api/v2/authenticate");
  expect(body).toContain("/api/v2/services/{challenge_id}/unlock");
});

test("legacy platform manual route redirects to the participant manual", async ({
  page,
}) => {
  await page.goto("/docs/platform-manual.md");

  await expect(page).toHaveURL(/\/docs\/participant$/);
  await expect(
    page.locator("h1", { hasText: "Participant Manual" }),
  ).toBeVisible();
});
