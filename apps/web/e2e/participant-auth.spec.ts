import { expect, test } from "@playwright/test";

const mockApiBaseUrl = "http://127.0.0.1:4010";

test.use({
  viewport: { width: 1280, height: 900 },
  storageState: { cookies: [], origins: [] },
});

test.beforeEach(async ({ request }) => {
  await request.post(`${mockApiBaseUrl}/__reset`);
});

test("unauthenticated admin access redirects to the participant login page", async ({
  page,
}) => {
  await page.goto("/admin");

  await expect(page).toHaveURL(/\/login$/);
  await expect(page.getByText("Participant Sign In")).toBeVisible();
});

test("participant login shows an error for invalid credentials", async ({
  page,
}) => {
  await page.goto("/login");

  await page.getByLabel("Email").fill("alpha.captain@college.local");
  await page.getByLabel("Password").fill("wrong-password");
  await page.getByRole("button", { name: "Sign In" }).click();

  await expect(
    page.getByText("email or password is wrong."),
  ).toBeVisible();
  await expect(page).toHaveURL(/\/login$/);
});

test("participant can sign in and sign out through the session routes", async ({
  page,
}) => {
  await page.goto("/login");

  await page.getByLabel("Email").fill("alpha.captain@college.local");
  await page.getByLabel("Password").fill("alpha-password");
  await page.getByRole("button", { name: "Sign In" }).click();

  await expect(page).toHaveURL(/\/services$/);
  await expect(page.locator("h1", { hasText: "Services" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Sign Out" })).toBeVisible();

  await page.getByRole("button", { name: "Sign Out" }).click();

  await expect(page).toHaveURL(/\/$/);
  await expect(page.getByRole("link", { name: "Sign In" })).toBeVisible();
});

test("authenticated participants are redirected away from organizer routes and the login page", async ({
  page,
}) => {
  await page.goto("/login");

  await page.getByLabel("Email").fill("alpha.captain@college.local");
  await page.getByLabel("Password").fill("alpha-password");
  await page.getByRole("button", { name: "Sign In" }).click();

  await expect(page).toHaveURL(/\/services$/);
  await expect(page.locator("h1", { hasText: "Services" })).toBeVisible();

  await page.goto("/admin");

  await expect(page).toHaveURL(/\/$/);
  await expect(page.locator("h1", { hasText: "Participant" })).toBeVisible();

  await page.goto("/login");

  await expect(page).toHaveURL(/\/services$/);
  await expect(page.locator("h1", { hasText: "Services" })).toBeVisible();
});
