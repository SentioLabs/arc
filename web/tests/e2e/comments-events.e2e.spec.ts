import { test, expect, createTestIssue, updateTestIssue, addTestComment } from './fixtures';

test.describe('Comments and Events Timeline', () => {
	test('post comment via form', async ({ page, testWorkspace }) => {
		const issue = await createTestIssue(testWorkspace.id, { title: 'Comment form test' });
		const issueId = issue.id as string;

		await page.goto(`/${testWorkspace.id}/issues/${issueId}`);
		await expect(page.getByText('Loading issue...')).not.toBeVisible({ timeout: 10000 });

		// Fill the comment textarea and post
		const textarea = page.getByLabel('Add a comment');
		await textarea.fill('Test comment from E2E');

		const postButton = page.getByRole('button', { name: 'Post comment' });
		await expect(postButton).toBeEnabled();
		await postButton.click();

		// Verify the comment appears without page reload
		await expect(page.getByText('Test comment from E2E')).toBeVisible({ timeout: 10000 });
	});

	test('multiple comments in order', async ({ page, testWorkspace }) => {
		const issue = await createTestIssue(testWorkspace.id, { title: 'Multiple comments test' });
		const issueId = issue.id as string;

		// Add 3 comments via API
		await addTestComment(testWorkspace.id, issueId, 'First');
		await addTestComment(testWorkspace.id, issueId, 'Second');
		await addTestComment(testWorkspace.id, issueId, 'Third');

		await page.goto(`/${testWorkspace.id}/issues/${issueId}`);
		await expect(page.getByText('Loading issue...')).not.toBeVisible({ timeout: 10000 });

		// Verify all 3 comments are visible
		await expect(page.getByText('First')).toBeVisible();
		await expect(page.getByText('Second')).toBeVisible();
		await expect(page.getByText('Third')).toBeVisible();

		// Verify order: First should appear before Second, Second before Third
		const commentsSection = page.locator('section', { has: page.getByText('Comments (3)') });
		const commentTexts = await commentsSection.locator('.border-b, .last\\:border-0').allTextContents();
		const orderedTexts = commentTexts.join('|||');
		expect(orderedTexts.indexOf('First')).toBeLessThan(orderedTexts.indexOf('Second'));
		expect(orderedTexts.indexOf('Second')).toBeLessThan(orderedTexts.indexOf('Third'));
	});

	test('comments count shown', async ({ page, testWorkspace }) => {
		const issue = await createTestIssue(testWorkspace.id, { title: 'Comments count test' });
		const issueId = issue.id as string;

		// Add 2 comments via API
		await addTestComment(testWorkspace.id, issueId, 'Comment one');
		await addTestComment(testWorkspace.id, issueId, 'Comment two');

		await page.goto(`/${testWorkspace.id}/issues/${issueId}`);
		await expect(page.getByText('Loading issue...')).not.toBeVisible({ timeout: 10000 });

		// Verify "Comments (2)" is visible
		await expect(page.getByText('Comments (2)')).toBeVisible();
	});

	test('events show creation', async ({ page, testWorkspace }) => {
		const issue = await createTestIssue(testWorkspace.id, { title: 'Creation event test' });
		const issueId = issue.id as string;

		await page.goto(`/${testWorkspace.id}/issues/${issueId}`);
		await expect(page.getByText('Loading issue...')).not.toBeVisible({ timeout: 10000 });

		// Verify Activity section shows a "created" event
		const activitySection = page.locator('section', { has: page.getByText('Activity') });
		await expect(activitySection.getByText('created this issue')).toBeVisible();
	});

	test('events show status change', async ({ page, testWorkspace }) => {
		const issue = await createTestIssue(testWorkspace.id, { title: 'Status change event test' });
		const issueId = issue.id as string;

		// Update status via API
		await updateTestIssue(testWorkspace.id, issueId, { status: 'in_progress' });

		await page.goto(`/${testWorkspace.id}/issues/${issueId}`);
		await expect(page.getByText('Loading issue...')).not.toBeVisible({ timeout: 10000 });

		// Verify Activity section shows status change event
		const activitySection = page.locator('section', { has: page.getByText('Activity') });
		await expect(activitySection.getByText('changed status from')).toBeVisible();
		await expect(activitySection.getByText('in_progress')).toBeVisible();
	});

	test('empty comments state', async ({ page, testWorkspace }) => {
		const issue = await createTestIssue(testWorkspace.id, { title: 'Empty comments test' });
		const issueId = issue.id as string;

		await page.goto(`/${testWorkspace.id}/issues/${issueId}`);
		await expect(page.getByText('Loading issue...')).not.toBeVisible({ timeout: 10000 });

		// Verify "Comments (0)" is visible
		await expect(page.getByText('Comments (0)')).toBeVisible();
		// Verify "No comments yet" message
		await expect(page.getByText('No comments yet')).toBeVisible();
	});
});
