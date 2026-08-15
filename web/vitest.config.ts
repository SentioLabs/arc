import { defineConfig } from 'vitest/config';

export default defineConfig({
	test: {
		// planner module standardizes on vitest; legacy web tests still use bun:test
		include: ['src/lib/planner/**/*.test.ts'],
		environment: 'node'
	}
});
