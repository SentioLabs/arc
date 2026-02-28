import { test, expect } from '@playwright/test';

// Mock workspace data
const mockWorkspaces = [
	{
		id: 'ws-test1',
		name: 'Test Workspace 1',
		prefix: 'tw1',
		path: '/tmp/test-workspace-1',
		description: 'First test workspace',
		created_at: '2024-01-01T00:00:00Z',
		updated_at: '2024-01-01T00:00:00Z'
	},
	{
		id: 'ws-test2',
		name: 'Test Workspace 2',
		prefix: 'tw2',
		path: '/tmp/test-workspace-2',
		description: 'Second test workspace',
		created_at: '2024-01-01T00:00:00Z',
		updated_at: '2024-01-01T00:00:00Z'
	},
	{
		id: 'ws-test3',
		name: 'Test Workspace 3',
		prefix: 'tw3',
		path: '/tmp/test-workspace-3',
		description: 'Third test workspace',
		created_at: '2024-01-01T00:00:00Z',
		updated_at: '2024-01-01T00:00:00Z'
	}
];

test.describe('Workspace Deletion', () => {
	test.beforeEach(async ({ page }) => {
		// Mock the API endpoints
		let currentWorkspaces = [...mockWorkspaces];

		await page.route('**/api/v1/workspaces', async (route) => {
			if (route.request().method() === 'GET') {
				await route.fulfill({
					status: 200,
					contentType: 'application/json',
					body: JSON.stringify(currentWorkspaces)
				});
			} else {
				await route.continue();
			}
		});

		await page.route('**/api/v1/workspaces/*', async (route) => {
			if (route.request().method() === 'DELETE') {
				const url = route.request().url();
				const workspaceId = url.split('/').pop();
				currentWorkspaces = currentWorkspaces.filter((w) => w.id !== workspaceId);
				await route.fulfill({ status: 204 });
			} else {
				await route.continue();
			}
		});
	});

	test('shows edit button when workspaces exist', async ({ page }) => {
		await page.goto('/');

		// Wait for workspaces to load
		await expect(page.getByRole('heading', { name: 'Workspaces' })).toBeVisible();

		// Edit button should be visible
		const editButton = page.getByRole('button', { name: 'Edit' });
		await expect(editButton).toBeVisible();
	});

	test('enters edit mode and shows selection UI', async ({ page }) => {
		await page.goto('/');

		// Click edit button
		await page.getByRole('button', { name: 'Edit' }).click();

		// Should show "Done" button
		await expect(page.getByRole('button', { name: 'Done' })).toBeVisible();

		// Should show "Select all" checkbox
		await expect(page.getByText('Select all')).toBeVisible();

		// Should show checkboxes on cards
		const checkboxes = page.locator('.checkbox');
		await expect(checkboxes).toHaveCount(4); // 3 cards + 1 select all
	});

	test('selects single workspace and shows delete button', async ({ page }) => {
		await page.goto('/');

		// Enter edit mode
		await page.getByRole('button', { name: 'Edit' }).click();

		// Click on the first workspace card to select it
		await page.getByRole('button', { name: /Test Workspace 1/i }).click();

		// Should show "1 selected" text
		await expect(page.getByText('1 selected')).toBeVisible();

		// Should show batch delete button (target the button element with exact text)
		await expect(
			page.locator('button.btn-danger').filter({ hasText: 'Delete workspace' })
		).toBeVisible();
	});

	test('selects multiple workspaces', async ({ page }) => {
		await page.goto('/');

		// Enter edit mode
		await page.getByRole('button', { name: 'Edit' }).click();

		// Click on multiple workspace cards
		await page.getByRole('button', { name: /Test Workspace 1/i }).click();
		await page.getByRole('button', { name: /Test Workspace 2/i }).click();

		// Should show "2 selected" text
		await expect(page.getByText('2 selected')).toBeVisible();

		// Should show delete button with count
		await expect(page.getByRole('button', { name: 'Delete 2 workspaces' })).toBeVisible();
	});

	test('select all selects all workspaces', async ({ page }) => {
		await page.goto('/');

		// Enter edit mode
		await page.getByRole('button', { name: 'Edit' }).click();

		// Click "Select all" checkbox
		await page.getByText('Select all').click();

		// Should show "3 selected" text
		await expect(page.getByText('3 selected')).toBeVisible();
	});

	test('shows confirmation dialog when deleting', async ({ page }) => {
		await page.goto('/');

		// Enter edit mode
		await page.getByRole('button', { name: 'Edit' }).click();

		// Select a workspace
		await page.getByRole('button', { name: /Test Workspace 1/i }).click();

		// Click batch delete button
		await page.locator('button.btn-danger').filter({ hasText: 'Delete workspace' }).click();

		// Confirmation dialog should appear
		const dialog = page.getByRole('dialog');
		await expect(page.getByRole('heading', { name: 'Delete workspace?' })).toBeVisible();
		await expect(page.getByText('This action cannot be undone')).toBeVisible();
		await expect(dialog.getByText('Test Workspace 1')).toBeVisible();
	});

	test('cancels deletion when clicking cancel', async ({ page }) => {
		await page.goto('/');

		// Enter edit mode and select workspace
		await page.getByRole('button', { name: 'Edit' }).click();
		await page.getByRole('button', { name: /Test Workspace 1/i }).click();
		await page.locator('button.btn-danger').filter({ hasText: 'Delete workspace' }).click();

		// Click cancel
		await page.getByRole('button', { name: 'Cancel' }).click();

		// Dialog should close
		await expect(page.getByRole('heading', { name: 'Delete workspace?' })).not.toBeVisible();

		// Workspace should still be selected
		await expect(page.getByText('1 selected')).toBeVisible();
	});

	test('deletes single workspace', async ({ page }) => {
		await page.goto('/');

		// Enter edit mode and select workspace
		await page.getByRole('button', { name: 'Edit' }).click();
		await page.getByRole('button', { name: /Test Workspace 1/i }).click();
		await page.locator('button.btn-danger').filter({ hasText: 'Delete workspace' }).click();

		// Confirm deletion (target the button inside the dialog)
		const dialog = page.getByRole('dialog');
		await dialog.getByRole('button', { name: 'Delete Workspace' }).click();

		// Wait for dialog to close
		await expect(page.getByRole('heading', { name: 'Delete workspace?' })).not.toBeVisible();

		// Workspace should be removed from the list (check heading specifically)
		await expect(page.getByRole('heading', { name: 'Test Workspace 1' })).not.toBeVisible();

		// Other workspaces should still be visible
		await expect(page.getByRole('heading', { name: 'Test Workspace 2' })).toBeVisible();
		await expect(page.getByRole('heading', { name: 'Test Workspace 3' })).toBeVisible();
	});

	test('deletes multiple workspaces', async ({ page }) => {
		await page.goto('/');

		// Enter edit mode and select workspaces
		await page.getByRole('button', { name: 'Edit' }).click();
		await page.getByRole('button', { name: /Test Workspace 1/i }).click();
		await page.getByRole('button', { name: /Test Workspace 2/i }).click();
		await page.getByRole('button', { name: 'Delete 2 workspaces' }).click();

		// Confirm deletion (target the button inside the dialog)
		const dialog = page.getByRole('dialog');
		await dialog.getByRole('button', { name: 'Delete Workspaces' }).click();

		// Wait for dialog to close
		await expect(page.getByRole('heading', { name: 'Delete 2 workspaces?' })).not.toBeVisible();

		// Deleted workspaces should be removed (check headings specifically)
		await expect(page.getByRole('heading', { name: 'Test Workspace 1' })).not.toBeVisible();
		await expect(page.getByRole('heading', { name: 'Test Workspace 2' })).not.toBeVisible();

		// Remaining workspace should still be visible
		await expect(page.getByRole('heading', { name: 'Test Workspace 3' })).toBeVisible();
	});

	test('exits edit mode when clicking Done', async ({ page }) => {
		await page.goto('/');

		// Enter edit mode
		await page.getByRole('button', { name: 'Edit' }).click();
		await expect(page.getByRole('button', { name: 'Done' })).toBeVisible();

		// Exit edit mode
		await page.getByRole('button', { name: 'Done' }).click();

		// Should show Edit button again
		await expect(page.getByRole('button', { name: 'Edit' })).toBeVisible();

		// Selection UI should be hidden
		await expect(page.getByText('Select all')).not.toBeVisible();
	});

	test('clears selection when exiting edit mode', async ({ page }) => {
		await page.goto('/');

		// Enter edit mode and select workspaces
		await page.getByRole('button', { name: 'Edit' }).click();
		await page.getByRole('button', { name: /Test Workspace 1/i }).click();
		await expect(page.getByText('1 selected')).toBeVisible();

		// Exit edit mode
		await page.getByRole('button', { name: 'Done' }).click();

		// Re-enter edit mode
		await page.getByRole('button', { name: 'Edit' }).click();

		// Selection should be cleared
		await expect(page.getByText('Select all')).toBeVisible();
		await expect(page.getByText('1 selected')).not.toBeVisible();
	});

	test('individual delete button works on hover', async ({ page }) => {
		await page.goto('/');

		// Enter edit mode
		await page.getByRole('button', { name: 'Edit' }).click();

		// Hover over the first workspace card to reveal delete button
		const firstCard = page.getByRole('button', { name: /Test Workspace 1/i });
		await firstCard.hover();

		// Find and click the individual delete button (trash icon)
		const deleteButton = firstCard.locator('[title="Delete workspace"]');
		await deleteButton.click();

		// Confirmation dialog should appear for single workspace
		const dialog = page.getByRole('dialog');
		await expect(page.getByRole('heading', { name: 'Delete workspace?' })).toBeVisible();
		await expect(dialog.getByText('Test Workspace 1')).toBeVisible();
	});

	test('keyboard navigation works in edit mode', async ({ page }) => {
		await page.goto('/');

		// Enter edit mode
		await page.getByRole('button', { name: 'Edit' }).click();

		// Tab to first workspace card
		await page.keyboard.press('Tab');
		await page.keyboard.press('Tab'); // Skip select all checkbox

		// Press Enter to select
		await page.keyboard.press('Enter');

		// Should show "1 selected"
		await expect(page.getByText('1 selected')).toBeVisible();
	});
});

test.describe('Workspace Deletion - Empty State', () => {
	test('hides edit button when no workspaces', async ({ page }) => {
		// Mock empty workspaces
		await page.route('**/api/v1/workspaces', async (route) => {
			await route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify([])
			});
		});

		await page.goto('/');

		// Should show empty state (use heading role for specificity)
		await expect(page.getByRole('heading', { name: 'No workspaces yet' })).toBeVisible();

		// Edit button should not be visible
		await expect(page.getByRole('button', { name: 'Edit' })).not.toBeVisible();
	});
});
