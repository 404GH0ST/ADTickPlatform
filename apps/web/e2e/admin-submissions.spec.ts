import { expect, test } from '@playwright/test';
import { loginAsParticipant, mockApiBaseUrl } from './test-utils';

test.beforeEach(async ({ request }) => {
  await request.post(`${mockApiBaseUrl}/__reset`, {
    data: { scenario: 'submissions' },
  });
});

test('organizer can browse all verdicts, filter, and page submission history', async ({
  page,
}) => {
  await page.goto('/admin/submissions');
  await expect(
    page.getByRole('link', { name: 'Submissions', exact: true }),
  ).toBeVisible();
  await expect(page.getByText('Showing 1–25 of 30')).toBeVisible();
  await expect(page.getByTestId('submission-30')).toContainText('First blood');
  await expect(page.getByTestId('submission-29')).toContainText('Rejected');
  await expect(page.getByTestId('submission-28')).toContainText('Duplicate');
  await page.screenshot({ path: 'test-results/admin-submissions-desktop.png' });
  await page.getByRole('link', { name: 'Next', exact: true }).click();
  await expect(page.getByText('Showing 26–30 of 30')).toBeVisible();
  await page.getByRole('link', { name: 'Reset', exact: true }).click();
  await page.getByLabel('Verdict', { exact: true }).selectOption('invalid');
  await page.getByRole('button', { name: 'Apply filters' }).click();
  await expect(page).toHaveURL(/status=invalid/);
  await expect(page.getByTestId('submissions-table')).not.toContainText(
    'First blood',
  );
  await expect(
    page
      .getByTestId('submissions-table')
      .getByText('Accepted', { exact: true }),
  ).toHaveCount(0);
});

test('deleting first blood requires confirmation and reports the reopened award', async ({
  page,
}) => {
  await page.goto('/admin/submissions');
  await page
    .getByRole('button', { name: 'Delete submission #30', exact: true })
    .click();
  const dialog = page.getByRole('dialog');
  await expect(dialog).toContainText('existing captures will not be promoted');
  await page.screenshot({ path: 'test-results/admin-first-blood-delete.png' });
  await dialog.getByRole('button', { name: 'Cancel', exact: true }).click();
  await expect(page.getByTestId('submission-30')).toBeVisible();
  await page
    .getByRole('button', { name: 'Delete submission #30', exact: true })
    .click();
  await dialog
    .getByRole('button', { name: 'Delete submission', exact: true })
    .click();
  await expect(page.getByTestId('submission-30')).toHaveCount(0);
  await expect(page.getByText('Showing 1–25 of 29')).toBeVisible();
  await expect(page.getByRole('status')).toContainText(
    'open for the next accepted capture',
  );
});

test('deleting a duplicate preserves the first-blood capture', async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.emulateMedia({ colorScheme: 'dark' });
  await page.goto('/admin/submissions');
  await page
    .getByRole('button', { name: 'Delete submission #28', exact: true })
    .click();
  const dialog = page.getByRole('dialog');
  await expect(dialog).toContainText(
    'It does not change scores or an existing capture',
  );
  await expect(dialog).not.toContainText('First blood on');
  await page.screenshot({
    path: 'test-results/admin-submission-delete-mobile.png',
  });
  await dialog
    .getByRole('button', { name: 'Delete submission', exact: true })
    .click();
  await expect(page.getByTestId('submission-28')).toHaveCount(0);
  await expect(page.getByTestId('submission-30')).toContainText('First blood');
});

test('the first-blood warning and actions are reachable in a short dialog', async ({ page, request }) => {
  const appearance = await request.put(`${mockApiBaseUrl}/api/v2/admin/platform/appearance`, {
    data: { theme_id: 'neubrutalism' },
  });
  expect(appearance.ok()).toBeTruthy();
  await page.setViewportSize({ width: 320, height: 320 });
  await page.goto('/admin/submissions');
  const opener = page.getByRole('button', { name: 'Delete submission #30', exact: true });
  await opener.press('Enter');

  const dialog = page.getByRole('dialog', { name: 'Delete submission #30?' });
  await expect(dialog.getByRole('heading')).toBeFocused();
  const warning = dialog.getByText(/existing captures will not be promoted/);
  await page.keyboard.press('PageDown');
  await expect(warning).toBeInViewport({ ratio: 0.5 });
  await page.keyboard.press('Tab');
  const cancel = dialog.getByRole('button', { name: 'Cancel', exact: true });
  await expect(cancel).toBeFocused();
  await expect(cancel).toBeInViewport();
  for (const label of ['Delete submission', 'Close', 'Cancel']) {
    await page.keyboard.press('Tab');
    const action = dialog.getByRole('button', { name: label, exact: true });
    await expect(action).toBeFocused();
    await expect(action).toBeInViewport();
  }
  await page.keyboard.press('Escape');
  await expect(dialog).toBeHidden();
  await expect(opener).toBeFocused();
});

test('failed deletion keeps the row and shows a retryable error', async ({
  page,
  request,
}) => {
  await request.post(`${mockApiBaseUrl}/__reset`, {
    data: { scenario: 'submissions-delete-failure' },
  });
  await page.goto('/admin/submissions');
  await page
    .getByRole('button', { name: 'Delete submission #30', exact: true })
    .click();
  await page
    .getByRole('dialog')
    .getByRole('button', { name: 'Delete submission', exact: true })
    .click();
  await expect(page.getByRole('dialog').getByRole('status')).toHaveCount(2);
  await expect(page.getByTestId('submission-30')).toHaveCount(1);
});

test('participant cannot read or delete organizer submissions', async ({
  page,
  context,
}) => {
  await loginAsParticipant(context);
  await page.goto('/admin/submissions');
  await expect(page).not.toHaveURL(/\/admin\/submissions/);
  expect((await page.request.get('/api/admin/submissions')).status()).toBe(403);
  expect(
    (await page.request.delete('/api/admin/submissions/30')).status(),
  ).toBe(403);
});
