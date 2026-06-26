import { describe, expect, test } from 'bun:test';
import { Client } from '@modelcontextprotocol/sdk/client/index.js';
import { InMemoryTransport } from '@modelcontextprotocol/sdk/inMemory.js';
import { createServer } from './server';
import type { Api } from './client';

function stubApi(overrides: Partial<Api> = {}): Api {
	return {
		listProjects: async () => [{ id: 'p1', name: 'Demo', slug: 'demo', description: '', baseLanguageId: 'L_en' }],
		getProject: async (id) => ({ id, name: 'Demo', slug: 'demo', description: '', baseLanguageId: 'L_en' }),
		listLanguages: async () => [
			{ id: 'L_en', tag: 'en', name: 'English', isRtl: false },
			{ id: 'L_fr', tag: 'fr', name: 'French', isRtl: false }
		],
		listKeys: async () => [{ id: 'k1', name: 'greeting.hi', namespaceId: '', description: '' }],
		createKey: async (_p, b) => ({ id: 'k2', name: b.name, namespaceId: '', description: b.description ?? '' }),
		listKeyTranslations: async () => [],
		setTranslation: async (_p, _k, lang, text) => ({ id: 't1', keyId: 'k1', languageId: 'L_' + lang, text, state: 'translated', subId: 7 }),
		transition: async (_p, _k, lang, action) => ({ id: 't1', keyId: 'k1', languageId: 'L_' + lang, text: 'x', state: action === 'approve' ? 'reviewed' : 'needs_work', subId: 7 }),
		editorFeed: async () => ({
			keys: [
				{
					id: 'k1',
					name: 'greeting.hi',
					namespaceId: '',
					description: '',
					translations: { L_en: { id: 't0', keyId: 'k1', languageId: 'L_en', text: 'Hi', state: 'reviewed', subId: 6 } }
				}
			],
			total: 1
		}),
		addComment: async (_p, _k, _l, body) => ({ id: 'c1', body, resolved: false }),
		mtSuggest: async () => ({ text: 'Bonjour', confidence: 0.9 }),
		autoTranslate: async (_p, b) => ({
			taskId: 'task1', targetLang: b.targetLang, scanned: 0, translated: 0, fromTM: 0, fromMT: 0, skipped: 0, failed: 0
		}),
		searchTM: async () => [{ sourceText: 'Hi', targetText: 'Salut', score: 80, exact: false }],
		listGlossary: async () => [{ id: 'g1', term: 'Hijau' }],
		createGlossaryTerm: async (_p, b) => ({ id: 'g2', term: b.term }),
		exportTranslations: async () => '{"greeting.hi":"Hi"}',
		importTranslations: async () => ({ created: 1, updated: 0, skipped: 0, warnings: [] }),
		getTask: async (_p, taskId) => ({ id: taskId, type: 'auto_translate', status: 'succeeded', progress: 100, createdAt: '2026-01-01T00:00:00Z' }),
		listTasks: async () => [],
		exportTMX: async () => '<?xml version="1.0"?>\n<tmx version="1.4"></tmx>',
		importTMX: async () => ({ imported: 1, skipped: 0, warnings: [] }),
		...overrides
	};
}

async function connect(api: Api): Promise<Client> {
	const server = createServer(api);
	const client = new Client({ name: 'test', version: '0' });
	const [clientT, serverT] = InMemoryTransport.createLinkedPair();
	await Promise.all([server.connect(serverT), client.connect(clientT)]);
	return client;
}

function firstText(res: { content: unknown }): string {
	return (res.content as { text: string }[])[0].text;
}

describe('hijau mcp server', () => {
	test('registers the expected tools', async () => {
		const client = await connect(stubApi());
		const { tools } = await client.listTools();
		const names = tools.map((t) => t.name);
		for (const n of [
				'list_projects', 'get_project', 'list_keys', 'create_key', 'set_translation', 'set_review_state',
				'find_untranslated_keys', 'add_comment', 'mt_suggest', 'search_translation_memory', 'auto_translate',
				'get_task', 'list_tasks', 'list_glossary', 'add_glossary_term', 'export_translations',
				'import_translations', 'export_tmx', 'import_tmx'
			]) {
			expect(names).toContain(n);
		}
	});

	test('list_projects returns project data', async () => {
		const client = await connect(stubApi());
		const res = await client.callTool({ name: 'list_projects', arguments: {} });
		expect(firstText(res)).toContain('Demo');
	});

	test('auto_translate enqueues and returns a taskId to poll', async () => {
		const client = await connect(stubApi());
		const res = await client.callTool({ name: 'auto_translate', arguments: { projectId: 'p1', targetLang: 'fr' } });
		expect(firstText(res)).toContain('"taskId": "task1"');
	});

	test('export_translations returns the raw file text (not JSON-wrapped)', async () => {
		const client = await connect(stubApi());
		const res = await client.callTool({
			name: 'export_translations',
			arguments: { projectId: 'p1', format: 'json', lang: 'en' }
		});
		expect(firstText(res)).toBe('{"greeting.hi":"Hi"}');
	});

	test('set_translation passes args through to the API', async () => {
		const client = await connect(stubApi());
		const res = await client.callTool({
			name: 'set_translation',
			arguments: { projectId: 'p1', keyId: 'k1', language: 'fr', text: 'Bonjour' }
		});
		const text = firstText(res);
		expect(text).toContain('Bonjour');
		expect(text).toContain('"state": "translated"');
	});

	test('find_untranslated_keys flags the missing French string', async () => {
		const client = await connect(stubApi());
		const res = await client.callTool({
			name: 'find_untranslated_keys',
			arguments: { projectId: 'p1', language: 'fr' }
		});
		const text = firstText(res);
		expect(text).toContain('greeting.hi');
		expect(text).toContain('"count": 1');
	});

	test('a failing API call surfaces as an isError result', async () => {
		const client = await connect(
			stubApi({
				getProject: async () => {
					throw new Error('boom');
				}
			})
		);
		const res = await client.callTool({ name: 'get_project', arguments: { projectId: 'x' } });
		expect(res.isError).toBe(true);
		expect(firstText(res)).toContain('boom');
	});
});
