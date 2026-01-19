import adapter from '@sveltejs/adapter-static';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

/** @type {import('@sveltejs/kit').Config} */
const config = {
	// Consult https://svelte.dev/docs/kit/integrations
	// for more information about preprocessors
	preprocess: vitePreprocess(),

	kit: {
		// Static adapter for embedding in Go binary
		adapter: adapter({
			// Output directory (relative to web/)
			pages: 'build',
			assets: 'build',
			// SPA fallback for client-side routing
			fallback: 'index.html',
			// Precompress with gzip/brotli for embedded serving
			precompress: true,
			// Strict mode requires all pages to be prerenderable
			strict: false
		}),
		// All pages will be prerendered
		prerender: {
			handleHttpError: 'warn'
		}
	}
};

export default config;
