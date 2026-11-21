import { expect, test, type APIRequestContext, type Page } from "@playwright/test";

import { adminTest, mockApiBaseUrl, participantStorageState, resetMockApi } from "./test-utils";

/*
 * Challenge category and connection protocol: the organizer sets them, and
 * every participant service card shows the category icon and label plus a
 * connection line that is a URL for HTTP and an nc command for TCP.
 */

async function useTheme(request: APIRequestContext, themeID: string) {
  const response = await request.put(`${mockApiBaseUrl}/api/v2/admin/platform/appearance`, {
    data: { theme_id: themeID },
  });
  expect(response.ok()).toBeTruthy();
}

async function setChallengeMetadata(request: APIRequestContext, category: string, protocol: string) {
  const response = await request.put(`${mockApiBaseUrl}/api/v2/admin/challenges/1`, {
    data: { name: "college-http", category, protocol },
  });
  expect(response.ok()).toBeTruthy();
}

async function expectHttpCard(page: Page) {
  await expect(page.getByTestId("service-category-svc-1")).toHaveText("Web exploitation");
  const connection = page.getByTestId("service-connection-svc-1");
  await expect(connection).toHaveAttribute("data-protocol", "http");
  await expect(connection.getByText("HTTP", { exact: true })).toBeVisible();
  const link = connection.getByRole("link", { name: /http:\/\/10\.80\.50\.11:30050\// });
  await expect(link).toHaveAttribute("href", "http://10.80.50.11:30050/");
  await expect(link).toHaveAttribute("target", "_blank");
  await connection.getByRole("button", { name: "Copy URL" }).click();
  await expect(connection.getByRole("button", { name: "Copied" })).toBeVisible();
}

async function expectTcpCard(page: Page) {
  await expect(page.getByTestId("service-category-svc-1")).toHaveText("Pwn");
  const connection = page.getByTestId("service-connection-svc-1");
  await expect(connection).toHaveAttribute("data-protocol", "tcp");
  await expect(connection.getByText("TCP", { exact: true })).toBeVisible();
  await expect(connection.getByText("nc 10.80.50.11 30050", { exact: true })).toBeVisible();
  await expect(connection.getByRole("link")).toHaveCount(0);
  await expect(connection.getByRole("button", { name: "Copy command" })).toBeVisible();
}

test.describe("participant service cards", () => {
  test.use({ storageState: participantStorageState });

  for (const theme of ["graphite", "neubrutalism"]) {
    test(`${theme}: category and connection follow the organizer's settings`, async ({ page, request, context }) => {
      await context.grantPermissions(["clipboard-read", "clipboard-write"]);
      await resetMockApi(request);
      await request.post(`${mockApiBaseUrl}/api/v2/admin/game/match/start`);
      await useTheme(request, theme);

      await page.goto("/services");
      await expectHttpCard(page);
      expect(await page.evaluate(() => navigator.clipboard.readText())).toBe("http://10.80.50.11:30050/");

      await setChallengeMetadata(request, "pwn", "tcp");
      await page.reload();
      await expectTcpCard(page);
      await page.getByTestId("service-connection-svc-1").getByRole("button", { name: "Copy command" }).click();
      expect(await page.evaluate(() => navigator.clipboard.readText())).toBe("nc 10.80.50.11 30050");
    });
  }
});

adminTest("organizer picks a category and connection and sees the participant preview", async ({ page }) => {
  await page.goto("/admin/challenges");
  await expect(page.getByTestId("challenge-category-badge-1")).toHaveText("Web exploitation");
  await expect(page.getByTestId("challenge-protocol-badge-1")).toHaveText("HTTP");

  await page.getByRole("button", { name: "Create Challenge" }).click();
  const dialog = page.getByRole("dialog", { name: "Create challenge" });
  await dialog.getByLabel("Challenge name").fill("heap-notes");
  await dialog.getByLabel("Baseline image").fill("adplatform/heap-notes:baseline");
  await dialog.getByLabel("Checker image").fill("adplatform/heap-notes-checker:latest");
  await dialog.getByLabel("Service port").fill("31337");

  const createButton = dialog.getByRole("button", { name: "Create" });
  await expect(createButton).toBeDisabled();
  await dialog.getByLabel("Category").selectOption("pwn");

  // TCP is the default; the preview shows exactly what participants will get.
  const protocol = dialog.getByTestId("form-challenge-protocol");
  await expect(protocol.getByRole("radio", { name: /TCP/ })).toBeChecked();
  await expect(dialog.getByTestId("form-challenge-connection-example")).toHaveText(/^nc 10\.80\.\d+\.11 31337$/);
  await protocol.getByRole("radio", { name: /HTTP/ }).check();
  await expect(dialog.getByTestId("form-challenge-connection-example")).toHaveText(/^http:\/\/10\.80\.\d+\.11:31337\/$/);

  await createButton.click();
  await expect(page.getByTestId("challenge-category-badge-2")).toHaveText("Pwn");
  await expect(page.getByTestId("challenge-protocol-badge-2")).toHaveText("HTTP");

  // Editing keeps both, and a challenge may go back to uncategorized.
  await page.getByTestId("edit-challenge-2").click();
  const edit = page.getByRole("dialog", { name: "Edit challenge" });
  await expect(edit.getByLabel("Category")).toHaveValue("pwn");
  await expect(edit.getByTestId("form-challenge-protocol").getByRole("radio", { name: /HTTP/ })).toBeChecked();
  await edit.getByLabel("Category").selectOption("");
  await edit.getByRole("button", { name: "Update" }).click();
  await expect(page.getByTestId("challenge-category-badge-2")).toHaveText("Uncategorized");
  await expect(page.getByTestId("challenge-protocol-badge-2")).toHaveText("HTTP");
});
