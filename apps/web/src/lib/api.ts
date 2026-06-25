// Typed client for the Hijau REST API. Hand-written to match the Go DTOs; can
// be swapped for an OpenAPI-generated client later. All calls are same-origin
// (the Vite dev proxy forwards /api → :8080, and in production the Go server
// serves the built app), so cookies authenticate automatically.

const BASE = '/api/v1';

export class ApiError extends Error {
	status: number;
	code: string;
	constructor(status: number, code: string, message: string) {
		super(message);
		this.status = status;
		this.code = code;
	}
}

async function req<T>(method: string, path: string, body?: unknown): Promise<T> {
	const res = await fetch(BASE + path, {
		method,
		headers: body !== undefined ? { 'Content-Type': 'application/json' } : undefined,
		body: body !== undefined ? JSON.stringify(body) : undefined,
		credentials: 'include'
	});
	const text = await res.text();
	const data = text ? JSON.parse(text) : undefined;
	if (!res.ok) {
		const e = data?.error;
		throw new ApiError(res.status, e?.code ?? 'ERROR', e?.message ?? res.statusText);
	}
	return data as T;
}

function qs(q?: Record<string, unknown>): string {
	if (!q) return '';
	const sp = new URLSearchParams();
	for (const [k, v] of Object.entries(q)) {
		if (v !== undefined && v !== null && v !== '') sp.set(k, String(v));
	}
	const s = sp.toString();
	return s ? `?${s}` : '';
}

export interface User {
	id: string;
	email: string;
	name: string;
}
export interface Org {
	id: string;
	name: string;
	slug: string;
}
export interface Project {
	id: string;
	orgId: string;
	name: string;
	slug: string;
	description: string;
	baseLanguageId: string;
	createdAt: string;
}
export interface Language {
	id: string;
	tag: string;
	name: string;
	isRtl: boolean;
	pluralForms: string[];
}
export interface Namespace {
	id: string;
	name: string;
}
export interface Key {
	id: string;
	projectId: string;
	namespaceId: string;
	name: string;
	description: string;
	isPlural: boolean;
	tags: string[];
	createdAt: string;
}
export type TranslationState =
	| 'untranslated'
	| 'translated'
	| 'reviewed'
	| 'needs_work'
	| 'outdated';
export interface Translation {
	id: string;
	keyId: string;
	languageId: string;
	text: string;
	state: TranslationState;
	origin: string;
	isMachine: boolean;
	subId: number;
	version: number;
	updatedAt: string;
}
export interface EditorRow extends Key {
	translations: Record<string, Translation>;
}
export interface EditorFeed {
	keys: EditorRow[];
	total: number;
}
export interface HistoryEntry {
	id: string;
	oldText: string;
	newText: string;
	oldState: string;
	newState: string;
	origin: string;
	authorKind: string;
	authorEmail: string;
	createdAt: string;
}
export interface Comment {
	id: string;
	body: string;
	parentId: string;
	authorEmail: string;
	authorName: string;
	resolved: boolean;
	createdAt: string;
}

export interface ScreenshotRegion {
	id: string;
	keyId: string;
	x: number;
	y: number;
	w: number;
	h: number;
}
export interface Screenshot {
	id: string;
	name: string;
	width: number;
	height: number;
	imageUrl: string;
	createdAt: string;
	regions: ScreenshotRegion[];
}

export interface MtConfig {
	provider: string;
	model: string;
	enabled: boolean;
	hasCredentials: boolean;
}
export interface MtResult {
	text: string;
	provider: string;
	model: string;
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
	description: string;
	caseSensitive: boolean;
	doNotTranslate: boolean;
	translations: Record<string, string>;
}

export interface Pat {
	id: string;
	name: string;
	prefix: string;
	createdAt: string;
	lastUsedAt?: string;
}
export interface Member {
	id: string;
	userId: string;
	email: string;
	name: string;
	role: string;
	languageIds: string[];
}
export interface Activity {
	id: string;
	type: string;
	actorKind: string;
	actorEmail: string;
	keyName: string;
	languageTag: string;
	createdAt: string;
}
export interface ImportResult {
	taskId?: string; // set when the import was enqueued async (poll getTask)
	created: number;
	updated: number;
	skipped: number;
	warnings: string[];
}
export interface TmxImportResult {
	imported: number;
	skipped: number;
	warnings: string[];
}
export interface Task {
	id: string;
	type: string;
	status: 'queued' | 'running' | 'succeeded' | 'failed';
	progress: number;
	processed?: number;
	total?: number;
	result?: unknown;
	error?: string;
	createdAt: string;
	startedAt?: string;
	finishedAt?: string;
}
export interface Webhook {
	id: string;
	url: string;
	events: string[];
	active: boolean;
	createdAt: string;
	secret?: string; // returned only once, on create
}
export interface WebhookDelivery {
	id: string;
	event: string;
	statusCode: number;
	success: boolean;
	error: string;
	createdAt: string;
}

