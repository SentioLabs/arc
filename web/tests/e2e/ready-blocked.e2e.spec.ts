import { test, expect, createTestIssue, updateTestIssue, addTestDependency } from './fixtures';

test.describe('Ready Work page', () => {
	test('shows unblocked open issues', async ({ page, testWorkspace: ws }) => {
		await createTestIssue(ws.id, { title: 'Ready issue alpha' });
		await createTestIssue(ws.id, { title: 'Ready issue beta' });

		await page.goto(`/${ws.id}/ready`);

		await expect(page.getByRole('heading', { name: 'Ready Work' })).toBeVisible();
		await expect(page.getByText('Ready issue alpha')).toBeVisible();
		await expect(page.getByText('Ready issue beta')).toBeVisible();
	});

	test('empty state when no issues exist', async ({ page, testWorkspace: ws }) => {
		await page.goto(`/${ws.id}/ready`);

		await expect(page.getByText('No ready work')).toBeVisible();
		await expect(
			page.getByText('All open issues have blocking dependencies or are already in progress')
		).toBeVisible();
	});

	test('excludes closed and blocked issues', async ({ page, testWorkspace: ws }) => {
		await createTestIssue(ws.id, { title: 'Still open issue' });
		const closedIssue = await createTestIssue(ws.id, { title: 'Closed issue' });
		const blockedIssue = await createTestIssue(ws.id, { title: 'Blocked issue' });
		const blockerIssue = await createTestIssue(ws.id, { title: 'Blocker for test' });

		await updateTestIssue(ws.id, closedIssue.id, { status: 'closed' });
		await addTestDependency(ws.id, blockedIssue.id, blockerIssue.id, 'blocks');
		await updateTestIssue(ws.id, blockedIssue.id, { status: 'blocked' });

		await page.goto(`/${ws.id}/ready`);

		await expect(page.getByText('Still open issue')).toBeVisible();
		await expect(page.getByText('Blocker for test')).toBeVisible();
		await expect(page.getByText('Closed issue')).not.toBeVisible();
		await expect(page.getByText('Blocked issue')).not.toBeVisible();
	});

	test('issue cards link to detail page', async ({ page, testWorkspace: ws }) => {
		const issue = await createTestIssue(ws.id, { title: 'Clickable issue' });

		await page.goto(`/${ws.id}/ready`);

		await expect(page.getByText('Clickable issue')).toBeVisible();

		// IssueCard is an <a> tag, click it
		await page.getByText('Clickable issue').click();

		// Should navigate to the issue detail page
		await expect(page).toHaveURL(new RegExp(`/${ws.id}/issues/${issue.id}`));
	});
});

test.describe('Blocked Issues page', () => {
	test('shows blocked issues', async ({ page, testWorkspace: ws }) => {
		const issueA = await createTestIssue(ws.id, { title: 'Blocked task' });
		const issueB = await createTestIssue(ws.id, { title: 'Blocker task' });

		// A is blocked by B
		await addTestDependency(ws.id, issueA.id, issueB.id, 'blocks');
		await updateTestIssue(ws.id, issueA.id, { status: 'blocked' });

		await page.goto(`/${ws.id}/blocked`);

		await expect(page.getByRole('heading', { name: 'Blocked Issues' })).toBeVisible();
		await expect(page.getByText('Blocked task')).toBeVisible();
	});

	test('shows blocker IDs', async ({ page, testWorkspace: ws }) => {
		const issueA = await createTestIssue(ws.id, { title: 'Blocked task for IDs' });
		const issueB = await createTestIssue(ws.id, { title: 'Blocker task for IDs' });

		await addTestDependency(ws.id, issueA.id, issueB.id, 'blocks');
		await updateTestIssue(ws.id, issueA.id, { status: 'blocked' });

		await page.goto(`/${ws.id}/blocked`);

		// The blocker's ID should appear as a reference in the "Blocked by:" section
		await expect(page.getByText(issueB.id)).toBeVisible();
	});

	test('empty state when no blocked issues', async ({ page, testWorkspace: ws }) => {
		await page.goto(`/${ws.id}/blocked`);

		await expect(page.getByText('No blocked issues')).toBeVisible();
		await expect(
			page.getByText('All issues are ready to work on or already resolved')
		).toBeVisible();
	});
});
