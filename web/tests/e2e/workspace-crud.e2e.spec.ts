import {
	test,
	expect,
	createTestWorkspace,
	deleteTestWorkspace,
	createTestIssue,
	updateTestIssue,
	uniqueName,
} from './fixtures';

test.describe('Workspace CRUD', () => {
	// Tests share server state, so run serially to avoid interference
	test.describe.configure({ mode: 'serial' });

	test('workspace appears in list', async ({ page, testWorkspace }) => {
		await page.goto('/');
		// Target the main content area to avoid sidebar duplicates
		const main = page.locator('main');
		await expect(main.getByRole('heading', { name: testWorkspace.name })).toBeVisible();
	});

	test('stats dashboard shows correct counts', async ({ page, testWorkspace }) => {
		// Create 2 issues: one open, one closed
		const issue1 = await createTestIssue(testWorkspace.id, { title: 'Open issue' });
		const issue2 = await createTestIssue(testWorkspace.id, { title: 'Closed issue' });
		await updateTestIssue(testWorkspace.id, issue2.id as string, { status: 'closed' });

		await page.goto(`/${testWorkspace.id}`);

		// Wait for stats to load
		await expect(page.getByText('Loading stats...')).not.toBeVisible({ timeout: 10000 });

		// Find stat cards by their label and verify values
		const statsGrid = page.locator('.grid').first();

		// Verify Total Issues shows 2
		const totalCard = statsGrid.locator('.card').filter({ hasText: 'Total Issues' });
		await expect(totalCard.locator('.text-2xl')).toHaveText('2');

		// Verify Open shows 1
		const openCard = statsGrid.locator('.card').filter({ has: page.getByText('Open', { exact: true }) });
		await expect(openCard.locator('.text-2xl')).toHaveText('1');

		// Verify Closed shows 1
		const closedCard = statsGrid.locator('.card').filter({ hasText: 'Closed' });
		await expect(closedCard.locator('.text-2xl')).toHaveText('1');
	});

	test('delete workspace via edit mode', async ({ page }) => {
		// Create an extra workspace specifically for deletion
		const extraWs = await createTestWorkspace(uniqueName('delete-me'));

		try {
			await page.goto('/');
			const main = page.locator('main');

			// Wait for the workspace card to appear (h3 inside cards)
			await expect(
				main.locator('h3').filter({ hasText: extraWs.name }),
			).toBeVisible();

			// Enter edit mode
			await main.getByRole('button', { name: 'Edit' }).click();

			// Select the extra workspace card by clicking it
			const extraCard = main.locator('button.card').filter({ hasText: extraWs.name });
			await extraCard.click();

			// Click the delete button in the batch actions bar (btn-danger class)
			await main.locator('button.btn-danger').click();

			// Confirm deletion in the dialog
			const dialog = page.locator('dialog');
			await expect(dialog).toBeVisible();
			await dialog.getByRole('button', { name: /Delete/ }).click();

			// Verify workspace is gone
			await expect(
				main.locator('h3').filter({ hasText: extraWs.name }),
			).not.toBeVisible();
		} catch (err) {
			// Cleanup on failure
			await deleteTestWorkspace(extraWs.id).catch(() => {});
			throw err;
		}
	});

	test('merge workspaces via UI', async ({ page, testWorkspace }) => {
		// Create a second workspace with an issue
		const ws2 = await createTestWorkspace(uniqueName('merge-src'));
		await createTestIssue(ws2.id, { title: 'Issue to merge' });

		try {
			await page.goto('/');
			const main = page.locator('main');
			await expect(
				main.locator('h3').filter({ hasText: ws2.name }),
			).toBeVisible();

			// Enter edit mode
			await main.getByRole('button', { name: 'Edit' }).click();

			// Select ws2 (the source to merge)
			const ws2Card = main.locator('button.card').filter({ hasText: ws2.name });
			await ws2Card.click();

			// Click "Merge into..."
			await main.getByRole('button', { name: 'Merge into...' }).click();

			// The merge dialog should appear (use [open] to target only the visible dialog)
			const dialog = page.locator('dialog.dialog-modal[open]');
			await expect(dialog).toBeVisible();

			// Open the Select dropdown and pick testWorkspace as target
			await dialog.locator('.select-trigger').click();
			await dialog.locator('.select-option', { hasText: testWorkspace.name }).click();

			// Confirm the merge
			await dialog.getByRole('button', { name: /Merge/ }).click();

			// Wait for success state
			await expect(dialog.getByText('Merge complete')).toBeVisible({ timeout: 10000 });

			// Close the success dialog
			await dialog.getByRole('button', { name: 'Done' }).click();

			// ws2 should be gone from the list
			await expect(
				main.locator('h3').filter({ hasText: ws2.name }),
			).not.toBeVisible();
		} catch (err) {
			await deleteTestWorkspace(ws2.id).catch(() => {});
			throw err;
		}
	});

	test('workspace search filters list', async ({ page, testWorkspace }) => {
		// Create another workspace so we have multiple
		const otherWs = await createTestWorkspace(uniqueName('other'));

		try {
			await page.goto('/');
			const main = page.locator('main');
			await expect(
				main.locator('h3').filter({ hasText: testWorkspace.name }),
			).toBeVisible();
			await expect(
				main.locator('h3').filter({ hasText: otherWs.name }),
			).toBeVisible();

			// Type the test workspace name in search (scope to main to avoid sidebar search)
			const searchInput = main.getByPlaceholder('Search workspaces...');
			await searchInput.fill(testWorkspace.name);

			// testWorkspace should be visible, other should not
			await expect(
				main.locator('h3').filter({ hasText: testWorkspace.name }),
			).toBeVisible();
			await expect(
				main.locator('h3').filter({ hasText: otherWs.name }),
			).not.toBeVisible();

			// Clear and search for other
			await searchInput.fill(otherWs.name);
			await expect(
				main.locator('h3').filter({ hasText: otherWs.name }),
			).toBeVisible();
			await expect(
				main.locator('h3').filter({ hasText: testWorkspace.name }),
			).not.toBeVisible();
		} finally {
			await deleteTestWorkspace(otherWs.id).catch(() => {});
		}
	});

	test('quick action links navigate correctly', async ({ page, testWorkspace }) => {
		// Create at least one issue so stats load
		await createTestIssue(testWorkspace.id, { title: 'Nav test issue' });

		await page.goto(`/${testWorkspace.id}`);

		// Wait for stats to load
		await expect(page.getByText('Loading stats...')).not.toBeVisible({ timeout: 10000 });

		// Click "All Issues" link
		await page.getByRole('link', { name: 'All Issues' }).click();

		// Verify URL contains the issues path
		await expect(page).toHaveURL(new RegExp(`/${testWorkspace.id}/issues`));
	});

	test('empty state shows when no workspaces', async ({ page }) => {
		// First, clean up all workspaces
		const res = await fetch('http://localhost:7433/api/v1/workspaces');
		const allWs = (await res.json()) as { id: string }[];
		for (const ws of allWs) {
			await deleteTestWorkspace(ws.id).catch(() => {});
		}

		// Navigate fresh so the SPA fetches the empty list
		await page.goto('/');
		const main = page.locator('main');
		await expect(
			main.getByRole('heading', { name: 'No workspaces yet' }),
		).toBeVisible({ timeout: 10000 });
	});
});
