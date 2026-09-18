import { afterEach, expect, test, vi } from 'vitest';

afterEach(() => {
	vi.unstubAllEnvs();
	vi.resetModules();
});
test.each([
	undefined,
	'http://127.0.0.1:7432',
	'http://localhost:7433',
	'https://example.com:1234'
])('rejects unsafe or missing endpoint %s', async (endpoint) => {
	vi.stubEnv('ARC_TEST_BASE_URL', endpoint);
	await expect(import('./base-url')).rejects.toThrow(/ARC_TEST_BASE_URL/);
});
test('page configuration and API helpers share the discovered loopback origin', async () => {
	vi.stubEnv('ARC_TEST_BASE_URL', 'http://127.0.0.1:32123');
	const { BASE_URL, API_BASE } = await import('./base-url');
	expect(BASE_URL).toBe('http://127.0.0.1:32123');
	expect(API_BASE).toBe(`${BASE_URL}/api/v1`);
});
