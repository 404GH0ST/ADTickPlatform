import { expect, test, type Locator, type Page } from "@playwright/test";
import {
  mockApiBaseUrl,
  participantStorageState,
  resetMockApi,
} from "./test-utils";

test.use({
  viewport: { width: 1280, height: 900 },
  storageState: participantStorageState,
});

test.beforeEach(async ({ request }) => {
  await resetMockApi(request);
  await request.post(`${mockApiBaseUrl}/api/v2/admin/game/match/start`);
});

async function submit(page: Page, flags: string[]) {
  await page.goto("/services");
  await page
    .getByRole("textbox", { name: "Flags", exact: true })
    .fill(flags.join("\n"));
  await page.getByRole("button", { name: "Submit", exact: true }).click();
}

async function colors(locator: Locator) {
  return locator.evaluate((element) => {
    const style = getComputedStyle(element);
    return {
      background: style.backgroundColor,
      border: style.borderColor,
      text: style.color,
    };
  });
}

test("rejected flags are styled differently from accepted flags", async ({
  page,
}) => {
  await submit(page, [
    "PLAYIT{enemy-challenge-1}",
    "PLAYIT{already-submitted}",
    "PLAYIT{wrong}",
  ]);

  const verdicts = page.getByTestId("flag-verdict");
  await expect(verdicts).toHaveCount(3);
  const accepted = verdicts.nth(0);
  const duplicate = verdicts.nth(1);
  const rejected = verdicts.nth(2);

  await expect(accepted).toHaveAttribute("data-verdict", "accepted");
  await expect(accepted).toContainText("Accepted: PLAYIT{enemy-challenge-1}");
  await expect(duplicate).toHaveAttribute("data-verdict", "duplicate");
  await expect(duplicate).toContainText("flag already submitted.");
  await expect(rejected).toHaveAttribute("data-verdict", "rejected");
  await expect(rejected).toContainText("Rejected: PLAYIT{wrong}");
  await expect(rejected).toContainText("flag is wrong or expired.");

  const acceptedColors = await colors(accepted);
  const duplicateColors = await colors(duplicate);
  const rejectedColors = await colors(rejected);
  expect(rejectedColors.background).not.toBe(acceptedColors.background);
  expect(rejectedColors.text).not.toBe(acceptedColors.text);
  expect(duplicateColors.background).not.toBe(acceptedColors.background);
  expect(duplicateColors.background).not.toBe(rejectedColors.background);

  // Mixed batch: the summary is neither the success nor the error tone.
  const summary = page.getByTestId("flag-submit-summary");
  await expect(summary).toHaveAttribute("data-variant", "warning");
  await expect(summary).toContainText("Accepted 1, rejected 2.");
});

test("an all-rejected batch shows an error summary, not a success one", async ({
  page,
}) => {
  await submit(page, ["PLAYIT{wrong}", "PLAYIT{also-wrong}"]);

  await expect(page.getByTestId("flag-verdict")).toHaveCount(2);
  await expect(
    page.locator('[data-testid="flag-verdict"][data-verdict="rejected"]'),
  ).toHaveCount(2);
  const summary = page.getByTestId("flag-submit-summary");
  await expect(summary).toHaveAttribute("data-variant", "error");
  await expect(summary).toContainText("Accepted 0, rejected 2.");
  await expect(summary.locator(".tone-danger")).toBeVisible();
});

test("an all-accepted batch shows a success summary", async ({ page }) => {
  await submit(page, ["PLAYIT{enemy-challenge-1}"]);

  await expect(page.getByTestId("flag-verdict")).toHaveAttribute(
    "data-verdict",
    "accepted",
  );
  const summary = page.getByTestId("flag-submit-summary");
  await expect(summary).toHaveAttribute("data-variant", "success");
  await expect(summary.locator(".tone-success")).toBeVisible();
});
