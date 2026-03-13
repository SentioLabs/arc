import { test, expect } from '@playwright/test';

// Mock project data
const mockProjects = [
	{
		id: 'ws-test1',
		name: 'Test Project 1',
		prefix: 'tp1',
		description: 'First test project',
		created_at: '2024-01-01T00:00:00Z',
		updated_at: '2024-01-01T00:00:00Z'
	},
	{
		id: 'ws-test2',
		name: 'Test Project 2',
		prefix: 'tp2',
		description: 'Second test project',
		created_at: '2024-01-01T00:00:00Z',
		updated_at: '2024-01-01T00:00:00Z'
	},
	{
		id: 'ws-test3',
		name: 'Test Project 3',
		prefix: 'tp3',
		description: 'Third test project',
		created_at: '2024-01-01T00:00:00Z',
		updated_at: '2024-01-01T00:00:00Z'
	}
];

test.describe('Project Deletion', () => {
	test.beforeEach(async ({ page }) => {
		let currentProjects = [...mockProjects];

		// Mock listing projects
		await page.route('**/api/v1/projects', async (route) => {
			if (route.request().method() === 'GET') {
				await route.fulfill({
					status: 200,
					contentType: 'application/json',
					body: JSON.stringify(currentProjects)
				});
			} else {
				await route.continue();
			}
		});

		// Mock workspace paths (for project insights loading)
		await page.route('**/api/v1/projects/*/workspaces', async (route) => {
			await route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify([])
			});
		});

		// Mock individual project delete
		await page.route(/\/api\/v1\/projects\/[^/]+$/, async (route) => {
			if (route.request().method() === 'DELETE') {
				const url = route.request().url();
				const projectId = url.split('/').pop();
				currentProjects = currentProjects.filter((p) => p.id !== projectId);
				await route.fulfill({ status: 204 });
			} else {
				await route.continue();
			}
		});
	});

	test('shows edit button when projects exist', async ({ page }) => {
		await page.goto('/');

		// Wait for projects to load
		await expect(page.getByRole('heading', { name: 'Projects', exact: true })).toBeVisible();

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

	test('selects single project and shows delete button', async ({ page }) => {
		await page.goto('/');

		// Enter edit mode
		await page.getByRole('button', { name: 'Edit' }).click();

		// Click on the first project card to select it
		await page.getByRole('button', { name: /Test Project 1/i }).click();

		// Should show "1 selected" text
		await expect(page.getByText('1 selected')).toBeVisible();

		// Should show batch delete button
		await expect(
			page.locator('button.btn-danger').filter({ hasText: 'Delete project' })
		).toBeVisible();
	});

	test('selects multiple projects', async ({ page }) => {
		await page.goto('/');

		// Enter edit mode
		await page.getByRole('button', { name: 'Edit' }).click();

		// Click on multiple project cards
		await page.getByRole('button', { name: /Test Project 1/i }).click();
		await page.getByRole('button', { name: /Test Project 2/i }).click();

		// Should show "2 selected" text
		await expect(page.getByText('2 selected')).toBeVisible();

		// Should show delete button with count
		await expect(page.getByRole('button', { name: 'Delete 2 projects' })).toBeVisible();
	});

	test('select all selects all projects', async ({ page }) => {
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

		// Select a project
		await page.getByRole('button', { name: /Test Project 1/i }).click();

		// Click batch delete button
		await page.locator('button.btn-danger').filter({ hasText: 'Delete project' }).click();

		// Confirmation dialog should appear
		const dialog = page.getByRole('dialog');
		await expect(page.getByRole('heading', { name: 'Delete project?' })).toBeVisible();
		await expect(dialog.getByText('Test Project 1')).toBeVisible();
	});

	test('cancels deletion when clicking cancel', async ({ page }) => {
		await page.goto('/');

		// Enter edit mode and select project
		await page.getByRole('button', { name: 'Edit' }).click();
		await page.getByRole('button', { name: /Test Project 1/i }).click();
		await page.locator('button.btn-danger').filter({ hasText: 'Delete project' }).click();

		// Click cancel
		await page.getByRole('button', { name: 'Cancel' }).click();

		// Dialog should close
		await expect(page.getByRole('heading', { name: 'Delete project?' })).not.toBeVisible();

		// Project should still be selected
		await expect(page.getByText('1 selected')).toBeVisible();
	});

	test('deletes single project', async ({ page }) => {
		await page.goto('/');

		// Enter edit mode and select project
		await page.getByRole('button', { name: 'Edit' }).click();
		await page.getByRole('button', { name: /Test Project 1/i }).click();
		await page.locator('button.btn-danger').filter({ hasText: 'Delete project' }).click();

		// Confirm deletion (target the button inside the dialog)
		const dialog = page.getByRole('dialog');
		await dialog.getByRole('button', { name: 'Delete Project' }).click();

		// Wait for dialog to close
		await expect(page.getByRole('heading', { name: 'Delete project?' })).not.toBeVisible();

		// Project should be removed from the list
		await expect(page.locator('main').getByText('Test Project 1')).not.toBeVisible();

		// Other projects should still be visible
		await expect(page.locator('main').getByText('Test Project 2')).toBeVisible();
		await expect(page.locator('main').getByText('Test Project 3')).toBeVisible();
	});

	test('deletes multiple projects', async ({ page }) => {
		await page.goto('/');

		// Enter edit mode and select projects
		await page.getByRole('button', { name: 'Edit' }).click();
		await page.getByRole('button', { name: /Test Project 1/i }).click();
		await page.getByRole('button', { name: /Test Project 2/i }).click();
		await page.getByRole('button', { name: 'Delete 2 projects' }).click();

		// Confirm deletion (target the button inside the dialog)
		const dialog = page.getByRole('dialog');
		await dialog.getByRole('button', { name: 'Delete Projects' }).click();

		// Wait for dialog to close
		await expect(
			page.getByRole('heading', { name: 'Delete 2 projects?' })
		).not.toBeVisible();

		// Deleted projects should be removed
		await expect(page.locator('main').getByText('Test Project 1')).not.toBeVisible();
		await expect(page.locator('main').getByText('Test Project 2')).not.toBeVisible();

		// Remaining project should still be visible
		await expect(page.locator('main').getByText('Test Project 3')).toBeVisible();
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

		// Enter edit mode and select projects
		await page.getByRole('button', { name: 'Edit' }).click();
		await page.getByRole('button', { name: /Test Project 1/i }).click();
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

		// Hover over the first project card to reveal delete button
		const firstCard = page.getByRole('button', { name: /Test Project 1/i });
		await firstCard.hover();

		// Find and click the individual delete button (trash icon)
		const deleteButton = firstCard.locator('[title="Delete project"]');
		await deleteButton.click();

		// Confirmation dialog should appear for single project
		const dialog = page.getByRole('dialog');
		await expect(page.getByRole('heading', { name: 'Delete project?' })).toBeVisible();
		await expect(dialog.getByText('Test Project 1')).toBeVisible();
	});

	test('keyboard navigation works in edit mode', async ({ page }) => {
		await page.goto('/');

		// Enter edit mode
		await page.getByRole('button', { name: 'Edit' }).click();

		// Tab to first project card
		await page.keyboard.press('Tab');
		await page.keyboard.press('Tab'); // Skip select all checkbox

		// Press Enter to select
		await page.keyboard.press('Enter');

		// Should show "1 selected"
		await expect(page.getByText('1 selected')).toBeVisible();
	});
});

test.describe('Project Deletion - Empty State', () => {
	test('hides edit button when no projects', async ({ page }) => {
		// Mock empty projects
		await page.route('**/api/v1/projects', async (route) => {
			await route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify([])
			});
		});

		await page.goto('/');

		// Should show empty state
		await expect(page.getByRole('heading', { name: 'No projects yet' })).toBeVisible();

		// Edit button should not be visible
		await expect(page.getByRole('button', { name: 'Edit' })).not.toBeVisible();
	});
});
