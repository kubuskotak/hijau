// Live updates over Server-Sent Events. Same-origin (cookie-authenticated, via
// the Vite proxy in dev or the Go server in prod). The browser EventSource
// auto-reconnects on transient errors.

export interface LiveUpdate {
	event: string;
	projectId: string;
	key: string;
	language: string;
	text: string;
	state: string;
	timestamp: string;
}

/** Subscribe to a project's live update stream. Returns an unsubscribe fn. */
export function subscribeUpdates(pid: string, onUpdate: (u: LiveUpdate | null) => void): () => void {
	if (typeof EventSource === 'undefined') return () => {};
	const es = new EventSource(`/api/v1/projects/${pid}/events`);
	es.addEventListener('update', (e) => {
		try {
			onUpdate(JSON.parse((e as MessageEvent).data) as LiveUpdate);
		} catch {
			onUpdate(null);
		}
	});
	return () => es.close();
}
