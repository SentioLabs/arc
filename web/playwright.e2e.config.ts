import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
	testDir: './tests/e2e',
	timeout: 30000,
	expect: { timeout: 10000 },
	fullyParallel: true,
	forbidOnly: !!process.env.CI,
	retries: process.env.CI ? 2 : 0,
	workers: process.env.CI ? 1 : undefined,
	reporter: [['html', { open: 'never' }]],
	use: {
		baseURL: 'http://localhost:7433',
		trace: 'on-first-retry'
	},
	projects: [
		{
			name: 'chromium',
			use: { ...devices['Desktop Chrome'] }
		}
	]
	// No webServer block — test server is managed externally by docker compose
});
