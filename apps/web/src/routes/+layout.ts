// Client-rendered SPA: the Go server serves the built static app in production,
// and in dev the Vite proxy forwards /api to the API. Disabling SSR keeps all
// API calls in the browser where cookies + the proxy work uniformly.
export const ssr = false;
export const prerender = false;
