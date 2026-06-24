import adapter from '@sveltejs/adapter-static';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

/** @type {import('@sveltejs/kit').Config} */
export default {
	preprocess: vitePreprocess(),

	kit: {
		// Pure SPA (ssr=false everywhere): every route falls back to index.html so
		// client-side routing works. The Go server serves this build in production.
		adapter: adapter({ fallback: 'index.html' })
	},

	vitePlugin: {
		// Force runes mode for app source; leave node_modules libraries on auto.
		dynamicCompileOptions({ filename }) {
			return {
				runes: filename.split(/[/\\]/).includes('node_modules') ? undefined : true
			};
		}
	}
};
