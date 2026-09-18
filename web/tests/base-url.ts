// Both browser pages and API seed helpers use the harness-discovered endpoint.
const endpoint = process.env.ARC_TEST_BASE_URL;
if (!endpoint) throw new Error('ARC_TEST_BASE_URL is required; run scripts/test-e2e.sh playwright');
const parsed = new URL(endpoint);
if (
	parsed.protocol !== 'http:' ||
	parsed.hostname !== '127.0.0.1' ||
	!parsed.port ||
	parsed.port === '7432'
) {
	throw new Error('ARC_TEST_BASE_URL must be an isolated loopback port (never production 7432)');
}
export const BASE_URL = parsed.origin;
export const API_BASE = `${BASE_URL}/api/v1`;
