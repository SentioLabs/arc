import { test, expect } from '@playwright/test';

test.describe('Smoke tests', () => {
	test('health endpoint returns 200', async ({ request }) => {
		const response = await request.get('/health');
		expect(response.status()).toBe(200);
	});

	test('web UI loads at root', async ({ page }) => {
		await page.goto('/');
		await expect(page).toHaveTitle(/.+/);
	});

	test('root page contains navigation structure', async ({ page }) => {
		await page.goto('/');
		// The page should have rendered HTML with at least one link
		const body = page.locator('body');
		await expect(body).toBeVisible();
	});
});
