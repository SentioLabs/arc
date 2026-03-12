import {
	test,
	expect,
	uniqueName,
	createTestLabel,
	deleteTestLabel,
	createTestIssue,
	addLabelToIssue
} from './fixtures';

test.describe('Labels E2E', () => {
	test('labels page loads', async ({ page }) => {
		await page.goto('/labels');
		await expect(page.getByRole('button', { name: 'New Label' })).toBeVisible();
	});

	test('create label via UI', async ({ page }) => {
		const labelName = uniqueName('label');
		await page.goto('/labels');

		// Click "New Label" to show the form
		await page.getByRole('button', { name: 'New Label' }).click();

		// Fill in the name
		await page.locator('#label-name').fill(labelName);

		// Click "Create"
		await page.getByRole('button', { name: 'Create' }).click();

		// Verify label appears in the grid
		await expect(page.getByText(labelName)).toBeVisible();

		// Cleanup
		await deleteTestLabel(labelName);
	});

	test('edit label description', async ({ page }) => {
		const label = await createTestLabel();
		await page.goto('/labels');

		// Wait for label card to appear
		await expect(page.getByText(label.name)).toBeVisible();

		// Hover over the card to reveal edit button, then click pencil
		const card = page.locator('.card').filter({ hasText: label.name });
		await card.hover();
		await card.getByTitle('Edit').click();

		// Fill in description
		const desc = 'Updated description for testing';
		await page.locator('#label-desc').fill(desc);

		// Click "Update"
		await page.getByRole('button', { name: 'Update' }).click();

		// Verify description is shown on the card
		await expect(card.getByText(desc)).toBeVisible();
	});

	test('delete label', async ({ page }) => {
		const label = await createTestLabel();
		await page.goto('/labels');

		// Wait for label to appear
		await expect(page.getByText(label.name)).toBeVisible();

		// Hover card and click delete (trash) button
		const card = page.locator('.card').filter({ hasText: label.name });
		await card.hover();
		await card.getByTitle('Delete').click();

		// Confirm in dialog
		const dialog = page.getByRole('dialog');
		await dialog.getByRole('button', { name: 'Delete' }).click();

		// Verify label card is gone from the grid
		await expect(page.locator('.card').filter({ hasText: label.name })).not.toBeVisible();
	});

	test('add label to issue', async ({ page, testWorkspace }) => {
		const label = await createTestLabel();
		const issue = await createTestIssue(testWorkspace.id);

		// Navigate to the issue detail page
		await page.goto(`/${testWorkspace.id}/issues/${issue.id}`);

		// Wait for issue to load (issue ID appears in a mono span)
		await expect(page.locator('.font-mono').filter({ hasText: issue.id })).toBeVisible();

		// Click "+ Add" to open label picker dropdown
		await page.getByText('+ Add').click();

		// Search for the label
		await page.getByPlaceholder('Search labels...').fill(label.name);

		// Click the label option
		await page.locator('li[role="option"]').filter({ hasText: label.name }).click();

		// Verify label pill appears
		await expect(page.getByText(label.name)).toBeVisible();
	});

	test('remove label from issue', async ({ page, testWorkspace }) => {
		const label = await createTestLabel();
		const issue = await createTestIssue(testWorkspace.id);
		await addLabelToIssue(testWorkspace.id, issue.id, label.name as string);

		// Navigate to the issue detail page
		await page.goto(`/${testWorkspace.id}/issues/${issue.id}`);

		// Wait for issue and label pill to load
		await expect(page.locator('.font-mono').filter({ hasText: issue.id })).toBeVisible();
		await expect(page.getByLabel(`Remove label ${label.name}`)).toBeVisible();

		// Click the x button on the label pill
		await page.getByLabel(`Remove label ${label.name}`).click();

		// Verify label pill is removed
		await expect(page.getByLabel(`Remove label ${label.name}`)).not.toBeVisible();
	});

	test('label visible in issue list', async ({ page, testWorkspace }) => {
		const label = await createTestLabel();
		const issue = await createTestIssue(testWorkspace.id);
		await addLabelToIssue(testWorkspace.id, issue.id, label.name as string);

		// Navigate to the workspace issues list
		await page.goto(`/${testWorkspace.id}/issues`);

		// Wait for issues to load
		await expect(page.getByText(issue.title)).toBeVisible();

		// Verify label pill is visible on the issue card
		await expect(page.getByText(label.name)).toBeVisible();
	});
});
