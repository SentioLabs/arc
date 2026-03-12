import { test, expect, createTestIssue, updateTestIssue } from './fixtures';

test.describe('Issue Filtering and Search', () => {
	// Create 5 issues with varied attributes in beforeEach
	test.beforeEach(async ({ testWorkspace }) => {
		const wsId = testWorkspace.id;

		// "Bug Alpha": type=bug, priority=0 (open by default)
		await createTestIssue(wsId, { title: 'Bug Alpha', issue_type: 'bug', priority: 0 });

		// "Feature Beta": type=feature, priority=1, then update to in_progress
		const featureBeta = await createTestIssue(wsId, {
			title: 'Feature Beta',
			issue_type: 'feature',
			priority: 1
		});
		await updateTestIssue(wsId, featureBeta.id, { status: 'in_progress' });

		// "Task Gamma": type=task, priority=2 (open)
		await createTestIssue(wsId, { title: 'Task Gamma', issue_type: 'task', priority: 2 });

		// "Epic Delta": type=epic, priority=3, then update to closed
		const epicDelta = await createTestIssue(wsId, {
			title: 'Epic Delta',
			issue_type: 'epic',
			priority: 3
		});
		await updateTestIssue(wsId, epicDelta.id, { status: 'closed' });

		// "Chore Epsilon": type=chore, priority=4, then update to deferred
		const choreEpsilon = await createTestIssue(wsId, {
			title: 'Chore Epsilon',
			issue_type: 'chore',
			priority: 4
		});
		await updateTestIssue(wsId, choreEpsilon.id, { status: 'deferred' });
	});

	test('all issues shown by default', async ({ page, testWorkspace }) => {
		await page.goto(`/${testWorkspace.id}/issues`);

		// Wait for issues to load
		await expect(page.getByText('Bug Alpha')).toBeVisible();
		await expect(page.getByText('Feature Beta')).toBeVisible();
		await expect(page.getByText('Task Gamma')).toBeVisible();
		await expect(page.getByText('Epic Delta')).toBeVisible();
		await expect(page.getByText('Chore Epsilon')).toBeVisible();
	});

	test('filter by status', async ({ page, testWorkspace }) => {
		await page.goto(`/${testWorkspace.id}/issues`);
		await expect(page.getByText('Bug Alpha')).toBeVisible();

		// Click the status dropdown (first Select button showing "All Statuses")
		const statusTrigger = page
			.locator('button[aria-haspopup="listbox"]')
			.filter({ hasText: 'All Statuses' });
		await statusTrigger.click();

		// Select "Open"
		await page.getByRole('option', { name: 'Open' }).click();

		// Open issues: Bug Alpha, Task Gamma
		await expect(page.getByText('Bug Alpha')).toBeVisible();
		await expect(page.getByText('Task Gamma')).toBeVisible();

		// Non-open issues should not be visible
		await expect(page.getByText('Feature Beta')).not.toBeVisible();
		await expect(page.getByText('Epic Delta')).not.toBeVisible();
		await expect(page.getByText('Chore Epsilon')).not.toBeVisible();
	});

	test('filter by type', async ({ page, testWorkspace }) => {
		await page.goto(`/${testWorkspace.id}/issues`);
		await expect(page.getByText('Bug Alpha')).toBeVisible();

		// Click the type dropdown (showing "All Types")
		const typeTrigger = page
			.locator('button[aria-haspopup="listbox"]')
			.filter({ hasText: 'All Types' });
		await typeTrigger.click();

		// Select "Bug"
		await page.getByRole('option', { name: 'Bug' }).click();

		// Only Bug Alpha should be visible
		await expect(page.getByText('Bug Alpha')).toBeVisible();
		await expect(page.getByText('Feature Beta')).not.toBeVisible();
		await expect(page.getByText('Task Gamma')).not.toBeVisible();
		await expect(page.getByText('Epic Delta')).not.toBeVisible();
		await expect(page.getByText('Chore Epsilon')).not.toBeVisible();
	});

	test('filter by priority', async ({ page, testWorkspace }) => {
		await page.goto(`/${testWorkspace.id}/issues`);
		await expect(page.getByText('Bug Alpha')).toBeVisible();

		// Click the priority dropdown (showing "All Priorities")
		const priorityTrigger = page
			.locator('button[aria-haspopup="listbox"]')
			.filter({ hasText: 'All Priorities' });
		await priorityTrigger.click();

		// Select "High (P1)" — Feature Beta is the only issue with priority 1.
		// Note: priority 0 (Critical) cannot be tested via creation because the server's
		// SetDefaults() overrides priority 0 to 2.
		await page.getByRole('option', { name: 'High (P1)' }).click();

		// Only Feature Beta (priority 1) should be visible
		await expect(page.getByText('Feature Beta')).toBeVisible();
		await expect(page.getByText('Bug Alpha')).not.toBeVisible();
		await expect(page.getByText('Task Gamma')).not.toBeVisible();
		await expect(page.getByText('Epic Delta')).not.toBeVisible();
		await expect(page.getByText('Chore Epsilon')).not.toBeVisible();
	});

	test('search by text', async ({ page, testWorkspace }) => {
		await page.goto(`/${testWorkspace.id}/issues`);
		await expect(page.getByText('Bug Alpha')).toBeVisible();

		// Type in the search input
		const searchInput = page.getByPlaceholder('Search issues...');
		await searchInput.fill('Beta');

		// Wait for debounce (300ms) + some buffer
		await page.waitForTimeout(400);

		// Only Feature Beta should be visible
		await expect(page.getByText('Feature Beta')).toBeVisible();
		await expect(page.getByText('Bug Alpha')).not.toBeVisible();
		await expect(page.getByText('Task Gamma')).not.toBeVisible();
	});

	test('combined filters', async ({ page, testWorkspace }) => {
		await page.goto(`/${testWorkspace.id}/issues`);
		await expect(page.getByText('Bug Alpha')).toBeVisible();

		// Set status to "Open"
		const statusTrigger = page
			.locator('button[aria-haspopup="listbox"]')
			.filter({ hasText: 'All Statuses' });
		await statusTrigger.click();
		await page.getByRole('option', { name: 'Open' }).click();

		// Wait for filter to apply
		await expect(page.getByText('Feature Beta')).not.toBeVisible();

		// Set type to "Task"
		const typeTrigger = page
			.locator('button[aria-haspopup="listbox"]')
			.filter({ hasText: 'All Types' });
		await typeTrigger.click();
		await page.getByRole('option', { name: 'Task' }).click();

		// Only Task Gamma should be visible (open + task)
		await expect(page.getByText('Task Gamma')).toBeVisible();
		await expect(page.getByText('Bug Alpha')).not.toBeVisible();
	});

	test('clear filters', async ({ page, testWorkspace }) => {
		await page.goto(`/${testWorkspace.id}/issues`);
		await expect(page.getByText('Bug Alpha')).toBeVisible();

		// Apply a filter
		const statusTrigger = page
			.locator('button[aria-haspopup="listbox"]')
			.filter({ hasText: 'All Statuses' });
		await statusTrigger.click();
		await page.getByRole('option', { name: 'Open' }).click();

		// Verify filter is active (some issues hidden)
		await expect(page.getByText('Feature Beta')).not.toBeVisible();

		// Click "Clear filters"
		await page.getByRole('button', { name: 'Clear filters' }).click();

		// All 5 issues should be visible again
		await expect(page.getByText('Bug Alpha')).toBeVisible();
		await expect(page.getByText('Feature Beta')).toBeVisible();
		await expect(page.getByText('Task Gamma')).toBeVisible();
		await expect(page.getByText('Epic Delta')).toBeVisible();
		await expect(page.getByText('Chore Epsilon')).toBeVisible();
	});

	test('issue count updates with filters', async ({ page, testWorkspace }) => {
		await page.goto(`/${testWorkspace.id}/issues`);

		// Verify initial count shows 5 issues
		await expect(page.getByText('5 issues')).toBeVisible();

		// Apply status filter to "Open"
		const statusTrigger = page
			.locator('button[aria-haspopup="listbox"]')
			.filter({ hasText: 'All Statuses' });
		await statusTrigger.click();
		await page.getByRole('option', { name: 'Open' }).click();

		// Count should decrease to 2 issues
		await expect(page.getByText('2 issues')).toBeVisible();
	});
});
