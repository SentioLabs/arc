import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vite';

const apiPort = process.env.ARC_API_PORT || '7432';

export default defineConfig({
	plugins: [tailwindcss(), sveltekit()],
	server: {
		proxy: {
			'/api': {
				target: `http://localhost:${apiPort}`,
				changeOrigin: true
			}
		}
	}
});
