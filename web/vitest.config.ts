import { defineConfig } from 'vitest/config';

export default defineConfig({
	test: {
		// Vitest scope is intentionally narrow: only the paste/share-review unit
		// tests we added. The rest of the suite uses Bun's native runner
		// (`import from 'bun:test'`) or Playwright (e2e) and isn't compatible
		// with Vitest's runtime.
		include: ['src/lib/paste/**/*.{test,spec}.{js,ts}'],
		environment: 'node'
	}
});
