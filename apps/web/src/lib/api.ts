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

export const api = {
	// auth
	signup: (b: { email: string; password: string; name: string }) =>
		req<User>('POST', '/auth/signup', b),
	login: (b: { email: string; password: string }) => req<User>('POST', '/auth/login', b),
	logout: () => req<{ ok: boolean }>('POST', '/auth/logout'),
	me: () => req<User>('GET', '/auth/me'),

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
	listKeys: (
		pid: string,
		q?: { namespaceId?: string; search?: string; limit?: number; offset?: number }
	) => req<Key[]>('GET', `/projects/${pid}/keys${qs(q)}`),
	createKey: (
		pid: string,
		b: { name: string; namespace?: string; description?: string; isPlural?: boolean }
	) => req<Key>('POST', `/projects/${pid}/keys`, b),
	deleteKey: (pid: string, kid: string) =>
		req<{ ok: boolean }>('DELETE', `/projects/${pid}/keys/${kid}`),

	// translations
	listKeyTranslations: (pid: string, kid: string) =>
		req<Translation[]>('GET', `/projects/${pid}/keys/${kid}/translations`),
	setTranslation: (pid: string, kid: string, lang: string, text: string) =>
		req<Translation>('PUT', `/projects/${pid}/keys/${kid}/translations/${lang}`, { text }),
	transition: (pid: string, kid: string, lang: string, action: 'approve' | 'reject') =>
		req<Translation>('POST', `/projects/${pid}/keys/${kid}/translations/${lang}/transition`, {
			action
		})
};
