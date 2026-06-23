import { defineConfig } from 'vite';

// Standalone demo app, separate from the product UI, to show what dropping the
// Hijau SDK into a real app looks like. Runs on :5174. The /api proxy lets you
// switch the demo from offline mode to a live Hijau instance on :8080 without
// CORS fuss (see src/main.ts → LIVE).
export default defineConfig({
	server: {
		port: 5174,
		proxy: { '/api': 'http://localhost:8080' }
	},
	// Serve the workspace SDK packages as TS source rather than pre-bundling them.
	optimizeDeps: { exclude: ['@hijau/web', '@hijau/incontext', '@hijau/i18n'] }
});
