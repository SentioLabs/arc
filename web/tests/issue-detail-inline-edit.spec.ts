import { test, expect } from '@playwright/test';

const mockProjects = [
	{
		id: 'ws-test1',
		name: 'Test Project',
		prefix: 'TW',
		description: 'Test project',
		created_at: '2024-01-01T00:00:00Z',
		updated_at: '2024-01-01T00:00:00Z'
	}
];

const mockIssue = {
	id: 'TW-1',
	project_id: 'ws-test1',
	title: 'Test Issue Title',
	description: 'Some **markdown** description',
	status: 'open',
	priority: 2,
	issue_type: 'task',
	labels: ['bug', 'urgent'],
	external_ref: null,
	close_reason: null,
	closed_at: null,
	created_at: '2024-01-01T00:00:00Z',
	updated_at: '2024-01-01T00:00:00Z'
};

const mockLabels = [
	{ name: 'bug', color: '#ff0000', description: 'Bug label' },
	{ name: 'urgent', color: '#ff8800', description: 'Urgent label' },
	{ name: 'enhancement', color: '#00ff00', description: 'Enhancement' }
];

test.describe('Issue Detail - Inline Editing', () => {
	test.beforeEach(async ({ page }) => {
		// Mock project list (root layout)
		await page.route('**/api/v1/projects', async (route) => {
			if (route.request().method() === 'GET') {
				await route.fulfill({
					status: 200,
					contentType: 'application/json',
					body: JSON.stringify(mockProjects)
				});
			}
		});

		// Mock issue endpoint and sub-resources
		await page.route('**/api/v1/projects/ws-test1/issues/TW-1**', async (route) => {
			const url = route.request().url();
			if (url.includes('/comments')) {
				await route.fulfill({
					status: 200,
					contentType: 'application/json',
					body: JSON.stringify([])
				});
			} else if (url.includes('/events')) {
				await route.fulfill({
					status: 200,
					contentType: 'application/json',
					body: JSON.stringify([])
				});
			} else if (url.includes('/deps')) {
				await route.fulfill({
					status: 200,
					contentType: 'application/json',
					body: JSON.stringify({ dependencies: [], dependents: [] })
				});
			} else if (url.includes('/labels')) {
				await route.fulfill({
					status: 200,
					contentType: 'application/json',
					body: JSON.stringify(null)
				});
			} else if (route.request().method() === 'PUT') {
				// Mock update endpoint - return updated issue
				const body = JSON.parse(route.request().postData() || '{}');
				await route.fulfill({
					status: 200,
					contentType: 'application/json',
					body: JSON.stringify({ ...mockIssue, ...body })
				});
			} else {
				await route.fulfill({
					status: 200,
					contentType: 'application/json',
					body: JSON.stringify(mockIssue)
				});
			}
		});

		// Mock global labels endpoint
		await page.route('**/api/v1/labels', async (route) => {
			await route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify(mockLabels)
			});
		});
	});

	test('badges are wrapped in clickable inline selects', async ({ page }) => {
		await page.goto('/ws-test1/issues/TW-1');

		// Wait for issue to load
		await expect(page.getByText('TW-1').first()).toBeVisible();

		// The badges should be inside buttons (InlineSelect renders a button trigger)
		// TypeBadge should be clickable
		const typeBadge = page.locator('button[aria-haspopup="listbox"]').filter({ hasText: /task/i });
		await expect(typeBadge).toBeVisible();

		// StatusBadge should be clickable
		const statusBadge = page
			.locator('button[aria-haspopup="listbox"]')
			.filter({ hasText: /open/i });
		await expect(statusBadge).toBeVisible();

		// PriorityBadge should be clickable
		const priorityBadge = page
			.locator('button[aria-haspopup="listbox"]')
			.filter({ hasText: /P2/i });
		await expect(priorityBadge).toBeVisible();
	});

	test('title is editable via click', async ({ page }) => {
		await page.goto('/ws-test1/issues/TW-1');
		await expect(page.getByText('Test Issue Title')).toBeVisible();

		// Click on the title to edit it - InlineTextEdit renders a button in view mode
		await page.getByText('Test Issue Title').click();

		// An input should appear (InlineTextEdit switches to input on click)
		const input = page.locator('input[type="text"]').filter({ hasText: '' });
		await expect(input.first()).toBeVisible();
	});

	test('description section always shows (even when empty)', async ({ page }) => {
		await page.goto('/ws-test1/issues/TW-1');

		// Description heading should be visible
		await expect(page.getByRole('heading', { name: 'Description' })).toBeVisible();

		// The description markdown should be rendered
		await expect(page.getByText('markdown')).toBeVisible();

		// Should have an Edit button (InlineMarkdownEdit shows an Edit button)
		const editButton = page.locator('button').filter({ hasText: 'Edit' });
		await expect(editButton).toBeVisible();
	});

	test('label picker shows current labels with remove buttons', async ({ page }) => {
		await page.goto('/ws-test1/issues/TW-1');

		// Should show current labels
		await expect(page.getByText('bug').first()).toBeVisible();
		await expect(page.getByText('urgent').first()).toBeVisible();

		// Should show remove buttons (the x button) - LabelPicker has aria-label "Remove label X"
		await expect(page.getByLabel('Remove label bug')).toBeVisible();
		await expect(page.getByLabel('Remove label urgent')).toBeVisible();

		// Should show "+ Add" button for adding new labels
		await expect(page.getByText('+ Add')).toBeVisible();
	});

	test('comment form is present at bottom of comments section', async ({ page }) => {
		await page.goto('/ws-test1/issues/TW-1');

		// Wait for the page to load
		await expect(page.getByRole('heading', { name: /Comments/ })).toBeVisible();

		// CommentForm has a textarea with placeholder "Add a comment..."
		await expect(page.getByPlaceholder('Add a comment...')).toBeVisible();

		// And a "Post comment" button
		await expect(page.getByRole('button', { name: 'Post comment' })).toBeVisible();
	});

	test('clicking type badge opens dropdown with options', async ({ page }) => {
		await page.goto('/ws-test1/issues/TW-1');
		await expect(page.getByText('TW-1').first()).toBeVisible();

		// Click the type badge
		const typeBadge = page.locator('button[aria-haspopup="listbox"]').filter({ hasText: /task/i });
		await typeBadge.click();

		// Should show dropdown with issue type options
		await expect(page.getByRole('option', { name: 'Bug' })).toBeVisible();
		await expect(page.getByRole('option', { name: 'Feature' })).toBeVisible();
		await expect(page.getByRole('option', { name: 'Task' })).toBeVisible();
		await expect(page.getByRole('option', { name: 'Epic' })).toBeVisible();
		await expect(page.getByRole('option', { name: 'Chore' })).toBeVisible();
	});
});
