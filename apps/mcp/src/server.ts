// The Hijau MCP server: a thin, Zod-validated tool surface over the REST API so
// coding assistants can manage translations programmatically. Tools map 1:1 to
// REST endpoints and cover the full workflow: keys/translations/review, machine
// translation, translation memory, glossary, import/export (incl. TMX), and the
// async task queue (auto-translate returns a task id you poll with get_task).

import { McpServer, ResourceTemplate } from '@modelcontextprotocol/sdk/server/mcp.js';
import { z } from 'zod';
import type { Api } from './client';

type ToolResult = {
	content: { type: 'text'; text: string }[];
	isError?: boolean;
};

function jsonResult(data: unknown): ToolResult {
	return { content: [{ type: 'text', text: JSON.stringify(data, null, 2) }] };
}

function errorResult(e: unknown): ToolResult {
	const msg = e instanceof Error ? e.message : String(e);
	return { content: [{ type: 'text', text: `Error: ${msg}` }], isError: true };
}

function textResult(text: string): ToolResult {
	return { content: [{ type: 'text', text }] };
}

/** Wrap a tool body so any thrown API error becomes an isError result rather
 *  than crashing the transport. */
function tool<T>(run: () => Promise<T>): Promise<ToolResult> {
	return run().then(jsonResult, errorResult);
}

/** Like tool() but returns the raw string (for file exports, not JSON). */
function textTool(run: () => Promise<string>): Promise<ToolResult> {
	return run().then(textResult, errorResult);
}

