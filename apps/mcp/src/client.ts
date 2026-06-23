// Thin REST client for the Hijau API. The MCP server is a pure adapter over
// the same REST surface the web UI and CLI use, so all domain logic stays in
// the Go service. `Api` is the interface the tools depend on, which lets tests
// inject a stub instead of hitting a live server.

export interface Project {
	id: string;
	name: string;
	slug: string;
	description: string;
	baseLanguageId: string;
}
export interface Language {
	id: string;
	tag: string;
	name: string;
	isRtl: boolean;
}
export interface Key {
	id: string;
	name: string;
	namespaceId: string;
	description: string;
}
export interface Translation {
	id: string;
	keyId: string;
	languageId: string;
	text: string;
	state: string;
	subId: number;
}
export interface EditorRow extends Key {
	translations: Record<string, Translation>;
}
export interface EditorFeed {
	keys: EditorRow[];
	total: number;
}
export interface Comment {
	id: string;
	body: string;
	resolved: boolean;
}

export interface KeyQuery {
	search?: string;
	namespaceId?: string;
	limit?: number;
	offset?: number;
}

/** The subset of the REST API the MCP tools use. */
export interface Api {
	listProjects(): Promise<Project[]>;
	getProject(id: string): Promise<Project>;
	listLanguages(projectId: string): Promise<Language[]>;
	listKeys(projectId: string, q?: KeyQuery): Promise<Key[]>;
	createKey(projectId: string, body: { name: string; namespace?: string; description?: string }): Promise<Key>;
	listKeyTranslations(projectId: string, keyId: string): Promise<Translation[]>;
	setTranslation(projectId: string, keyId: string, lang: string, text: string): Promise<Translation>;
	transition(projectId: string, keyId: string, lang: string, action: 'approve' | 'reject'): Promise<Translation>;
	editorFeed(projectId: string, q?: KeyQuery): Promise<EditorFeed>;
	addComment(projectId: string, keyId: string, lang: string, body: string): Promise<Comment>;
}

export class HijauApiError extends Error {
	constructor(
		readonly status: number,
		readonly code: string,
		message: string
	) {
		super(message);
		this.name = 'HijauApiError';
	}
}

function qs(q?: KeyQuery): string {
	if (!q) return '';
	const sp = new URLSearchParams();
	for (const [k, v] of Object.entries(q)) {
		if (v !== undefined && v !== null && v !== '') sp.set(k, String(v));
	}
	const s = sp.toString();
	return s ? `?${s}` : '';
}

export class HijauClient implements Api {
	private base: string;
	constructor(
		baseUrl: string,
		private token?: string
	) {
		this.base = baseUrl.replace(/\/$/, '');
	}

	private async req<T>(method: string, path: string, body?: unknown): Promise<T> {
		const res = await fetch(this.base + path, {
			method,
			headers: {
				...(this.token ? { Authorization: `Bearer ${this.token}` } : {}),
				...(body !== undefined ? { 'Content-Type': 'application/json' } : {})
			},
			body: body !== undefined ? JSON.stringify(body) : undefined
		});
		const text = await res.text();
		const data = text ? JSON.parse(text) : undefined;
		if (!res.ok) {
			const e = data?.error;
			throw new HijauApiError(res.status, e?.code ?? 'ERROR', e?.message ?? res.statusText);
		}
		return data as T;
	}

	listProjects() {
		return this.req<Project[]>('GET', '/projects');
	}
	getProject(id: string) {
		return this.req<Project>('GET', `/projects/${id}`);
	}
	listLanguages(projectId: string) {
		return this.req<Language[]>('GET', `/projects/${projectId}/languages`);
	}
	listKeys(projectId: string, q?: KeyQuery) {
		return this.req<Key[]>('GET', `/projects/${projectId}/keys${qs(q)}`);
	}
	createKey(projectId: string, body: { name: string; namespace?: string; description?: string }) {
		return this.req<Key>('POST', `/projects/${projectId}/keys`, body);
	}
	listKeyTranslations(projectId: string, keyId: string) {
		return this.req<Translation[]>('GET', `/projects/${projectId}/keys/${keyId}/translations`);
	}
	setTranslation(projectId: string, keyId: string, lang: string, text: string) {
		return this.req<Translation>('PUT', `/projects/${projectId}/keys/${keyId}/translations/${lang}`, { text });
	}
	transition(projectId: string, keyId: string, lang: string, action: 'approve' | 'reject') {
		return this.req<Translation>('POST', `/projects/${projectId}/keys/${keyId}/translations/${lang}/transition`, {
			action
		});
	}
	editorFeed(projectId: string, q?: KeyQuery) {
		return this.req<EditorFeed>('GET', `/projects/${projectId}/editor${qs(q)}`);
	}
	addComment(projectId: string, keyId: string, lang: string, body: string) {
		return this.req<Comment>('POST', `/projects/${projectId}/keys/${keyId}/translations/${lang}/comments`, {
			body
		});
	}
}
