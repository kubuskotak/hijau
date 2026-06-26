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
export interface MtResult {
	text: string;
	confidence?: number;
	notes?: string;
}
export interface TmMatch {
	sourceText: string;
	targetText: string;
	score: number;
	exact: boolean;
}
export interface GlossaryTerm {
	id: string;
	term: string;
	description?: string;
	caseSensitive?: boolean;
	doNotTranslate?: boolean;
}
export interface ImportResult {
	taskId?: string;
	created: number;
	updated: number;
	skipped: number;
	warnings: string[];
}
export interface AutoTranslateResult {
	taskId?: string;
	targetLang: string;
	scanned: number;
	translated: number;
	fromTM: number;
	fromMT: number;
	skipped: number;
	failed: number;
}
export interface Task {
	id: string;
	type: string;
	status: string;
	progress: number;
	processed?: number;
	total?: number;
	result?: unknown;
	error?: string;
	createdAt: string;
	startedAt?: string;
	finishedAt?: string;
}
export interface TmxImportResult {
	imported: number;
	skipped: number;
	warnings: string[];
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
	mtSuggest(projectId: string, keyId: string, targetLang: string): Promise<MtResult>;
	autoTranslate(
		projectId: string,
		body: { targetLang: string; limit?: number; async?: boolean }
	): Promise<AutoTranslateResult>;
	searchTM(projectId: string, keyId: string, targetLang: string): Promise<TmMatch[]>;
	listGlossary(projectId: string): Promise<GlossaryTerm[]>;
	createGlossaryTerm(
		projectId: string,
		body: { term: string; description?: string; caseSensitive?: boolean; doNotTranslate?: boolean }
	): Promise<GlossaryTerm>;
	exportTranslations(projectId: string, q: { format: string; lang: string; state?: string }): Promise<string>;
	importTranslations(
		projectId: string,
		body: { format: string; lang: string; conflict?: string; content: string; async?: boolean }
	): Promise<ImportResult>;
	getTask(projectId: string, taskId: string): Promise<Task>;
	listTasks(projectId: string): Promise<Task[]>;
	exportTMX(projectId: string, q: { sourceLang?: string; targetLang?: string }): Promise<string>;
	importTMX(projectId: string, content: string): Promise<TmxImportResult>;
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
	return query(q as Record<string, unknown> | undefined);
}

function query(q?: Record<string, unknown>): string {
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

	// reqText returns the raw response body (for file exports, which are CSV/XML/
	// TMX/etc. — not JSON, so req()'s JSON.parse would fail).
	private async reqText(method: string, path: string): Promise<string> {
		const res = await fetch(this.base + path, {
			method,
			headers: this.token ? { Authorization: `Bearer ${this.token}` } : {}
		});
		const text = await res.text();
		if (!res.ok) {
			let code = 'ERROR';
			let message = res.statusText;
			try {
				const e = JSON.parse(text)?.error;
				if (e) {
					code = e.code ?? code;
					message = e.message ?? message;
				}
			} catch {
				/* non-JSON error body */
			}
			throw new HijauApiError(res.status, code, message);
		}
		return text;
	}

	mtSuggest(projectId: string, keyId: string, targetLang: string) {
		return this.req<MtResult>('POST', `/projects/${projectId}/keys/${keyId}/mt/suggest`, { targetLang });
	}
	autoTranslate(projectId: string, body: { targetLang: string; limit?: number; async?: boolean }) {
		return this.req<AutoTranslateResult>('POST', `/projects/${projectId}/auto-translate`, body);
	}
	searchTM(projectId: string, keyId: string, targetLang: string) {
		return this.req<TmMatch[]>('POST', `/projects/${projectId}/keys/${keyId}/tm/suggest`, { targetLang });
	}
	listGlossary(projectId: string) {
		return this.req<GlossaryTerm[]>('GET', `/projects/${projectId}/glossary`);
	}
	createGlossaryTerm(
		projectId: string,
		body: { term: string; description?: string; caseSensitive?: boolean; doNotTranslate?: boolean }
	) {
		return this.req<GlossaryTerm>('POST', `/projects/${projectId}/glossary`, body);
	}
	exportTranslations(projectId: string, q: { format: string; lang: string; state?: string }) {
		return this.reqText('GET', `/projects/${projectId}/export${query(q)}`);
	}
	importTranslations(
		projectId: string,
		body: { format: string; lang: string; conflict?: string; content: string; async?: boolean }
	) {
		return this.req<ImportResult>('POST', `/projects/${projectId}/import`, body);
	}
	getTask(projectId: string, taskId: string) {
		return this.req<Task>('GET', `/projects/${projectId}/tasks/${taskId}`);
	}
	listTasks(projectId: string) {
		return this.req<Task[]>('GET', `/projects/${projectId}/tasks`);
	}
	exportTMX(projectId: string, q: { sourceLang?: string; targetLang?: string }) {
		return this.reqText('GET', `/projects/${projectId}/tm/export${query(q)}`);
	}
	importTMX(projectId: string, content: string) {
		return this.req<TmxImportResult>('POST', `/projects/${projectId}/tm/import`, { content });
	}
}
