import { expect, test } from "@playwright/test";

const mockApiBaseUrl = "http://127.0.0.1:4010";

test.use({ viewport: { width: 1280, height: 900 } });

test.beforeEach(async ({ request }) => {
  await request.post(`${mockApiBaseUrl}/__reset`);
});

test("organizer challenge redeploy supersedes the older queued job", async ({
  page,
  request,
}) => {
  await request.post(`${mockApiBaseUrl}/__reset`, {
    data: { scenario: "redeploy" },
  });

  await page.goto("/admin/challenges");

  const challengeRow = page.getByTestId("challenge-row-1");
  await expect(challengeRow).toBeVisible();
  await expect(page.getByTestId("deploy-challenge-1")).toContainText(
    "Redeploy",
  );

  await page.getByTestId("deploy-challenge-1").click();

  await expect(
    page.getByText(
      "Queued college-http for 3 team runtimes. Reconcile to mark the rollout ready.",
    ),
  ).toBeVisible();
  await expect(challengeRow.getByText("valid", { exact: true })).toBeVisible();
  await expect(
    challengeRow.getByText("baseline ok / checker ok"),
  ).toBeVisible();
  await expect(challengeRow.getByText("ready 0 / queued 3")).toBeVisible();

  await page.goto("/admin/deployments");

  const newDeploymentRow = page.getByTestId("deployment-row-89");
  const oldDeploymentRow = page.getByTestId("deployment-row-88");

  await expect(newDeploymentRow).toBeVisible();
  await expect(newDeploymentRow.getByText("queued")).toBeVisible();
  await expect(newDeploymentRow.getByText("0/3")).toBeVisible();
  await expect(oldDeploymentRow).toBeVisible();
  await expect(oldDeploymentRow.getByText("superseded")).toBeVisible();
});

test("organizer can create, edit, and delete a challenge", async ({
  page,
}) => {
  await page.goto("/admin/challenges");

  await page.getByRole("button", { name: "Create Challenge" }).click();
  const createDialog = page.getByRole("dialog", { name: "Create challenge" });
  await expect(createDialog).toBeVisible();
  await createDialog.getByLabel("Challenge name").fill("college-ftp");
  await createDialog
    .getByLabel("Baseline image")
    .fill("adplatform/sample-ftp:baseline");
  await createDialog
    .getByLabel("Checker image")
    .fill("adplatform/sample-ftp-checker:latest");
  await createDialog.getByLabel("Service port").fill("30060");
  await createDialog.getByLabel("Subnet octet").fill("60");
  await createDialog.getByLabel("Weight").fill("7");
  await createDialog.getByRole("button", { name: "Create" }).click();

  await expect(
    page.getByText(
      "Created draft challenge college-ftp. Deploy it to replicate one service per team.",
    ),
  ).toBeVisible();

  const challengeRow = page.getByTestId("challenge-row-2");
  await expect(challengeRow).toBeVisible();
  await expect(challengeRow).toContainText("college-ftp");
  await expect(challengeRow).toContainText("draft");
  await expect(challengeRow).toContainText("7");
  await expect(challengeRow).toContainText("10.80.60.0/24");
  await expect(challengeRow).toContainText("port 30060");

  await page.getByTestId("edit-challenge-2").click();
  const challengeDialog = page.getByRole("dialog", { name: "Edit challenge" });
  await expect(challengeDialog).toBeVisible();
  await challengeDialog.getByLabel("Challenge name").fill("college-ftp-v2");
  await challengeDialog
    .getByLabel("Baseline image")
    .fill("adplatform/sample-ftp:patched");
  await challengeDialog
    .getByLabel("Checker image")
    .fill("adplatform/sample-ftp-checker:v2");
  await challengeDialog.getByLabel("Weight").fill("9");
  await challengeDialog.getByRole("button", { name: "Update" }).click();

  await expect(page.getByText("Updated challenge college-ftp-v2.")).toBeVisible();
  await expect(challengeRow).toContainText("college-ftp-v2");
  await expect(challengeRow).toContainText("adplatform/sample-ftp:patched");
  await expect(challengeRow).toContainText("adplatform/sample-ftp-checker:v2");
  await expect(challengeRow).toContainText("9");

  await page.getByTestId("delete-challenge-2").click();
  const deleteDialog = page.getByRole("dialog", { name: "Delete challenge" });
  await expect(deleteDialog).toBeVisible();
  await deleteDialog.getByRole("button", { name: "Delete" }).click();

  await expect(page.getByText('Challenge "college-ftp-v2" deleted.')).toBeVisible();
  await expect(page.getByTestId("challenge-row-2")).toHaveCount(0);
});
