import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vite';

// Adapter + Svelte compiler options now live in svelte.config.js. Here we only
// wire the dev-server proxy (so /api reaches the Go API without CORS) + plugins.
export default defineConfig({
	server: {
		proxy: {
			'/api': 'http://localhost:8080'
		}
	},
	plugins: [tailwindcss(), sveltekit()]
});
