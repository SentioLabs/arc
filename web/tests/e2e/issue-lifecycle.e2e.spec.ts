import { test, expect, createTestIssue } from './fixtures';

test.describe('Issue Lifecycle E2E', () => {
	test('create issue via form', async ({ page, testWorkspace: ws }) => {
		await page.goto(`/${ws.id}/issues/new`);

		// Fill in the title
		await page.getByPlaceholder('Brief description of the issue').fill('E2E test bug report');

		// Select type Bug
		await page.locator('#type').selectOption('bug');

		// Select priority P1 - High
		await page.locator('#priority').selectOption('1');

		// Submit
		await page.getByRole('button', { name: 'Create Issue' }).click();

		// Should redirect to detail page with the title visible
		await expect(page).not.toHaveURL(/\/issues\/new$/);
		await expect(page.getByText('E2E test bug report')).toBeVisible();
	});

	test('issue appears in list', async ({ page, testWorkspace: ws }) => {
		await createTestIssue(ws.id, { title: 'List visibility test issue' });

		await page.goto(`/${ws.id}/issues`);

		await expect(page.getByText('List visibility test issue')).toBeVisible();
	});

	test('detail shows all fields', async ({ page, testWorkspace: ws }) => {
		const issue = await createTestIssue(ws.id, {
			title: 'Fully loaded issue',
			issue_type: 'bug',
			priority: 1,
			description: 'Detailed description here',
		});

		await page.goto(`/${ws.id}/issues/${issue.id}`);

		// Title
		await expect(page.getByText('Fully loaded issue')).toBeVisible();

		// Type badge (Bug)
		const typeBadge = page.locator('button[aria-haspopup="listbox"]').filter({ hasText: /bug/i });
		await expect(typeBadge).toBeVisible();

		// Status badge (open by default)
		const statusBadge = page.locator('button[aria-haspopup="listbox"]').filter({ hasText: /open/i });
		await expect(statusBadge).toBeVisible();

		// Priority badge (P1)
		const priorityBadge = page.locator('button[aria-haspopup="listbox"]').filter({ hasText: /P1/i });
		await expect(priorityBadge).toBeVisible();

		// Description
		await expect(page.getByText('Detailed description here')).toBeVisible();
	});

	test('update title inline', async ({ page, testWorkspace: ws }) => {
		const issue = await createTestIssue(ws.id, { title: 'Original title' });
		await page.goto(`/${ws.id}/issues/${issue.id}`);

		// Wait for the title button to appear, then click to enter edit mode
		const titleButton = page.getByRole('button', { name: 'Original title' });
		await expect(titleButton).toBeVisible();
		await titleButton.click();

		// The InlineTextEdit input appears inside the title container (text-2xl wrapper)
		const titleInput = page.locator('.text-2xl input[type="text"]');
		await expect(titleInput).toBeVisible();

		// Clear and type new title, then blur to save
		await titleInput.fill('Updated title');
		await titleInput.blur();

		// Wait for the save to complete and the button text to update
		await expect(page.getByRole('button', { name: 'Updated title' })).toBeVisible({ timeout: 15000 });
	});

	test('update status inline', async ({ page, testWorkspace: ws }) => {
		const issue = await createTestIssue(ws.id, { title: 'Status test issue' });
		await page.goto(`/${ws.id}/issues/${issue.id}`);

		// Click the status badge to open dropdown
		const statusBadge = page.locator('button[aria-haspopup="listbox"]').filter({ hasText: /open/i });
		await statusBadge.click();

		// Select "In Progress" from listbox
		await page.getByRole('option', { name: 'In Progress' }).click();

		// Verify badge changes — look for in_progress or "In Progress" text in the badge area
		await expect(
			page.locator('button[aria-haspopup="listbox"]').filter({ hasText: /in.progress/i })
		).toBeVisible();
	});

	test('update priority inline', async ({ page, testWorkspace: ws }) => {
		const issue = await createTestIssue(ws.id, { title: 'Priority test issue', priority: 2 });
		await page.goto(`/${ws.id}/issues/${issue.id}`);

		// Click the priority badge
		const priorityBadge = page.locator('button[aria-haspopup="listbox"]').filter({ hasText: /P2/i });
		await priorityBadge.click();

		// Select Critical (P0)
		await page.getByRole('option', { name: /Critical/i }).click();

		// Verify badge changes to P0
		await expect(
			page.locator('button[aria-haspopup="listbox"]').filter({ hasText: /P0/i })
		).toBeVisible();
	});

	test('update type inline', async ({ page, testWorkspace: ws }) => {
		const issue = await createTestIssue(ws.id, { title: 'Type test issue', issue_type: 'task' });
		await page.goto(`/${ws.id}/issues/${issue.id}`);

		// Click the type badge
		const typeBadge = page.locator('button[aria-haspopup="listbox"]').filter({ hasText: /task/i });
		await typeBadge.click();

		// Select Bug
		await page.getByRole('option', { name: 'Bug' }).click();

		// Verify badge changes to bug
		await expect(
			page.locator('button[aria-haspopup="listbox"]').filter({ hasText: /bug/i })
		).toBeVisible();
	});

	test('update description inline', async ({ page, testWorkspace: ws }) => {
		const issue = await createTestIssue(ws.id, {
			title: 'Description test issue',
			description: 'Old description',
		});
		await page.goto(`/${ws.id}/issues/${issue.id}`);

		// Click Edit button to enter edit mode (the one in the description section)
		await page.getByRole('button', { name: 'Edit' }).click();

		// A textarea should appear for the description (use placeholder to disambiguate)
		const textarea = page.getByPlaceholder('Add a description...');
		await expect(textarea).toBeVisible();

		// Clear and type new description
		await textarea.fill('New description content');

		// Click Save
		await page.getByRole('button', { name: 'Save' }).click();

		// Verify the new content is visible
		await expect(page.getByText('New description content')).toBeVisible();
	});

	test('close issue via status', async ({ page, testWorkspace: ws }) => {
		const issue = await createTestIssue(ws.id, { title: 'Close test issue' });
		await page.goto(`/${ws.id}/issues/${issue.id}`);

		// Click the status badge
		const statusBadge = page.locator('button[aria-haspopup="listbox"]').filter({ hasText: /open/i });
		await statusBadge.click();

		// Select "Closed"
		await page.getByRole('option', { name: 'Closed' }).click();

		// Verify closed status
		await expect(
			page.locator('button[aria-haspopup="listbox"]').filter({ hasText: /closed/i })
		).toBeVisible();
	});

	test('create multiple types', async ({ page, testWorkspace: ws }) => {
		const issueTypes = [
			{ value: 'bug', label: 'Bug' },
			{ value: 'feature', label: 'Feature' },
			{ value: 'task', label: 'Task' },
			{ value: 'epic', label: 'Epic' },
			{ value: 'chore', label: 'Chore' },
		];

		for (const issueType of issueTypes) {
			await page.goto(`/${ws.id}/issues/new`);

			await page.getByPlaceholder('Brief description of the issue').fill(`Test ${issueType.label} issue`);
			await page.locator('#type').selectOption(issueType.value);
			await page.getByRole('button', { name: 'Create Issue' }).click();

			// Verify redirect to detail and correct type badge is shown
			await expect(page).not.toHaveURL(/\/issues\/new$/);
			const typeBadge = page.locator('button[aria-haspopup="listbox"]').filter({
				hasText: new RegExp(issueType.label, 'i'),
			});
			await expect(typeBadge).toBeVisible();
		}
	});
});