export function createServer(api: Api): McpServer {
	const server = new McpServer({ name: 'hijau', version: '0.0.0' });

	server.registerTool(
		'list_projects',
		{ description: 'List the localization projects the token can access.', inputSchema: {} },
		() => tool(() => api.listProjects())
	);

	server.registerTool(
		'get_project',
		{ description: 'Get one project by id.', inputSchema: { projectId: z.string() } },
		({ projectId }) => tool(() => api.getProject(projectId))
	);

	server.registerTool(
		'list_languages',
		{ description: 'List a project’s languages (the base language holds source strings).', inputSchema: { projectId: z.string() } },
		({ projectId }) => tool(() => api.listLanguages(projectId))
	);

	server.registerTool(
		'list_keys',
		{
			description: 'List translation keys in a project, optionally filtered by search/namespace.',
			inputSchema: {
				projectId: z.string(),
				search: z.string().optional(),
				namespaceId: z.string().optional(),
				limit: z.number().int().positive().max(500).optional()
			}
		},
		({ projectId, search, namespaceId, limit }) =>
			tool(() => api.listKeys(projectId, { search, namespaceId, limit }))
	);

	server.registerTool(
		'create_key',
		{
			description: 'Create a translation key.',
			inputSchema: {
				projectId: z.string(),
				name: z.string().describe('Dotted key name, e.g. cart.checkout.button'),
				namespace: z.string().optional(),
				description: z.string().optional().describe('Context shown to translators')
			}
		},
		({ projectId, name, namespace, description }) =>
			tool(() => api.createKey(projectId, { name, namespace, description }))
	);

	server.registerTool(
		'list_translations',
		{ description: 'List all translations (one per language) for a key.', inputSchema: { projectId: z.string(), keyId: z.string() } },
		({ projectId, keyId }) => tool(() => api.listKeyTranslations(projectId, keyId))
	);

	server.registerTool(
		'set_translation',
		{
			description: 'Set the text for a key in a language. Editing the base language marks siblings outdated. ICU placeholders are validated server-side.',
			inputSchema: {
				projectId: z.string(),
				keyId: z.string(),
				language: z.string().describe('BCP-47 tag, e.g. fr or pt-BR'),
				text: z.string()
			}
		},
		({ projectId, keyId, language, text }) =>
			tool(() => api.setTranslation(projectId, keyId, language, text))
	);

	server.registerTool(
		'set_review_state',
		{
			description: 'Approve a translation (→ reviewed) or reject it (→ needs_work). Requires review permission.',
			inputSchema: {
				projectId: z.string(),
				keyId: z.string(),
				language: z.string(),
				action: z.enum(['approve', 'reject'])
			}
		},
		({ projectId, keyId, language, action }) =>
			tool(() => api.transition(projectId, keyId, language, action))
	);

	server.registerTool(
		'find_untranslated_keys',
		{
			description: 'Find keys in a language that still need work (missing, untranslated, needs_work, or outdated).',
			inputSchema: {
				projectId: z.string(),
				language: z.string().describe('BCP-47 tag to check'),
				states: z.array(z.string()).optional().describe('Override which states count as "needs work"'),
				limit: z.number().int().positive().max(500).optional()
			}
		},
		({ projectId, language, states, limit }) =>
			tool(async () => {
				const langs = await api.listLanguages(projectId);
				const lang = langs.find((l) => l.tag === language);
				if (!lang) throw new Error(`No language "${language}" in this project`);
				const want = states && states.length ? states : ['untranslated', 'needs_work', 'outdated'];
				const feed = await api.editorFeed(projectId, { limit: limit ?? 500 });
				const keys = feed.keys
					.map((k) => {
						const t = k.translations[lang.id];
						return { key: k.name, keyId: k.id, state: t?.state ?? 'untranslated', text: t?.text ?? '' };
					})
					.filter((r) => want.includes(r.state) || r.text.trim() === '');
				return { language, count: keys.length, keys };
			})
	);

	server.registerTool(
		'add_comment',
		{
			description: 'Add a comment on a key+language thread.',
			inputSchema: { projectId: z.string(), keyId: z.string(), language: z.string(), body: z.string() }
		},
		({ projectId, keyId, language, body }) =>
			tool(() => api.addComment(projectId, keyId, language, body))
	);

	server.registerTool(
		'mt_suggest',
		{
			description: 'Machine-translate one key into a target language and return the suggestion (does not save it). Requires an MT provider configured on the project.',
			inputSchema: { projectId: z.string(), keyId: z.string(), targetLang: z.string().describe('BCP-47 tag') }
		},
		({ projectId, keyId, targetLang }) => tool(() => api.mtSuggest(projectId, keyId, targetLang))
	);

	server.registerTool(
		'search_translation_memory',
		{
			description: 'Find translation-memory matches (exact + fuzzy) for a key’s source text in a target language.',
			inputSchema: { projectId: z.string(), keyId: z.string(), targetLang: z.string() }
		},
		({ projectId, keyId, targetLang }) => tool(() => api.searchTM(projectId, keyId, targetLang))
	);

	server.registerTool(
		'auto_translate',
		{
			description: 'Bulk-fill untranslated keys for a target language (translation memory first, then MT). Runs as a background task — returns a taskId; poll get_task for progress and final counts.',
			inputSchema: {
				projectId: z.string(),
				targetLang: z.string().describe('BCP-47 tag'),
				limit: z.number().int().positive().max(200).optional().describe('Max keys to translate this run (default 50)')
			}
		},
		({ projectId, targetLang, limit }) =>
			tool(() => api.autoTranslate(projectId, { targetLang, limit, async: true }))
	);

	server.registerTool(
		'get_task',
		{
			description: 'Get an async task’s status, progress and result/error (e.g. an auto_translate or import job).',
			inputSchema: { projectId: z.string(), taskId: z.string() }
		},
		({ projectId, taskId }) => tool(() => api.getTask(projectId, taskId))
	);

	server.registerTool(
		'list_tasks',
		{
			description: 'List recent async tasks for a project (newest first).',
			inputSchema: { projectId: z.string() }
		},
		({ projectId }) => tool(() => api.listTasks(projectId))
	);

	server.registerTool(
		'list_glossary',
		{
			description: 'List the project glossary terms (injected into MT prompts; do-not-translate terms are preserved).',
			inputSchema: { projectId: z.string() }
		},
		({ projectId }) => tool(() => api.listGlossary(projectId))
	);

	server.registerTool(
		'add_glossary_term',
		{
			description: 'Add a glossary term.',
			inputSchema: {
				projectId: z.string(),
				term: z.string(),
				description: z.string().optional(),
				caseSensitive: z.boolean().optional(),
				doNotTranslate: z.boolean().optional().describe('Keep this term verbatim in all languages')
			}
		},
		({ projectId, term, description, caseSensitive, doNotTranslate }) =>
			tool(() => api.createGlossaryTerm(projectId, { term, description, caseSensitive, doNotTranslate }))
	);

	server.registerTool(
		'export_translations',
		{
			description: 'Export one language’s strings as a file and return its contents.',
			inputSchema: {
				projectId: z.string(),
				format: z.string().describe('json | json-nested | csv | android | apple | xliff | po'),
				lang: z.string().describe('BCP-47 tag'),
				state: z.string().optional().describe('Only include translations in this state, e.g. reviewed')
			}
		},
		({ projectId, format, lang, state }) =>
			textTool(() => api.exportTranslations(projectId, { format, lang, state }))
	);

	server.registerTool(
		'import_translations',
		{
			description: 'Import a file’s strings into one language, upserting keys + translations.',
			inputSchema: {
				projectId: z.string(),
				format: z.string().describe('json | json-nested | csv | android | apple | xliff | po'),
				lang: z.string().describe('BCP-47 tag'),
				content: z.string().describe('The file contents'),
				conflict: z.enum(['overwrite', 'keep-existing', 'only-empty']).optional()
			}
		},
		({ projectId, format, lang, content, conflict }) =>
			tool(() => api.importTranslations(projectId, { format, lang, content, conflict }))
	);

	server.registerTool(
		'export_tmx',
		{
			description: 'Export the project’s translation memory as a TMX 1.4 file (optionally narrowed to a source/target language pair).',
			inputSchema: {
				projectId: z.string(),
				sourceLang: z.string().optional(),
				targetLang: z.string().optional()
			}
		},
		({ projectId, sourceLang, targetLang }) =>
			textTool(() => api.exportTMX(projectId, { sourceLang, targetLang }))
	);

	server.registerTool(
		'import_tmx',
		{
			description: 'Import a TMX file into the project’s translation memory (idempotent; duplicates skipped).',
			inputSchema: { projectId: z.string(), content: z.string().describe('The TMX XML') }
		},
		({ projectId, content }) => tool(() => api.importTMX(projectId, content))
	);

	// Read-only resource: project metadata (project + its languages).
	server.registerResource(
		'project-metadata',
		new ResourceTemplate('hijau://project/{projectId}/metadata', { list: undefined }),
		{ description: 'Project settings and languages', mimeType: 'application/json' },
		async (uri, { projectId }) => {
			const id = String(projectId);
			const [project, languages] = await Promise.all([api.getProject(id), api.listLanguages(id)]);
			return {
				contents: [
					{ uri: uri.href, mimeType: 'application/json', text: JSON.stringify({ project, languages }, null, 2) }
				]
			};
		}
	);

	return server;
}
