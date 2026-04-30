import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vite';

export default defineConfig({
	plugins: [tailwindcss(), sveltekit()],
	server: {
		proxy: {
			// Backend target overridable via ARC_PASTE_BACKEND so the dev server
			// can drive a non-default port (e.g. arc-paste running on :7436).
			'/api': {
				target: process.env.ARC_PASTE_BACKEND ?? 'http://localhost:7432',
				changeOrigin: true
			}
		}
	}
});
