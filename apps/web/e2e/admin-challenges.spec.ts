import { expect } from "@playwright/test";
import { adminTest as test, mockApiBaseUrl } from "./test-utils";


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
      "Queued college-http for 3 team runtimes. Run trusted reconcile to verify rollout, SSH access, and WireGuard state.",
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
  await expect(createDialog.getByRole("button", { name: "Create" })).toBeDisabled();
  await expect(createDialog.getByText("Challenge ID: #2")).toBeVisible();
  await expect(createDialog.getByText("Runtime network: 10.80.2.0/24")).toBeVisible();
  await expect(
    createDialog.getByText("Participant source download: disabled until a bundle path is set"),
  ).toBeVisible();

  await createDialog.getByLabel("Challenge name").fill("college-ftp");
  await createDialog
    .getByLabel("Baseline image")
    .fill("adplatform/sample-ftp:baseline");
  await createDialog
    .getByLabel("Checker image")
    .fill("adplatform/sample-ftp-checker:latest");
  await expect(createDialog.getByRole("button", { name: "Create" })).toBeEnabled();

  await createDialog.getByLabel("Source bundle path").fill("/bad");
  await expect(
    createDialog.getByText("Use a relative path inside AD_CHALLENGE_SOURCE_ROOT."),
  ).toBeVisible();
  await expect(createDialog.getByRole("button", { name: "Create" })).toBeDisabled();

  await createDialog
    .getByLabel("Source bundle path")
    .fill("examples/sample-lfi-challenge");
  await createDialog.getByLabel("Service port").fill("30060");
  await expect(createDialog.getByText("Example team endpoint: 10.80.2.11:30060")).toBeVisible();
  await createDialog.getByLabel("Subnet octet").fill("50");
  await expect(
    createDialog.getByText("Already assigned to college-http.", {
      exact: true,
    }),
  ).toBeVisible();
  await expect(createDialog.getByRole("button", { name: "Create" })).toBeDisabled();

  await createDialog.getByLabel("Subnet octet").fill("60");
  await createDialog.getByLabel("Weight").fill("7");
  await expect(createDialog.getByText("Runtime network: 10.80.60.0/24")).toBeVisible();
  await expect(createDialog.getByText("Example team endpoint: 10.80.60.11:30060")).toBeVisible();
  await expect(
    createDialog.getByText("Participant source download: examples/sample-lfi-challenge"),
  ).toBeVisible();
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
  await expect(
    deleteDialog.getByText(
      'Delete challenge "college-ftp-v2"? This cannot be undone from the dashboard.',
    ),
  ).toBeVisible();
  await expect(
    deleteDialog.getByText(
      "Associated service instances are removed from organizer and participant views.",
    ),
  ).toBeVisible();
  await deleteDialog.getByRole("button", { name: "Delete Challenge" }).click();

  await expect(page.getByText('Challenge "college-ftp-v2" deleted.')).toBeVisible();
  await expect(page.getByTestId("challenge-row-2")).toHaveCount(0);
});