type KeyQuery = { namespaceId?: string; search?: string; limit?: number; offset?: number };

export const api = {
	// auth
	signup: (b: { email: string; password: string; name: string }) =>
		req<User>('POST', '/auth/signup', b),
	login: (b: { email: string; password: string }) => req<User>('POST', '/auth/login', b),
	logout: () => req<{ ok: boolean }>('POST', '/auth/logout'),
	me: () => req<User>('GET', '/auth/me'),

	// personal access tokens (for the CLI + MCP)
	listMyTokens: () => req<Pat[]>('GET', '/me/tokens'),
	createToken: (name: string) => req<{ token: string; prefix: string }>('POST', '/me/tokens', { name }),
	revokeToken: (id: string) => req<{ ok: boolean }>('DELETE', `/me/tokens/${id}`),

	// orgs & projects
	listOrgs: () => req<Org[]>('GET', '/orgs'),
	listProjects: () => req<Project[]>('GET', '/projects'),
	createProject: (b: { orgId: string; name: string; slug?: string; description?: string }) =>
		req<Project>('POST', '/projects', b),
	getProject: (id: string) => req<Project>('GET', `/projects/${id}`),
	updateProject: (id: string, b: { name: string; description: string }) =>
		req<Project>('PATCH', `/projects/${id}`, b),

	// languages
	listLanguages: (pid: string) => req<Language[]>('GET', `/projects/${pid}/languages`),
	createLanguage: (
		pid: string,
		b: { tag: string; name: string; isRtl?: boolean; pluralForms?: string[] }
	) => req<Language>('POST', `/projects/${pid}/languages`, b),
	setBaseLanguage: (pid: string, languageId: string) =>
		req<{ ok: boolean }>('PUT', `/projects/${pid}/base-language`, { languageId }),

	// namespaces
	listNamespaces: (pid: string) => req<Namespace[]>('GET', `/projects/${pid}/namespaces`),
	createNamespace: (pid: string, name: string) =>
		req<Namespace>('POST', `/projects/${pid}/namespaces`, { name }),

	// keys
	listKeys: (pid: string, q?: KeyQuery) => req<Key[]>('GET', `/projects/${pid}/keys${qs(q)}`),
	createKey: (
		pid: string,
		b: { name: string; namespace?: string; description?: string; isPlural?: boolean }
	) => req<Key>('POST', `/projects/${pid}/keys`, b),
	deleteKey: (pid: string, kid: string) =>
		req<{ ok: boolean }>('DELETE', `/projects/${pid}/keys/${kid}`),

	// editor feed: keys + their translations across languages
	editorFeed: (pid: string, q?: KeyQuery) =>
		req<EditorFeed>('GET', `/projects/${pid}/editor${qs(q)}`),

	// translations
	listKeyTranslations: (pid: string, kid: string) =>
		req<Translation[]>('GET', `/projects/${pid}/keys/${kid}/translations`),
	setTranslation: (pid: string, kid: string, lang: string, text: string) =>
		req<Translation>('PUT', `/projects/${pid}/keys/${kid}/translations/${lang}`, { text }),
	transition: (pid: string, kid: string, lang: string, action: 'approve' | 'reject') =>
		req<Translation>('POST', `/projects/${pid}/keys/${kid}/translations/${lang}/transition`, {
			action
		}),

	// side panel: per-translation history & comments
	translationHistory: (pid: string, kid: string, lang: string) =>
		req<HistoryEntry[]>('GET', `/projects/${pid}/keys/${kid}/translations/${lang}/history`),
	listComments: (pid: string, kid: string, lang: string) =>
		req<Comment[]>('GET', `/projects/${pid}/keys/${kid}/translations/${lang}/comments`),
	addComment: (pid: string, kid: string, lang: string, body: string, parentId?: string) =>
		req<Comment>('POST', `/projects/${pid}/keys/${kid}/translations/${lang}/comments`, {
			body,
			parentId
		}),
	resolveComment: (cid: string, resolved: boolean) =>
		req<{ ok: boolean }>('POST', `/comments/${cid}/resolve`, { resolved }),

	// screenshots where a key appears (with regions highlighting it)
	listKeyScreenshots: (pid: string, kid: string) =>
		req<Screenshot[]>('GET', `/projects/${pid}/keys/${kid}/screenshots`),

	// machine translation + translation memory
	mtConfig: (pid: string) => req<MtConfig>('GET', `/projects/${pid}/mt/config`),
	configureMT: (pid: string, b: { provider: string; model?: string; apiKey?: string; enabled?: boolean }) =>
		req<MtConfig>('PUT', `/projects/${pid}/mt/config`, b),
	mtSuggest: (pid: string, kid: string, targetLang: string) =>
		req<MtResult>('POST', `/projects/${pid}/keys/${kid}/mt/suggest`, { targetLang }),
	tmSuggest: (pid: string, kid: string, targetLang: string) =>
		req<TmMatch[]>('POST', `/projects/${pid}/keys/${kid}/tm/suggest`, { targetLang }),

	// glossary
	listGlossary: (pid: string) => req<GlossaryTerm[]>('GET', `/projects/${pid}/glossary`),
	createGlossaryTerm: (
		pid: string,
		b: { term: string; description?: string; caseSensitive?: boolean; doNotTranslate?: boolean }
	) => req<GlossaryTerm>('POST', `/projects/${pid}/glossary`, b),
	deleteGlossaryTerm: (pid: string, termId: string) =>
		req<{ ok: boolean }>('DELETE', `/projects/${pid}/glossary/${termId}`),
	setGlossaryTranslation: (pid: string, termId: string, lang: string, text: string) =>
		req<{ ok: boolean }>('PUT', `/projects/${pid}/glossary/${termId}/translations/${lang}`, { text }),

	// import / export
	importTranslations: (
		pid: string,
		b: { format: string; lang: string; conflict?: string; content: string; async?: boolean }
	) => req<ImportResult>('POST', `/projects/${pid}/import`, b),
	exportUrl: (pid: string, p: { format: string; lang: string; state?: string }) =>
		`${BASE}/projects/${pid}/export${qs(p)}`,

	// translation-memory interchange (TMX)
	importTMX: (pid: string, content: string) =>
		req<TmxImportResult>('POST', `/projects/${pid}/tm/import`, { content }),
	tmxExportUrl: (pid: string, p?: { sourceLang?: string; targetLang?: string }) =>
		`${BASE}/projects/${pid}/tm/export${qs(p)}`,

	// async tasks (import / auto-translate run on the server worker)
	listTasks: (pid: string) => req<Task[]>('GET', `/projects/${pid}/tasks`),
	getTask: (pid: string, tid: string) => req<Task>('GET', `/projects/${pid}/tasks/${tid}`),

	// webhooks
	listWebhooks: (pid: string) => req<Webhook[]>('GET', `/projects/${pid}/webhooks`),
	createWebhook: (pid: string, b: { url: string; events?: string[] }) =>
		req<Webhook>('POST', `/projects/${pid}/webhooks`, b),
	deleteWebhook: (pid: string, wid: string) =>
		req<{ ok: boolean }>('DELETE', `/projects/${pid}/webhooks/${wid}`),
	listWebhookDeliveries: (pid: string, wid: string) =>
		req<WebhookDelivery[]>('GET', `/projects/${pid}/webhooks/${wid}/deliveries`),

	// activity feed
	listActivity: (pid: string, limit = 50) =>
		req<Activity[]>('GET', `/projects/${pid}/activity?limit=${limit}`),

	// members
	listMembers: (pid: string) => req<Member[]>('GET', `/projects/${pid}/members`),
	addMember: (pid: string, b: { email: string; role: string }) =>
		req<Member>('POST', `/projects/${pid}/members`, b),
	updateMemberRole: (pid: string, mid: string, role: string) =>
		req<{ ok: boolean }>('PATCH', `/projects/${pid}/members/${mid}`, { role }),
	removeMember: (pid: string, mid: string) =>
		req<{ ok: boolean }>('DELETE', `/projects/${pid}/members/${mid}`),
	setMemberLanguages: (pid: string, mid: string, languageIds: string[]) =>
		req<{ ok: boolean }>('PUT', `/projects/${pid}/members/${mid}/languages`, { languageIds })
};
