// The Hijau MCP server: a thin, Zod-validated tool surface over the REST API so
// coding assistants can manage translations programmatically. Tools map 1:1 to
// existing endpoints; M3/M4 tools (machine_translate_keys, translation memory,
// glossary, import/export, get_task) are intentionally absent until those
// endpoints land.

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

/** Wrap a tool body so any thrown API error becomes an isError result rather
 *  than crashing the transport. */
function tool<T>(run: () => Promise<T>): Promise<ToolResult> {
	return run().then(jsonResult, errorResult);
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
